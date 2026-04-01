package testutils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// GetImageFromCompose parses the compose.yaml at the repo root and extracts the image string.
func GetImageFromCompose(serviceName string) (string, error) {
	// runtime.Caller(0) gets the path to this exact file (e.g., /aer/pkg/testutils/compose.go)
	_, b, _, _ := runtime.Caller(0)

	// Navigate up the directory tree to the repository root
	repoRoot := filepath.Join(filepath.Dir(b), "..", "..")
	composePath := filepath.Join(repoRoot, "compose.yaml")

	file, err := os.Open(composePath)
	if err != nil {
		return "", fmt.Errorf("failed to open compose.yaml: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inService := false
	serviceIndent := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Calculate indentation by comparing length before and after stripping leading whitespace
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		// Check if we reached the desired service block
		if trimmed == serviceName+":" {
			inService = true
			serviceIndent = indent
			continue
		}

		if inService {
			// If indentation returns to the same level or higher, we exited the service block
			if indent <= serviceIndent {
				inService = false
				continue
			}
			// Extract the image string
			if strings.HasPrefix(trimmed, "image:") {
				return strings.TrimSpace(strings.TrimPrefix(trimmed, "image:")), nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("image for service '%s' not found", serviceName)
}
