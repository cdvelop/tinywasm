package tinywasm

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Test for ShouldCompileToWasm method
func TestShouldCompileToWasm(t *testing.T) {
	// Setup test environment
	rootDir := "test"
	webDir := filepath.Join(rootDir, "wasmTest")
	defer os.RemoveAll(webDir)

	modulesDir := filepath.Join(webDir, "modules")
	var outputBuffer bytes.Buffer
	config := &Config{
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

// Test for compiler comparison functionality
func TestCompilerComparison(t *testing.T) {
	// Setup test environment
	rootDir := "test"
	webDir := filepath.Join(rootDir, "compilerTest")
	defer os.RemoveAll(webDir)

	publicDir := filepath.Join(webDir, "public")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatalf("Error creating test directory: %v", err)
	}
	// Test data for compilation
	testCases := []struct {
		name          string
		tinyGoEnabled bool
	}{
		{
			name:          "Go Standard Compiler",
			tinyGoEnabled: false,
		},
		{
			name:          "TinyGo Compiler",
			tinyGoEnabled: true,
		},
	}

	// Create main.wasm.go file for testing
	mainWasmPath := filepath.Join(webDir, "main.wasm.go")
	wasmContent := `package main
	
	func main() {
		println("Test WASM compilation")
	}`
	os.WriteFile(mainWasmPath, []byte(wasmContent), 0644)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var outputBuffer bytes.Buffer
			config := &Config{
				WebFilesFolder: func() (string, string) { return webDir, "public" },
				Log:            &outputBuffer,
				TinyGoCompiler: tc.tinyGoEnabled,
			}

			tinyWasm := New(config)

			// Test compiler detection
			if tc.tinyGoEnabled {
				// Try to enable TinyGo (might fail if not installed)
				_, err := tinyWasm.SetTinyGoCompiler(true)
				if err != nil {
					t.Logf("TinyGo not available, skipping: %v", err)
					return
				}
			}

			// Verify compiler selection
			isUsingTinyGo := tinyWasm.TinyGoCompiler()
			if tc.tinyGoEnabled && !isUsingTinyGo {
				t.Logf("TinyGo requested but not available")
			} else if !tc.tinyGoEnabled && isUsingTinyGo {
				t.Error("Expected Go standard compiler but TinyGo is selected")
			} // Test compilation (this will fail but we can check the command preparation)
			err := tinyWasm.NewFileEvent("main.wasm.go", ".go", mainWasmPath, "write")

			// Check that the correct compiler is being used
			if tc.tinyGoEnabled && tinyWasm.tinyGoInstalled {
				// For TinyGo, verify it's actually being used
				if !tinyWasm.TinyGoCompiler() {
					t.Errorf("Expected TinyGo compiler to be enabled, but it's not")
				}
			} else {
				// For Go standard, verify TinyGo is not being used
				if tinyWasm.TinyGoCompiler() {
					t.Errorf("Expected Go standard compiler, but TinyGo is enabled")
				}
			}

			// Check that the WASM project was detected (this confirms the system is working)
			if !tinyWasm.wasmProject {
				t.Errorf("Expected WASM project to be detected for %s", tc.name)
			}

			// We expect compilation to fail in test environment, that's ok
			t.Logf("Compilation test completed for %s (error expected in test env): %v", tc.name, err)
		})
	}
}
