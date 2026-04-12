package config

import (
	"os"
	"strings"
)

type Config struct {
	DatabaseURL  string
	Port         string
	SentryDSN    string
	ShortURLBase string
}

func Load() Config {
	return Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		Port:         os.Getenv("PORT"),
		SentryDSN:    os.Getenv("SENTRY_DSN"),
		ShortURLBase: os.Getenv("SHORT_URL_BASE"),
	}
}

func (config Config) ServerAddress() string {
	if config.Port == "" {
		return ":8080"
	}
	if strings.HasPrefix(config.Port, ":") {
		return config.Port
	}

	return ":" + config.Port
}
