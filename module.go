package tinywasm

import (
	"errors"
	"path/filepath"
	"strings"
)

// GetModuleName extracts module name from file path
func GetModuleName(filePath string) (string, error) {
	// Extract module name from path like: modules/users/wasm/users.wasm.go
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	for i, part := range parts {
		if part == "modules" && i+1 < len(parts) {
			return parts[i+1], nil
		}
	}
	return "", errors.New("could not extract module name from path: " + filePath)
}
