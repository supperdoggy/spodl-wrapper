package main

import (
	"context"

	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/config"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/db"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/service"
	"go.uber.org/zap"
)

func main() {
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

	db, err := db.NewDatabase(ctx, log, cfg.DatabaseURL, cfg.DatabaseName)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	log.Info("connected to database")

	srv := service.NewService(db, log)

	srv.StartProcessing(ctx)
}
