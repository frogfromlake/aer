package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the environment variables required for the BFF API.
type Config struct {
	Environment        string `mapstructure:"APP_ENV"`
	LogLevel           string `mapstructure:"LOG_LEVEL"`
	ClickHouseUser     string `mapstructure:"CLICKHOUSE_USER"`
	ClickHousePassword string `mapstructure:"CLICKHOUSE_PASSWORD"`
	ClickHouseDB       string `mapstructure:"CLICKHOUSE_DB"`
}

// Load reads configuration from environment variables and the local .env file.
func Load() (*Config, error) {
	v := viper.New()

	v.SetDefault("APP_ENV", "development")
	v.SetDefault("LOG_LEVEL", "INFO")
	v.SetDefault("CLICKHOUSE_DB", "aer_gold")

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	_ = v.ReadInConfig()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
