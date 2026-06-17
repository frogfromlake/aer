package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

// captureStdout swaps os.Stdout for a pipe for the duration of fn and returns
// everything written. Init binds the handler to os.Stdout at call time, so fn
// must both call Init and emit the log line.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

func TestInit_LevelParsing(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		level     string
		wantDebug bool
		wantInfo  bool
		wantWarn  bool
	}{
		{"debug enables everything", "debug", true, true, true},
		{"info is the common default", "info", false, true, true},
		{"warn suppresses info", "warn", false, false, true},
		{"error suppresses warn", "error", false, false, false},
		{"unparseable level falls back to info", "not-a-real-level", false, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init("production", tt.level)
			l := slog.Default()
			if got := l.Enabled(ctx, slog.LevelDebug); got != tt.wantDebug {
				t.Errorf("Debug enabled = %v, want %v", got, tt.wantDebug)
			}
			if got := l.Enabled(ctx, slog.LevelInfo); got != tt.wantInfo {
				t.Errorf("Info enabled = %v, want %v", got, tt.wantInfo)
			}
			if got := l.Enabled(ctx, slog.LevelWarn); got != tt.wantWarn {
				t.Errorf("Warn enabled = %v, want %v", got, tt.wantWarn)
			}
		})
	}
}

func TestInit_StructuredEnvsEmitJSON(t *testing.T) {
	for _, env := range []string{"production", "staging"} {
		t.Run(env, func(t *testing.T) {
			out := captureStdout(t, func() {
				Init(env, "info")
				slog.Info("hello world", "key", "value")
			})
			if !strings.HasPrefix(strings.TrimSpace(out), "{") {
				t.Fatalf("%s output is not JSON: %q", env, out)
			}
			for _, want := range []string{`"msg":"hello world"`, `"key":"value"`, `"level":"INFO"`} {
				if !strings.Contains(out, want) {
					t.Errorf("JSON output missing %s: %q", want, out)
				}
			}
		})
	}
}

func TestInit_DevelopmentEmitsHumanReadable(t *testing.T) {
	out := captureStdout(t, func() {
		Init("development", "info")
		slog.Info("hello world", "key", "value")
	})
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("development output should not be JSON: %q", out)
	}
	if !strings.Contains(out, "hello world") {
		t.Errorf("development output missing message: %q", out)
	}
}
