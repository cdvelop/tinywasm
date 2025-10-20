package tinywasm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTinyWasmNewFileEvent(t *testing.T) {
	// Setup test environment with an isolated temporary directory
	rootDir := t.TempDir()
	// SourceDir should be the subfolder name under AppRootDir
	sourceDirName := "wasmTest"
	sourceDir := filepath.Join(rootDir, sourceDirName)

	outputDir := filepath.Join(rootDir, "output")
	// Create directories
	for _, dir := range []string{sourceDir, outputDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Error creating test directory: %v", err)
		}
	}

	// Write a minimal go.mod
	goModPath := filepath.Join(rootDir, "go.mod")
	goModContent := `module test

go 1.21
`
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Configure TinyWasm handler with a logger for testing output
	var logMessages []string
	config := &Config{
		AppRootDir: rootDir,
		SourceDir:  sourceDirName,
		OutputDir:  "output",
		Logger: func(message ...any) {
			logMessages = append(logMessages, fmt.Sprint(message...))
		},
	}

	tinyWasm := New(config)
	t.Run("Verify main.go compilation", func(t *testing.T) {
		mainWasmPath := filepath.Join(rootDir, sourceDirName, "main.go") // main.go in source root
		defer os.Remove(mainWasmPath)

		// Create main wasm file
		content := `package main

		func main() {
			println("Hello TinyWasm!")
		}`
		os.WriteFile(mainWasmPath, []byte(content), 0644)

		err := tinyWasm.NewFileEvent("main.go", ".go", mainWasmPath, "write")
		if err != nil {
			t.Fatal(err)
		}

		// Verify wasm file was created
		outputPath := tinyWasm.MainOutputFileAbsolutePath()
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Fatal("main.wasm file was not created")
		}
	})
	t.Run("Verify module wasm compilation now goes to main.wasm", func(t *testing.T) {
		// Create main.go in the web root first
		mainWasmPath := filepath.Join(rootDir, sourceDirName, "main.go") // main.go in source root
		mainContent := `package main

		func main() {
			println("Main WASM entry point")
		}`
		os.WriteFile(mainWasmPath, []byte(mainContent), 0644)

		// Create another .wasm.go file in sourceDir to simulate additional WASM entry
		moduleWasmPath := filepath.Join(rootDir, sourceDirName, "users.wasm.go")
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
		outputPath := tinyWasm.MainOutputFileAbsolutePath()
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Fatal("main.wasm file was not created")
		}
		// Individual per-module wasm outputs are deprecated; ensure main output exists
		oldOutputPath := tinyWasm.MainOutputFileAbsolutePath()
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
		mainWasmPath := filepath.Join(rootDir, sourceDirName, "main.wasm.go")
		err := tinyWasm.NewFileEvent("main.wasm.go", ".go", mainWasmPath, "create")
		if err != nil {
			t.Fatal("Expected no error for non-write event")
		}
	})
	t.Run("Verify TinyGo compiler is configurable", func(t *testing.T) {
		// Test initial configuration
		var logMessages []string
		config := NewConfig()
		config.AppRootDir = rootDir
		config.SourceDir = sourceDirName
		config.OutputDir = "output"
		config.Logger = func(message ...any) {
			logMessages = append(logMessages, fmt.Sprint(message...))
		}

		tinyWasm := New(config)
		// Tests run inside the package; set private tinyGoCompiler explicitly
		tinyWasm.tinyGoCompiler = false // Start with Go standard compiler

		// Verify initial state (should be coding mode)
		if tinyWasm.Value() != "L" {
			t.Fatal("Expected coding mode to be used initially")
		}

		// Test setting TinyGo compiler (debug mode) using progress callback
		var changeMsg string
		tinyWasm.Change("M", func(msgs ...any) {
			if len(msgs) > 0 {
				changeMsg = fmt.Sprint(msgs...)
			}
		})
		// If TinyGo isn't available, progress likely contains an error message
		if strings.Contains(strings.ToLower(changeMsg), "cannot") || strings.Contains(strings.ToLower(changeMsg), "not available") {
			t.Logf("TinyGo not available: %s", changeMsg)
		} else {
			// Check that we successfully switched to debug mode
			if tinyWasm.Value() != "M" {
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
	var logMessages []string
	config := &Config{
		AppRootDir: ".",
		SourceDir:  "web",
		OutputDir:  "public",
		Logger: func(message ...any) {
			logMessages = append(logMessages, fmt.Sprint(message...))
		},
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
