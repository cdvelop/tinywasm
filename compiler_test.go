package tinywasm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test for ShouldCompileToWasm method
func TestShouldCompileToWasm(t *testing.T) {
	// Setup test environment using temporary directory
	rootDir := t.TempDir()
	sourceDir := filepath.Join(rootDir, "wasmTest")
	outputDir := filepath.Join(rootDir, "output")

	// modules support removed; tests operate on sourceDir directly
	var logMessages []string
	config := &Config{
		AppRootDir: rootDir,
		SourceDir:  "wasmTest",
		OutputDir:  "output",
		Logger: func(message ...any) {
			logMessages = append(logMessages, fmt.Sprint(message...))
		},
	}

	tinyWasm := New(config)
	tests := []struct {
		name     string
		fileName string
		filePath string
		expected bool
	}{ // Main WASM file cases
		{"Main WASM file", "main.go", filepath.Join(sourceDir, "main.go"), true}, // main.go in source root
		{"Main WASM file in different location", "main.go", filepath.Join("project", "main.go"), true},

		// Module WASM files
		// .wasm.go files anywhere should trigger compilation
		{"Any WASM file", "users.wasm.go", filepath.Join(sourceDir, "users.wasm.go"), true},
		{"Another WASM file", "auth.wasm.go", filepath.Join(sourceDir, "auth.wasm.go"), true},

		// Non-Go files (should NOT compile)
		{"JavaScript file", "script.js", filepath.Join(outputDir, "js", "script.js"), false},
		{"CSS file", "style.css", filepath.Join(outputDir, "css", "style.css"), false},
		{"HTML file", "index.html", filepath.Join(outputDir, "index.html"), false},

		// Root level files (should NOT compile)
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
	// Setup test environment using temporary directory
	rootDir := t.TempDir()
	webDir := filepath.Join(rootDir, "compilerTest")

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
			var logMessages []string
			config := &Config{
				AppRootDir: rootDir,
				SourceDir:  "wasmTest",
				OutputDir:  "output",
				Logger: func(message ...any) {
					logMessages = append(logMessages, fmt.Sprint(message...))
				},
			}

			tinyWasm := New(config)
			// Tests run in the same package so we can set the private flag directly
			tinyWasm.tinyGoCompiler = tc.tinyGoEnabled

			// Test compiler detection
			if tc.tinyGoEnabled {
				// Try to enable TinyGo (might fail if not installed). Use progress callback to capture messages.
				var msg string
				tinyWasm.Change("b", func(msgs ...any) {
					if len(msgs) > 0 {
						msg = fmt.Sprint(msgs...)
					}
				})
				// If TinyGo isn't available, the progress callback likely contains an error message.
				if strings.Contains(strings.ToLower(msg), "cannot") || strings.Contains(strings.ToLower(msg), "not available") {
					t.Logf("TinyGo not available, skipping: %s", msg)
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
