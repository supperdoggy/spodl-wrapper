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

type Config struct {
	Blob BlobStorageConfig

	DatabaseURL  string `envconfig:"DATABASE_URL" required:"true"`
	DatabaseName string `envconfig:"DATABASE_NAME" required:"true"`
	Destination  string `envconfig:"DESTINATION" required:"true"`
}

func NewConfig() (*Config, error) {
	cfg := new(Config)
	err := envconfig.Process("", cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
