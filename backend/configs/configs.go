package configs

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"

	"backend/tools"
)

// LoadEnv loads variables from the .env file beside go.mod when one is
// available (a development convenience), so the process finds it regardless of
// which subdirectory it was started from. A compiled binary deployed without
// its source tree has no go.mod/.env — that is expected: real environment
// variables take precedence and are used directly, so this never fails the
// program. Load() still rejects a missing DATABASE_URL/JWT_SECRET, so nothing
// starts silently misconfigured.
func LoadEnv() {
	if rootDir, err := tools.FindRootDir(); err == nil {
		_ = godotenv.Load(filepath.Join(rootDir, ".env"))
	}
	// Fall back to a .env in the working directory if present; ignore if absent.
	_ = godotenv.Load()
}

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
