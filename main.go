package main

import (
	"context"
	"time"

	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/blob"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/config"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/db"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/service"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"go.uber.org/zap"
	"golang.org/x/oauth2/clientcredentials"
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

	spotifyConfig := clientcredentials.Config{
		ClientID:     cfg.Spotify.ClientID,
		ClientSecret: cfg.Spotify.ClientSecret,
		TokenURL:     spotifyauth.TokenURL,
	}

	token, err := spotifyConfig.Token(ctx)
	if err != nil {
		log.Fatal("failed to get token", zap.Error(err))
	}

	httpClient := spotifyauth.New().Client(context.Background(), token)
	spotifyClient := spotify.New(httpClient)

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

	srv := service.NewService(db, log, blobStorage, spotifyClient, cfg.Destination, cfg.MusicLibraryPath, cfg.Blob.Enabled, cfg.SleepInMinutes)

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
