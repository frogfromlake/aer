package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image string `yaml:"image"`
}

// GetImageFromCompose parses the compose.yaml at the repo root and extracts the image string.
func GetImageFromCompose(serviceName string) (string, error) {
	// runtime.Caller(0) gets the path to this exact file (e.g., /aer/pkg/testutils/compose.go)
	_, b, _, _ := runtime.Caller(0)

	// Navigate up the directory tree to the repository root
	repoRoot := filepath.Join(filepath.Dir(b), "..", "..")
	composePath := filepath.Join(repoRoot, "compose.yaml")

	data, err := os.ReadFile(composePath)
	if err != nil {
		return "", fmt.Errorf("failed to open compose.yaml: %w", err)
	}

	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return "", fmt.Errorf("failed to parse compose.yaml: %w", err)
	}

	svc, ok := compose.Services[serviceName]
	if !ok || svc.Image == "" {
		return "", fmt.Errorf("image for service '%s' not found", serviceName)
	}
	return svc.Image, nil
}
