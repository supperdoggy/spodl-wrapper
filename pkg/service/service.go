package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/DigitalIndependence/models"
	"github.com/DigitalIndependence/models/spotify"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/blob"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/db"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/utils"
	"go.uber.org/zap"
)

type Service interface {
	StartProcessing(ctx context.Context) error
}

type service struct {
	database       db.Database
	s3             blob.BlobStorage
	log            *zap.Logger
	spotifyService spotify.SpotifyService

	destination    string
	s3Enabled      bool
	sleepInMinutes int
	libraryPath    string
}

func NewService(database db.Database, log *zap.Logger, s3 blob.BlobStorage, spotifyService spotify.SpotifyService, destination, libraryPath string, s3Enabled bool, sleepInMinutes int) Service {
	return &service{
		database:       database,
		log:            log,
		spotifyService: spotifyService,
		destination:    destination,
		s3:             s3,
		s3Enabled:      s3Enabled,
		sleepInMinutes: sleepInMinutes,
		libraryPath:    libraryPath,
	}
}

// StartProcessing starts the processing of the requests
func (s *service) StartProcessing(ctx context.Context) error {
	downloadError := s.ProcessDownloadRequest(ctx)

	playlistError := s.ProcessPlaylistRequest(ctx)

	return errors.Join(downloadError, playlistError)
}

func (s *service) ProcessDownloadRequest(ctx context.Context) error {
	active, err := s.database.GetActiveRequests(ctx)
	if err != nil {
		s.log.Error("failed to get active requests", zap.Error(err))
		return err
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

func (s *service) ProcessPlaylistRequest(ctx context.Context) error {
	playlists, err := s.database.GetActivePlaylists(ctx)
	if err != nil {
		s.log.Error("failed to get active playlists", zap.Error(err))
		return err
	}

	s.log.Info("processing active playlists", zap.Any("playlists", len(playlists)))

	for _, playlist := range playlists {
		if err := s.ProcessPlaylist(ctx, playlist); err != nil {
			s.log.Error("failed to process playlist", zap.Error(err), zap.Any("playlist", playlist))
		} else {
			playlist.Active = false
		}

		if err := s.database.UpdatePlaylistRequest(ctx, playlist); err != nil {
			s.log.Error("failed to update playlist", zap.Error(err), zap.Any("playlist", playlist))
		}
	}

	s.log.Info("completed processing of active playlists")
	return nil
}

func (s *service) ProcessPlaylist(ctx context.Context, playlist models.PlaylistRequest) error {
	s.log.Info("processing playlist", zap.Any("playlist", playlist))

	// checking if playlist is ready to be processed
	// by checking if we have active request for playlist download
	downloadRequest, err := s.database.GetActiveRequest(ctx, playlist.SpotifyURL)
	if err != nil {
		s.log.Error("failed to get active request", zap.Error(err))
		return err
	}

	if downloadRequest.Active {
		s.log.Info("download request is still active, will continue to process playlist once done", zap.Any("playlist", playlist))
		return nil
	}

	playlistName, err := s.spotifyService.GetObjectName(ctx, playlist.SpotifyURL)
	if err != nil {
		s.log.Error("failed to get playlist name", zap.Error(err))
		return err
	}

	songList, err := s.spotifyService.GetPlaylistTracks(ctx, playlist.SpotifyURL)
	if err != nil {
		s.log.Error("failed to get playlist data", zap.Error(err))
		return err
	}

	artistSong := make(map[string]string)
	for _, item := range songList {
		artist := []string{}
		for _, artistItem := range item.Track.Track.Artists {
			artist = append(artist, strings.ToLower(artistItem.Name))
		}

		artistSong[strings.Join(artist, ", ")] = strings.ToLower(item.Track.Track.Name)
	}

	foundMusic, err := s.database.FindMusicFiles(ctx, artistSong)
	if err != nil {
		s.log.Error("failed to find music file paths", zap.Error(err))
		return err
	}

	if len(foundMusic) == 0 {
		s.log.Error("no indexed paths found for playlist", zap.Any("playlistName", playlistName))
		return errors.New("no indexed paths found for playlist")
	}

	foundMusicMap := make(map[string]models.MusicFile)
	for _, music := range foundMusic {
		key := music.Artist + " " + music.Title
		foundMusicMap[key] = music
	}

	indexedPaths := make([]string, len(foundMusic))
	for _, song := range songList {
		artists := []string{}
		for _, artistItem := range song.Track.Track.Artists {
			artists = append(artists, strings.ToLower(artistItem.Name))
		}

		artist := strings.Join(artists, ", ")

		songName := strings.ToLower(song.Track.Track.Name)

		key := artist + " " + songName

		if _, ok := foundMusicMap[key]; !ok {
			s.log.Error("song not found in indexed paths", zap.Any("artist", artist), zap.Any("songName", songName))
			// return errors.New("song not found in indexed paths")
			continue
		}

		indexedPaths = append(indexedPaths, foundMusicMap[key].Path)
	}

	for i, file := range foundMusic {
		indexedPaths[i] = strings.ReplaceAll(file.Path, "/srv/remotemount/nascore_media/Music", "/music/")
	}

	s.log.Info("got playlist data", zap.Any("playlistName", playlistName), zap.Any("artistSong", artistSong))

	outputPath := s.destination + "/Playlists/" + playlistName + ".m3u"

	if err := utils.CreateM3UPlaylist(indexedPaths, s.libraryPath, outputPath); err != nil {
		s.log.Error("failed to create m3u playlist", zap.Error(err))
		return err
	}

	s.log.Info("created m3u playlist", zap.Any("outputPath", outputPath))

	return nil
}
