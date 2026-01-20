package service

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"sort"
	"strings"
	"time"

	models "github.com/supperdoggy/spot-models"
	"go.uber.org/zap"
)

func (s *service) ProcessDownloadRequest(ctx context.Context) error {
	active, err := s.database.GetActiveRequests(ctx)
	if err != nil {
		s.log.Error("failed to get active requests", zap.Error(err))
		return err
	}

	if len(active) == 0 {
		s.log.Info("no active requests to process")
		return nil
	}

	s.log.Info("processing active requests", zap.Any("requests", len(active)))

	// sort active by errored so errored requests are processed last
	sort.Slice(active, func(i, j int) bool {
		if active[i].Errored && !active[j].Errored {
			return false
		}
		if !active[i].Errored && active[j].Errored {
			return true
		}
		return active[i].CreatedAt < active[j].CreatedAt
	})

	s.log.Info("sorted active requests", zap.Any("requests", active))

	for _, request := range active {
		time.Sleep(time.Duration(s.sleepInMinutes) * time.Minute)

		errored := false
		if err := s.ProcessRequest(ctx, request); err != nil {
			s.log.Error("failed to process request", zap.Error(err), zap.Any("request", request))
			errored = true
		}

		if !errored {
			if request.SyncCount >= 3 {
				request.Active = false
			} else {
				request.SyncCount++
			}
		} else {
			request.Errored = true
			request.RetryCount++
			s.log.Warn("request processing encountered an error", zap.Any("request", request))
		}

		s.log.Info("updated request status", zap.Any("request", request))

		if err := s.database.UpdateActiveRequest(ctx, request); err != nil {
			s.log.Error("failed to update request", zap.Error(err), zap.Any("request", request))
		}
	}

	indexStatus, err := s.database.GetIndexStatus(ctx)
	if err != nil {
		s.log.Error("failed to get index status", zap.Error(err))
		return err
	}

	indexStatus.LastUpdated = time.Now().UTC().Unix()
	if err := s.database.UpdateIndexStatus(ctx, indexStatus); err != nil {
		s.log.Error("failed to update index status", zap.Error(err))
		return err
	}

	// Update found track count for all active requests after indexing
	// This ensures progress is updated even if indexing happens separately
	for _, request := range active {
		if request.ExpectedTrackCount > 0 && len(request.TrackMetadata) > 0 {
			if err := s.UpdateFoundTrackCount(ctx, request); err != nil {
				s.log.Error("failed to update found track count after indexing", zap.Error(err), zap.String("request_id", request.ID))
			}
		}
	}

	s.log.Info("completed processing of active requests")
	return nil
}

// ProcessRequest processes the request
func (s *service) ProcessRequest(ctx context.Context, request models.DownloadQueueRequest) error {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("recovered from panic", zap.Any("panic", r))
		}
	}()

	// Fetch track count and metadata if not already set
	if request.ExpectedTrackCount == 0 || len(request.TrackMetadata) == 0 {
		s.log.Info("fetching track count and metadata", zap.String("url", request.SpotifyURL))
		trackCount, trackMetadata, err := s.spotifyService.GetTrackCount(ctx, request.SpotifyURL)
		if err != nil {
			s.log.Error("failed to get track count", zap.Error(err), zap.String("url", request.SpotifyURL))
			// Continue anyway, we'll just not have progress tracking
		} else {
			request.ExpectedTrackCount = trackCount
			request.TrackMetadata = trackMetadata
			request.UpdatedAt = time.Now().Unix()
			if err := s.database.UpdateActiveRequest(ctx, request); err != nil {
				s.log.Error("failed to update request with track count", zap.Error(err))
			}
			s.log.Info("fetched track count", zap.Int("count", trackCount), zap.String("url", request.SpotifyURL))
		}
	}

	// Build output format with destination path
	args := []string{
		request.SpotifyURL,
		"--output", s.destination,
		"--config",
		"--no-cache",
		"--sync-without-deleting",
	}

	// Run the "spotdl --sync {url}" command
	cmd := exec.Command("spotdl", args...)

	s.log.Info("executing command", zap.String("command", cmd.String()))

	// Capture stdout and stderr through the logger
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return err
	}

	// Stream stdout and stderr to logger
	go s.streamOutput(stdout, "stdout")
	go s.streamOutput(stderr, "stderr")

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		return err
	}

	// After download completes, compare with indexed files
	if request.ExpectedTrackCount > 0 && len(request.TrackMetadata) > 0 {
		if err := s.UpdateFoundTrackCount(ctx, request); err != nil {
			s.log.Error("failed to update found track count", zap.Error(err))
			// Don't fail the request, just log the error
		}
	}

	return nil
}

// UpdateFoundTrackCount compares indexed files with expected tracks and updates the found count
func (s *service) UpdateFoundTrackCount(ctx context.Context, request models.DownloadQueueRequest) error {
	if len(request.TrackMetadata) == 0 {
		return nil
	}

	// Extract artists and titles from track metadata
	artists := make([]string, 0, len(request.TrackMetadata))
	titles := make([]string, 0, len(request.TrackMetadata))
	for _, track := range request.TrackMetadata {
		artists = append(artists, track.Artist)
		titles = append(titles, track.Title)
	}

	// Find matching music files in the database
	foundMusic, err := s.database.FindMusicFiles(ctx, artists, titles)
	if err != nil {
		return err
	}

	// Create a map for quick lookup
	foundMap := make(map[string]bool)
	for _, music := range foundMusic {
		key := strings.ToLower(music.Artist) + " " + strings.ToLower(music.Title)
		foundMap[key] = true
	}

	// Count how many expected tracks were found
	foundCount := 0
	for _, track := range request.TrackMetadata {
		key := strings.ToLower(track.Artist) + " " + strings.ToLower(track.Title)
		if foundMap[key] {
			foundCount++
		}
	}

	// Update the request with found count
	request.FoundTrackCount = foundCount
	request.UpdatedAt = time.Now().Unix()

	if err := s.database.UpdateActiveRequest(ctx, request); err != nil {
		return err
	}

	s.log.Info("updated found track count",
		zap.String("request_id", request.ID),
		zap.Int("expected", request.ExpectedTrackCount),
		zap.Int("found", foundCount),
		zap.Float64("percentage", float64(foundCount)/float64(request.ExpectedTrackCount)*100))

	return nil
}

// streamOutput reads from a pipe and logs each line
func (s *service) streamOutput(pipe io.ReadCloser, stream string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			s.log.Info("spotdl", zap.String("stream", stream), zap.String("output", line))
		}
	}
}
