package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// Custom levels or constants if needed
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
)

// TintHandler is a custom wrapper to handle beautiful terminal output.
type TintHandler struct {
	slog.Handler
}

func (h *TintHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level.String()
	levelColor := colorReset

	switch r.Level {
	case slog.LevelDebug:
		levelColor = colorBlue
	case slog.LevelInfo:
		levelColor = colorGreen
	case slog.LevelWarn:
		levelColor = colorYellow
	case slog.LevelError:
		levelColor = colorRed
	}

	// Format: TIME | LEVEL | MESSAGE | ATTRS
	timeStr := r.Time.Format("15:04:05")
	fmt.Printf("%s %s%s%s %s",
		colorCyan+timeStr+colorReset,
		levelColor+level+colorReset,
		getPadding(level),
		r.Message,
		colorReset,
	)

	// Print attributes if any
	r.Attrs(func(a slog.Attr) bool {
		fmt.Printf(" %s=%v", colorCyan+a.Key+colorReset, a.Value.Any())
		return true
	})

	fmt.Println()
	return nil
}

func getPadding(level string) string {
	if len(level) < 5 {
		return "  | "
	}
	return " | "
}

// Init configures the global slog instance.
func Init(env string, levelStr string) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		level = slog.LevelInfo
	}

	if env == "production" || env == "staging" {
		// Production: Standard JSON
		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
		slog.SetDefault(slog.New(handler))
	} else {
		// Development: Beautiful Tinted Output
		h := &TintHandler{
			Handler: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
		}
		slog.SetDefault(slog.New(h))
	}
}
