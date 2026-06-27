// Package secretfile implements the <KEY>_FILE convention used by the AĒR
// services to read a credential either directly from an environment variable
// <KEY> or from a file whose path is given in <KEY>_FILE.
//
// The file form is how Docker secrets deliver credentials: Compose mounts each
// secret as a tmpfs file under /run/secrets/* and passes <KEY>_FILE pointing at
// it, so the secret value never appears in the container's on-disk config
// (/var/lib/docker/.../config.v2.json) — the goal of Phase 155 / ADR-046. The
// _FILE form takes precedence; when <KEY>_FILE is unset the plain env / .env
// value is used unchanged, so the convention is fully backward-compatible.
package secretfile

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Apply resolves the <KEY>_FILE convention for each given key into v.
//
// For every key whose <KEY>_FILE environment variable names a readable file,
// the file's contents (a single trailing newline stripped) are set as the value
// of <KEY> with viper's highest precedence (v.Set), overriding AutomaticEnv and
// the .env file. Keys with no <KEY>_FILE set are left untouched.
//
// It must run AFTER v.AutomaticEnv()/ReadInConfig() and BEFORE Unmarshal (or any
// v.Get). A configured-but-unreadable <KEY>_FILE is a hard error: the service
// must fail fast rather than boot with a missing credential, mirroring the
// boot-time secret validation in each service's config loader.
func Apply(v *viper.Viper, keys ...string) error {
	for _, key := range keys {
		path := strings.TrimSpace(os.Getenv(key + "_FILE"))
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s_FILE (%s): %w", key, path, err)
		}
		// Trim only a trailing newline (the common artefact of writing a secret
		// file with a shell heredoc / echo). Spaces and tabs are NOT trimmed —
		// a password may legitimately contain trailing whitespace.
		v.Set(key, strings.TrimRight(string(data), "\r\n"))
	}
	return nil
}
