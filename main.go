package main

import (
	"context"
	"time"

	"github.com/DigitalIndependence/models/spotify"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/blob"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/config"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/db"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/service"
	"go.uber.org/zap"
)

func main() {
	run()
	time.Sleep(5 * time.Second)

}

func run() {
	var ctx = context.Background()
	log, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal("failed to load config", zap.Error(err))
	}

	log.Info("loaded config", zap.Any("config", cfg))

	spotifyService := spotify.NewSpotifyService(ctx, cfg.Spotify.ClientID, cfg.Spotify.ClientSecret, log)

	db, err := db.NewDatabase(ctx, log, cfg.DatabaseURL, cfg.DatabaseName)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	log.Info("connected to database")

	blobStorage := blob.BlobStorage(nil)
	if cfg.Blob.Enabled {
		blobStorage, err = blob.NewBlobStorage(log, cfg.Blob)
		if err != nil {
			log.Fatal("failed to create blob storage", zap.Error(err))
		}
	}

	srv := service.NewService(db, log, blobStorage, spotifyService, cfg.Destination, cfg.MusicLibraryPath, cfg.OutputFormat, cfg.Blob.Enabled, cfg.SleepInMinutes)

	if err := srv.StartProcessing(ctx); err != nil {
		log.Fatal("failed to start processing", zap.Error(err))
	}

	defer func() {
		if r := recover(); r != nil {
			log.Error("recovered from panic", zap.Any("panic", r))
			run()
		}
	}()
}
