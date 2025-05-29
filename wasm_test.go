package tinywasm

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestTinyWasmNewFileEvent(t *testing.T) {
	// Setup test environment
	rootDir := "test"
	webDir := filepath.Join(rootDir, "wasmTest")
	defer os.RemoveAll(webDir)

	publicDir := filepath.Join(webDir, "public")
	modulesDir := filepath.Join(webDir, "modules")

	// Create directories
	for _, dir := range []string{webDir, publicDir, filepath.Join(publicDir, "wasm"), modulesDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Error creating test directory: %v", err)
		}
	}
	// Configure TinyWasm handler with a buffer for testing output
	var outputBuffer bytes.Buffer
	config := &WasmConfig{
		WebFilesFolder: func() (string, string) { return webDir, "public" },
		Log:            &outputBuffer,
	}

	tinyWasm := New(config)
	tinyWasm.ModulesFolder = filepath.Join(webDir, "modules") // override for testing

	t.Run("Verify main.wasm.go compilation", func(t *testing.T) {
		mainWasmPath := filepath.Join(publicDir, "wasm", "main.wasm.go")
		defer os.Remove(mainWasmPath)

		// Create main wasm file
		content := `package main
		
		func main() {
			println("Hello TinyWasm!")
		}`
		os.WriteFile(mainWasmPath, []byte(content), 0644)

		err := tinyWasm.NewFileEvent("main.wasm.go", ".go", mainWasmPath, "write")
		if err != nil {
			t.Fatal(err)
		}

		// Verify wasm file was created
		outputPath := tinyWasm.OutputPathMainFileWasm()
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Fatal("main.wasm file was not created")
		}
	})

	t.Run("Verify module wasm compilation", func(t *testing.T) {
		moduleName := "users"
		moduleDir := filepath.Join(modulesDir, moduleName, "wasm")
		if err := os.MkdirAll(moduleDir, 0755); err != nil {
			t.Fatal(err)
		}

		moduleWasmPath := filepath.Join(moduleDir, "users.wasm.go")
		content := `package main

		func main() {
			println("Hello Users Module with TinyWasm!")
		}`
		os.WriteFile(moduleWasmPath, []byte(content), 0644)

		err := tinyWasm.NewFileEvent("users.wasm.go", ".go", moduleWasmPath, "write")
		if err != nil {
			t.Fatal(err)
		}

		// Verify module wasm file was created
		outputPath := filepath.Join(publicDir, "wasm", "users.wasm")
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Fatal("users.wasm module file was not created")
		}
	})

	t.Run("Handle invalid file path", func(t *testing.T) {
		err := tinyWasm.NewFileEvent("invalid.go", ".go", "", "write")
		if err == nil {
			t.Fatal("Expected error for invalid file path")
		}
	})

	t.Run("Handle non-write event", func(t *testing.T) {
		mainWasmPath := filepath.Join(publicDir, "wasm", "main.wasm.go")
		err := tinyWasm.NewFileEvent("main.wasm.go", ".go", mainWasmPath, "create")
		if err != nil {
			t.Fatal("Expected no error for non-write event")
		}
	})

	t.Run("Verify TinyGo compiler is used by default", func(t *testing.T) {
		if !tinyWasm.TinyGoCompiler() {
			t.Fatal("Expected TinyGo compiler to be used by default in tinywasm package")
		}
	})
}

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
			result, err := GetModuleName(tt.filePath)

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
