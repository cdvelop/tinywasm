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
	for _, dir := range []string{webDir, publicDir, modulesDir} {
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

	t.Run("Verify TinyGo compiler is used by default", func(t *testing.T) {
		if !tinyWasm.TinyGoCompiler() {
			t.Fatal("Expected TinyGo compiler to be used by default in tinywasm package")
		}
	})
}

// Test for ShouldCompileToWasm method
func TestShouldCompileToWasm(t *testing.T) {
	// Setup test environment
	rootDir := "test"
	webDir := filepath.Join(rootDir, "wasmTest")
	defer os.RemoveAll(webDir)

	modulesDir := filepath.Join(webDir, "modules")

	var outputBuffer bytes.Buffer
	config := &WasmConfig{
		WebFilesFolder: func() (string, string) { return webDir, "public" },
		Log:            &outputBuffer,
		FrontendPrefix: []string{"f.", "frontend.", "ui."},
	}

	tinyWasm := New(config)
	tinyWasm.ModulesFolder = modulesDir // Set modules folder for testing
	tests := []struct {
		name     string
		fileName string
		filePath string
		expected bool
	}{ // Main WASM file cases
		{"Main WASM file", "main.wasm.go", filepath.Join(webDir, "main.wasm.go"), true}, // main.wasm.go in web root
		{"Main WASM file in different location", "main.wasm.go", filepath.Join("project", "main.wasm.go"), true},

		// Module WASM files
		{"Module WASM file", "users.wasm.go", filepath.Join(modulesDir, "users", "wasm", "users.wasm.go"), true},
		{"Module WASM file different name", "auth.wasm.go", filepath.Join(modulesDir, "auth", "wasm", "auth.wasm.go"), true},
		// Frontend prefix files
		{"Frontend prefix f.", "f.login.go", filepath.Join(modulesDir, "auth", "f.login.go"), true},
		{"Frontend prefix frontend.", "frontend.dashboard.go", filepath.Join(modulesDir, "admin", "frontend.dashboard.go"), true},
		{"Frontend prefix ui.", "ui.component.go", filepath.Join(modulesDir, "ui", "ui.component.go"), true},
		// Regular Go files in modules (should compile by default)
		{"Go file in modules", "api.go", filepath.Join(modulesDir, "users", "api.go"), true},
		{"Go file in nested modules", "handler.go", filepath.Join(modulesDir, "auth", "handlers", "handler.go"), true},
		// Files with backend prefixes (should NOT compile)
		{"Backend prefix b.", "b.service.go", filepath.Join(modulesDir, "users", "b.service.go"), false},
		{"Backend prefix backend.", "backend.logic.go", filepath.Join(modulesDir, "auth", "backend.logic.go"), false},
		{"Backend prefix api.", "api.service.go", filepath.Join(modulesDir, "users", "api.service.go"), false},

		// Non-Go files (should NOT compile)
		{"JavaScript file", "script.js", filepath.Join(webDir, "public", "js", "script.js"), false},
		{"CSS file", "style.css", filepath.Join(webDir, "public", "css", "style.css"), false},
		{"HTML file", "index.html", filepath.Join(webDir, "public", "index.html"), false},

		// Root level files (should NOT compile)
		{"Root level Go file", "main.go", "main.go", false},
		{"Root level config file", "config.go", "config.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tinyWasm.ShouldCompileToWasm(tt.fileName, tt.filePath)
			if result != tt.expected {
				t.Errorf("ShouldCompileToWasm(%q, %q) = %v, want %v",
					tt.fileName, tt.filePath, result, tt.expected)
			}
		})
	}
}

// Test for UnobservedFiles method
func TestUnobservedFiles(t *testing.T) {
	var outputBuffer bytes.Buffer
	config := &WasmConfig{
		WebFilesFolder: func() (string, string) { return "web", "public" },
		Log:            &outputBuffer,
	}

	tinyWasm := New(config)
	unobservedFiles := tinyWasm.UnobservedFiles()

	// Should only contain main.wasm (generated file)
	expectedFiles := []string{"main.wasm"}

	if len(unobservedFiles) != len(expectedFiles) {
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
	config := &WasmConfig{
		WebFilesFolder: func() (string, string) { return webDir, "public" },
		Log:            &outputBuffer,
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
