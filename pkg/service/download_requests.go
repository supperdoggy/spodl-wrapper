package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/DigitalIndependence/models"
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

	s.log.Info("completed processing of active requests")
	return nil
}

// ProcessRequest processes the request
func (s *service) ProcessRequest(ctx context.Context, request models.DownloadQueueRequest) error {
	defer func() error {
		if r := recover(); r != nil {
			s.log.Error("recovered from panic", zap.Any("panic", r))
			return errors.New("recovered from panic")
		}
		return nil
	}()

	// Run the "spotdl --sync {url}" command
	cmd := exec.Command("spotdl", request.SpotifyURL, "--sync-without-deleting", "--cookie-file", "/home/maks/music.youtube.com_cookies.txt", "--bitrate", "320k", "--format", "m4a", "--output", s.destination)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
