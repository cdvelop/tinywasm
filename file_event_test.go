package tinywasm

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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
	for _, dir := range []string{webDir, publicDir, modulesDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Error creating test directory: %v", err)
		}
	}

	// Configure TinyWasm handler with a buffer for testing output
	var outputBuffer bytes.Buffer
	config := &Config{
		WebFilesFolder: func() (string, string) { return webDir, "public" },
		Writer:         &outputBuffer,
	}

	tinyWasm := New(config)
	tinyWasm.ModulesFolder = filepath.Join(webDir, "modules") // override for testing
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
		outputPath := tinyWasm.OutputPathMainFileWasm()
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Fatal("main.wasm file was not created")
		}
	})
	t.Run("Verify module wasm compilation now goes to main.wasm", func(t *testing.T) {
		moduleName := "users"
		moduleDir := filepath.Join(modulesDir, moduleName, "wasm")
		if err := os.MkdirAll(moduleDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Create main.wasm.go in the web root first
		mainWasmPath := filepath.Join(webDir, "main.wasm.go") // main.wasm.go in web root
		mainContent := `package main

		func main() {
			println("Main WASM entry point")
		}`
		os.WriteFile(mainWasmPath, []byte(mainContent), 0644)

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

		// Verify main.wasm file was created (single output)
		outputPath := tinyWasm.OutputPathMainFileWasm()
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Fatal("main.wasm file was not created")
		}
		// Verify individual module wasm files are NOT created anymore (no wasm subdirectory)
		oldOutputPath := filepath.Join(publicDir, "users.wasm")
		if _, err := os.Stat(oldOutputPath); !os.IsNotExist(err) {
			t.Fatal("Individual module wasm file should not be created in new single compilation mode")
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
		config.WebFilesFolder = func() (string, string) { return webDir, "public" }
		config.Writer = &outputBuffer
		config.TinyGoCompiler = false // Start with Go standard compiler

		tinyWasm := New(config)

		// Verify initial state (should be coding mode)
		if tinyWasm.Value() != "c" {
			t.Fatal("Expected coding mode to be used initially")
		}

		// Test setting TinyGo compiler (debug mode)
		msg, err := tinyWasm.Change("d") // debug mode
		if err != nil {
			// TinyGo might not be installed, which is ok for testing
			t.Logf("TinyGo not available: %v", err)
		} else {
			// Check that we successfully switched to debug mode
			if tinyWasm.Value() != "d" {
				t.Fatal("Expected debug mode to be set after change")
			}
			// Message can be success or warning (auto-compilation might fail in test env)
			if !strings.Contains(msg, "debug") && !strings.Contains(msg, "Warning") {
				t.Fatalf("Expected debug mode message or warning, got: %s", msg)
			}
		}

		// Test invalid type
		_, err = tinyWasm.Change(123) // invalid type
		if err == nil {
			t.Fatal("Expected error when setting invalid type")
		}

		// Test invalid mode
		_, err = tinyWasm.Change("x") // invalid mode
		if err == nil {
			t.Fatal("Expected error when setting invalid mode")
		}
	})
}

// Test for UnobservedFiles method
func TestUnobservedFiles(t *testing.T) {
	var outputBuffer bytes.Buffer
	config := &Config{
		WebFilesFolder: func() (string, string) { return "web", "public" },
		Writer:         &outputBuffer,
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
	// Setup test environment
	rootDir := "test"
	webDir := filepath.Join(rootDir, "wasmTest2")
	defer os.RemoveAll(webDir)

	modulesDir := filepath.Join(webDir, "modules")

	var outputBuffer bytes.Buffer
	// Test with custom frontend prefixes
	config := &Config{
		WebFilesFolder: func() (string, string) { return webDir, "public" },
		Writer:         &outputBuffer,
		FrontendPrefix: []string{"client.", "view.", "component."},
	}

	tinyWasm := New(config)
	tinyWasm.ModulesFolder = modulesDir // Set modules folder for testing

	testCases := []struct {
		fileName string
		filePath string
		expected bool
	}{
		{"client.auth.go", filepath.Join(modulesDir, "auth", "client.auth.go"), true},
		{"view.dashboard.go", filepath.Join(modulesDir, "admin", "view.dashboard.go"), true},
		{"component.header.go", filepath.Join(modulesDir, "ui", "component.header.go"), true},
		{"server.auth.go", filepath.Join(modulesDir, "auth", "server.auth.go"), false}, // Unknown prefix "server." - not in frontend list
		{"model.user.go", filepath.Join(modulesDir, "users", "model.user.go"), false},  // Unknown prefix "model." - not in frontend list
	}

	for _, tc := range testCases {
		t.Run(tc.fileName, func(t *testing.T) {
			result := tinyWasm.ShouldCompileToWasm(tc.fileName, tc.filePath)
			if result != tc.expected {
				t.Errorf("ShouldCompileToWasm(%q, %q) = %v, want %v",
					tc.fileName, tc.filePath, result, tc.expected)
			}
		})
	}
}
