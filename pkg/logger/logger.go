package logger

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// Init configures the global slog instance.
// Production/staging environments use structured JSON output.
// Development uses colored, human-readable output via tint.
func Init(env string, levelStr string) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		level = slog.LevelInfo
	}

	var handler slog.Handler
	if env == "production" || env == "staging" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:      level,
			TimeFormat: time.TimeOnly,
		})
	}

	slog.SetDefault(slog.New(handler))
}
