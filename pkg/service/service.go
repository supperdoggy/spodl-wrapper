package service

import (
	"context"
	"errors"

	"github.com/DigitalIndependence/models/spotify"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/blob"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/db"
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

	// // run indexation for all downloaded files
	// if err := s.IndexDownloadedFiles(ctx); err != nil {
	// 	s.log.Error("failed to index downloaded files", zap.Error(err))
	// }

	playlistError := s.ProcessPlaylistRequest(ctx)

	return errors.Join(downloadError, playlistError)
}
