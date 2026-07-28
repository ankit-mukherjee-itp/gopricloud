package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds all runtime configuration, sourced from environment
// variables so the same binary works across environments without rebuilds.
type Config struct {
	Port string

	DatabaseURL string

	JWTSecret       string
	JWTIssuer       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// OSCloudName is the cloud entry (in clouds.yaml) used to authenticate
	// against OpenStack.
	OSCloudName string
}

// Load reads configuration from the environment, applying sane defaults for
// everything except the values that must be explicit in production
// (DATABASE_URL, JWT_SECRET).
func Load() (*Config, error) {
	database_url := os.Getenv("DATABASE_URL")

	cfg := &Config{
		Port:            os.Getenv("PORT"),
		DatabaseURL:     database_url,
		JWTSecret:       getEnv("JWT_SECRET", ""),
		JWTIssuer:       getEnv("JWT_ISSUER", "gopricloud"),
		AccessTokenTTL:  5 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		OSCloudName:     getEnv("OS_CLOUD_NAME", "openstack"),
	}

	if cfg.DatabaseURL == "" {
		fmt.Println("ok")
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
