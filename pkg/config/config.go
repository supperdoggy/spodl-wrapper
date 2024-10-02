package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
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
