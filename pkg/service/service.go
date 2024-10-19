package service

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/DigitalIndependence/models"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/blob"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/db"
	"go.uber.org/zap"
)

type Service interface {
	StartProcessing(ctx context.Context) error
}

type service struct {
	database db.Database
	s3       blob.BlobStorage
	log      *zap.Logger

	destination    string
	s3Enabled      bool
	sleepInMinutes int
}

func NewService(database db.Database, log *zap.Logger, s3 blob.BlobStorage, destination string, s3Enabled bool, sleepInMinutes int) Service {
	return &service{
		database:       database,
		log:            log,
		destination:    destination,
		s3:             s3,
		s3Enabled:      s3Enabled,
		sleepInMinutes: sleepInMinutes,
	}
}

// StartProcessing starts the processing of the requests
func (s *service) StartProcessing(ctx context.Context) error {
	for {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("recovered from panic", zap.Any("panic", r))
			}
		}()

		time.Sleep(time.Duration(s.sleepInMinutes) * time.Minute)

		active, err := s.database.GetActiveRequests(ctx)
		if err != nil {
			s.log.Error("failed to get active requests", zap.Error(err))
			continue
		}

		for _, request := range active {
			if err := s.ProcessRequest(ctx, request); err != nil {
				s.log.Error("failed to process request", zap.Error(err), zap.Any("request", request))
			}
		}

		if !s.s3Enabled {
			continue
		}
	}
}

// ProcessRequest processes the request
func (s *service) ProcessRequest(ctx context.Context, request models.DownloadQueueRequest) error {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("recovered from panic", zap.Any("panic", r))
		}
	}()

	// Run the "spotdl --sync {url}" command
	cmd := exec.Command("spotdl", request.SpotifyURL, "--sync-without-deleting", "--cookie-file", "cookies.txt", "--bitrate", "320k", "--output", s.destination)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
