package tinywasm_test

import (
	"testing"

	"github.com/cdvelop/tinywasm"
)

func TestGetModuleName(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
		hasError bool
	}{
		{
			name:     "Valid module path",
			filePath: "modules/users/wasm/users.wasm.go",
			expected: "users",
			hasError: false,
		},
		{
			name:     "Valid module path with prefix",
			filePath: "web/modules/auth/wasm/auth.wasm.go",
			expected: "auth",
			hasError: false,
		},
		{
			name:     "Invalid path without modules",
			filePath: "web/public/main.wasm.go",
			expected: "",
			hasError: true,
		},
		{
			name:     "Empty path",
			filePath: "",
			expected: "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tinywasm.GetModuleName(tt.filePath)

			if tt.hasError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Fatalf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}
