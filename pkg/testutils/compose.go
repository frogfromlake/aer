// Package testutils holds test-only helpers shared across the Go services.
// Its job is to keep Testcontainers honest with Hard Rule #1: image tags are
// read dynamically from compose.yaml (the SSoT) rather than hardcoded in tests,
// so a tag bump never silently diverges between the running stack and the suite.
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
	_, b, _, ok := runtime.Caller(0)
	if !ok {
		// SEC-096 — without the caller path the repo-root walk below silently
		// resolves against "" and reads a bogus compose.yaml; fail explicitly.
		return "", fmt.Errorf("failed to resolve caller path for compose.yaml lookup")
	}

	// Navigate up the directory tree to the repository root
	repoRoot := filepath.Join(filepath.Dir(b), "..", "..")
	composePath := filepath.Join(repoRoot, "compose.yaml")

	return getImageFromFile(composePath, serviceName)
}

func getImageFromFile(composePath, serviceName string) (string, error) {
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
