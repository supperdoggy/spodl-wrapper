package config

import "github.com/kelseyhightower/envconfig"

type BlobStorageConfig struct {
	Enabled bool `envconfig:"BLOB_ENABLED" required:"true"`

	AccessKeyID  string `envconfig:"S3_ACCESS_KEY"`
	SecretAccess string `envconfig:"S3_SECRET_ACCESS"`
	Region       string `envconfig:"S3_REGION"`
	Bucket       string `envconfig:"S3_BUCKET"`
	Endpoint     string `envconfig:"S3_ENDPOINT"`
}

type SpotifyConfig struct {
	ClientID     string `envconfig:"SPOTIFY_CLIENT_ID" required:"true"`
	ClientSecret string `envconfig:"SPOTIFY_CLIENT_SECRET" required:"true"`
}

type Config struct {
	Blob    BlobStorageConfig
	Spotify SpotifyConfig

	DatabaseURL      string `envconfig:"DATABASE_URL" required:"true"`
	DatabaseName     string `envconfig:"DATABASE_NAME" required:"true"`
	Destination      string `envconfig:"DESTINATION" required:"true"`
	MusicLibraryPath string `envconfig:"MUSIC_LIBRARY_PATH" required:"true"`
	SleepInMinutes   int    `envconfig:"SLEEP_IN_MINUTES" required:"true"`
}

func NewConfig() (*Config, error) {
	cfg := new(Config)
	err := envconfig.Process("", cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
