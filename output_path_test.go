package tinywasm

import (
	"path/filepath"
	"testing"
)

// TestOutputRelativePath verifies that OutputRelativePath returns a RELATIVE path,
// not an absolute path. This is critical for file watcher UnobservedFiles functionality.
func TestOutputRelativePath(t *testing.T) {
	tests := []struct {
		name       string
		appRootDir string
		outputDir  string
		outputName string
		expectPath string // Normalized with forward slashes for cross-platform testing
	}{
		{
			name:       "Unix absolute root",
			appRootDir: "/home/user/project",
			outputDir:  "deploy/edgeworker",
			outputName: "app",
			expectPath: "deploy/edgeworker/app.wasm",
		},
		{
			name:       "Windows absolute root",
			appRootDir: "C:\\Users\\user\\project",
			outputDir:  "deploy\\edgeworker",
			outputName: "worker",
			expectPath: "deploy/edgeworker/worker.wasm", // Normalized to forward slashes
		},
		{
			name:       "Temp directory root",
			appRootDir: "/tmp/test123",
			outputDir:  "output",
			outputName: "main",
			expectPath: "output/main.wasm",
		},
		{
			name:       "Current directory root",
			appRootDir: ".",
			outputDir:  "build",
			outputName: "app",
			expectPath: "build/app.wasm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				AppRootDir: tt.appRootDir,
				SourceDir:  "src",
				OutputDir:  tt.outputDir,
				OutputName: tt.outputName,
				Logger:     func(...any) {}, // Silent logger
			}

			tw := New(config)
			result := tw.OutputRelativePath()

			t.Logf("AppRootDir: %s", tt.appRootDir)
			t.Logf("OutputDir: %s", tt.outputDir)
			t.Logf("OutputName: %s", tt.outputName)
			t.Logf("Result: %s", result)

			// Check if path is absolute (should NOT be)
			if filepath.IsAbs(result) {
				t.Errorf("OutputRelativePath() returned ABSOLUTE path: %s (expected relative)", result)
			}

			// Verify it matches expected relative path (normalize for cross-platform)
			normalizedResult := filepath.ToSlash(result)
			normalizedExpect := filepath.ToSlash(tt.expectPath)

			if normalizedResult != normalizedExpect {
				t.Errorf("OutputRelativePath() = %s, want %s", normalizedResult, normalizedExpect)
			}

			// Additional check: ensure no leading separator
			if len(result) > 0 && (result[0] == '/' || result[0] == '\\') {
				t.Errorf("OutputRelativePath() has leading separator: %s", result)
			}
		})
	}
}

// TestOutputRelativePathConsistency verifies that OutputRelativePath returns
// consistent results across different compiler modes (coding, debug, production)
func TestOutputRelativePathConsistency(t *testing.T) {
	config := &Config{
		AppRootDir: "/test/project",
		SourceDir:  "src/cmd/webclient",
		OutputDir:  "src/web/public",
		OutputName: "main",
		Logger:     func(...any) {},
	}

	tw := New(config)
	expected := "src/web/public/main.wasm"

	// Test in coding mode (default)
	resultCoding := tw.OutputRelativePath()
	if filepath.IsAbs(resultCoding) {
		t.Errorf("Coding mode: returned absolute path: %s", resultCoding)
	}
	if filepath.ToSlash(resultCoding) != expected {
		t.Errorf("Coding mode: got %s, want %s", resultCoding, expected)
	}

	// Switch to debug mode
	tw.Change("b", func(...any) {})
	resultDebug := tw.OutputRelativePath()
	if filepath.IsAbs(resultDebug) {
		t.Errorf("Debug mode: returned absolute path: %s", resultDebug)
	}
	if filepath.ToSlash(resultDebug) != expected {
		t.Errorf("Debug mode: got %s, want %s", resultDebug, expected)
	}

	// Switch to production mode
	tw.Change("m", func(...any) {})
	resultProd := tw.OutputRelativePath()
	if filepath.IsAbs(resultProd) {
		t.Errorf("Production mode: returned absolute path: %s", resultProd)
	}
	if filepath.ToSlash(resultProd) != expected {
		t.Errorf("Production mode: got %s, want %s", resultProd, expected)
	}

	// All results should be identical
	if resultCoding != resultDebug || resultDebug != resultProd {
		t.Errorf("Inconsistent results across modes: coding=%s, debug=%s, prod=%s",
			resultCoding, resultDebug, resultProd)
	}
}
