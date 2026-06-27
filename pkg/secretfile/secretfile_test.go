package secretfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestApply_ReadsValueFromFileAndOverridesEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api_key")
	if err := os.WriteFile(path, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	v := viper.New()
	v.Set("API_KEY", "env-secret") // simulate an AutomaticEnv/.env value
	t.Setenv("API_KEY_FILE", path)

	if err := Apply(v, "API_KEY"); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := v.GetString("API_KEY"); got != "file-secret" {
		t.Errorf("expected file value to win, got %q", got)
	}
}

func TestApply_StripsOnlyTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pw")
	// trailing spaces must be preserved; only the trailing newline is stripped
	if err := os.WriteFile(path, []byte("p4ss  \r\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	v := viper.New()
	t.Setenv("PW_FILE", path)
	if err := Apply(v, "PW"); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := v.GetString("PW"); got != "p4ss  " {
		t.Errorf("expected trailing spaces preserved, newline stripped; got %q", got)
	}
}

func TestApply_NoFileEnvLeavesValueUntouched(t *testing.T) {
	v := viper.New()
	v.Set("TOKEN", "plain-env")
	// no TOKEN_FILE set
	if err := Apply(v, "TOKEN"); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := v.GetString("TOKEN"); got != "plain-env" {
		t.Errorf("expected env value untouched, got %q", got)
	}
}

func TestApply_UnreadableFileIsHardError(t *testing.T) {
	v := viper.New()
	t.Setenv("SECRET_FILE", filepath.Join(t.TempDir(), "does-not-exist"))
	if err := Apply(v, "SECRET"); err == nil {
		t.Fatal("expected error for unreadable _FILE, got nil")
	}
}

func TestApply_BlankFileEnvIsIgnored(t *testing.T) {
	v := viper.New()
	v.Set("K", "v")
	t.Setenv("K_FILE", "   ") // whitespace-only path is treated as unset
	if err := Apply(v, "K"); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := v.GetString("K"); got != "v" {
		t.Errorf("blank _FILE should be ignored, got %q", got)
	}
}
