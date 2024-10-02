package service

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/DigitalIndependence/models"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/db"
	"go.uber.org/zap"
)

type Service interface {
	StartProcessing(ctx context.Context) error
}

type service struct {
	database db.Database
	log      *zap.Logger
}

func NewService(database db.Database, log *zap.Logger) Service {
	return &service{
		database: database,
		log:      log,
	}
}

func (s *service) StartProcessing(ctx context.Context) error {
	for {
		time.Sleep(1 * time.Minute)
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
	}
}

func (s *service) ProcessRequest(ctx context.Context, request models.DownloadQueueRequest) error {
	// Run the "spotdl --sync {url}" command
	cmd := exec.Command("spotdl", "--sync", request.SpotifyURL, "--cookie-file cookies.txt", "--bitrate disable")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
