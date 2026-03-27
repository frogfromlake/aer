package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// AppConfig holds the global configuration for a service.
type AppConfig struct {
	Environment string `mapstructure:"APP_ENV"` // e.g., "development", "staging", "production"
	LogLevel    string `mapstructure:"LOG_LEVEL"`
}

// Load reads configuration from environment variables and an optional .env file.
func Load() (*AppConfig, error) {
	v := viper.New()

	// 1. Set default values
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("LOG_LEVEL", "INFO")

	// 2. Configure environment variable reading
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 3. Attempt to load from .env file for local development
	v.SetConfigFile(".env")
	v.SetConfigType("env")

	// Ignore errors if the file is not found, as we rely on system env vars in prod/staging
	_ = v.ReadInConfig()

	// 4. Unmarshal into struct
	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
