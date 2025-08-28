package tinywasm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTinyWasmNewFileEvent(t *testing.T) {
	// Setup test environment with an isolated temporary directory
	rootDir := t.TempDir()
	webDir := filepath.Join(rootDir, "wasmTest")

	publicDir := filepath.Join(webDir, "public")
	// Create directories
	for _, dir := range []string{webDir, publicDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Error creating test directory: %v", err)
		}
	}

	// Configure TinyWasm handler with a buffer for testing output
	var outputBuffer bytes.Buffer
	config := &Config{
		WebFilesRootRelative: webDir,
		WebFilesSubRelative:  "public",
		Logger:               &outputBuffer,
	}

	tinyWasm := New(config)
	t.Run("Verify main.wasm.go compilation", func(t *testing.T) {
		mainWasmPath := filepath.Join(webDir, "main.wasm.go") // main.wasm.go in web root
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
		outputPath := tinyWasm.MainInputFileRelativePath()
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Fatal("main.wasm file was not created")
		}
	})
	t.Run("Verify module wasm compilation now goes to main.wasm", func(t *testing.T) {
		// Create main.wasm.go in the web root first
		mainWasmPath := filepath.Join(webDir, "main.wasm.go") // main.wasm.go in web root
		mainContent := `package main

		func main() {
			println("Main WASM entry point")
		}`
		os.WriteFile(mainWasmPath, []byte(mainContent), 0644)

		// Create another .wasm.go file in webDir to simulate additional WASM entry
		moduleWasmPath := filepath.Join(webDir, "users.wasm.go")
		content := `package main

		func main() {
			println("Hello Users Module with TinyWasm!")
		}`
		os.WriteFile(moduleWasmPath, []byte(content), 0644)

		err := tinyWasm.NewFileEvent("users.wasm.go", ".go", moduleWasmPath, "write")
		if err != nil {
			t.Fatal(err)
		}

		// Verify main.wasm file was created (single output)
		outputPath := tinyWasm.MainInputFileRelativePath()
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Fatal("main.wasm file was not created")
		}
		// Individual per-module wasm outputs are deprecated; ensure main output exists
		oldOutputPath := tinyWasm.MainInputFileRelativePath()
		if _, err := os.Stat(oldOutputPath); os.IsNotExist(err) {
			t.Fatal("main.wasm file was not created")
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
	t.Run("Verify TinyGo compiler is configurable", func(t *testing.T) {
		// Test initial configuration
		var outputBuffer bytes.Buffer
		config := NewConfig()
		config.WebFilesRootRelative = webDir
		config.WebFilesSubRelative = "public"
		config.Logger = &outputBuffer
		config.TinyGoCompiler = false // Start with Go standard compiler

		tinyWasm := New(config)

		// Verify initial state (should be coding mode)
		if tinyWasm.Value() != "c" {
			t.Fatal("Expected coding mode to be used initially")
		}

		// Test setting TinyGo compiler (debug mode) using progress callback
		var changeMsg string
		tinyWasm.Change("d", func(msgs ...any) {
			if len(msgs) > 0 {
				changeMsg = fmt.Sprint(msgs...)
			}
		})
		// If TinyGo isn't available, progress likely contains an error message
		if strings.Contains(strings.ToLower(changeMsg), "cannot") || strings.Contains(strings.ToLower(changeMsg), "not available") {
			t.Logf("TinyGo not available: %s", changeMsg)
		} else {
			// Check that we successfully switched to debug mode
			if tinyWasm.Value() != "d" {
				t.Fatal("Expected debug mode to be set after change")
			}
			// Message can be success or warning (auto-compilation might fail in test env)
			if !strings.Contains(strings.ToLower(changeMsg), "debug") && !strings.Contains(strings.ToLower(changeMsg), "warning") {
				t.Fatalf("Expected debug mode message or warning, got: %s", changeMsg)
			}
		}
	})
}

// Test for UnobservedFiles method
func TestUnobservedFiles(t *testing.T) {
	var outputBuffer bytes.Buffer
	config := &Config{
		WebFilesRootRelative: "web",
		WebFilesSubRelative:  "public",
		Logger:               &outputBuffer,
	}

	tinyWasm := New(config)
	unobservedFiles := tinyWasm.UnobservedFiles()
	// Should contain main.wasm and main_temp.wasm (generated files from gobuild)
	expectedFiles := []string{"main.wasm", "main_temp.wasm"}
	if len(unobservedFiles) != len(expectedFiles) {
		t.Logf("Actual unobserved files: %v", unobservedFiles)
		t.Logf("Expected unobserved files: %v", expectedFiles)
		t.Fatalf("Expected %d unobserved files, got %d", len(expectedFiles), len(unobservedFiles))
	}

	for i, expected := range expectedFiles {
		if unobservedFiles[i] != expected {
			t.Errorf("Expected unobserved file %q, got %q", expected, unobservedFiles[i])
		}
	}

	// Verify main.wasm.go is NOT in unobserved files (should be watched)
	for _, file := range unobservedFiles {
		if file == "main.wasm.go" {
			t.Error("main.wasm.go should NOT be in unobserved files - it should be watched for changes")
		}
	}
}

// Test frontend prefix configuration
func TestFrontendPrefixConfiguration(t *testing.T) {
	// Frontend prefix configuration support has been removed.
}
