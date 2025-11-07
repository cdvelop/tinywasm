package tinywasm

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWasmExecFiles(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "goflare_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a TinyWasm instance for testing
	tw := New(&Config{
		AppRootDir:    tempDir,
		MainInputFile: "main.go",
	})

	// Test data
	tests := []struct {
		name         string
		getPathFunc  func() (string, error)
		expectedFile string
	}{
		{
			name:         "Go wasm_exec.js",
			getPathFunc:  tw.GetWasmExecJsPathGo,
			expectedFile: "wasm_exec_go.js",
		},
		{
			name:         "TinyGo wasm_exec.js",
			getPathFunc:  tw.GetWasmExecJsPathTinyGo,
			expectedFile: "wasm_exec_tinygo.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the source path
			sourcePath, err := tt.getPathFunc()
			if err != nil {
				if strings.Contains(err.Error(), "tinygo executable not found") {
					t.Skipf("Skipping test: %v", err)
				}
				t.Fatalf("Failed to get source path: %v", err)
			}

			// Check if source file exists
			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				t.Fatalf("Source file does not exist: %s", sourcePath)
			}

			// Define destination path
			destPath := filepath.Join(tempDir, tt.expectedFile)

			// Test 1: File should not exist initially
			if _, err := os.Stat(destPath); !os.IsNotExist(err) {
				t.Errorf("Expected file %s to not exist initially", destPath)
			}

			// Copy the file
			err = copyWasmExecFile(sourcePath, destPath)
			if err != nil {
				t.Fatalf("Failed to copy file: %v", err)
			}

			// Test 2: File should exist after copying
			if _, err := os.Stat(destPath); os.IsNotExist(err) {
				t.Errorf("Expected file %s to exist after copying", destPath)
			}

			// Test 3: File content should match
			sourceContent, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatalf("Failed to read source file: %v", err)
			}

			destContent, err := os.ReadFile(destPath)
			if err != nil {
				t.Fatalf("Failed to read destination file: %v", err)
			}

			if string(sourceContent) != string(destContent) {
				t.Errorf("File content does not match")
			}

			// Test 4: Test file update when source changes
			// Create a temporary source file for testing updates
			tempSourcePath := filepath.Join(tempDir, "temp_"+tt.expectedFile)
			err = os.WriteFile(tempSourcePath, sourceContent, 0644)
			if err != nil {
				t.Fatalf("Failed to create temp source file: %v", err)
			}

			// Modify temp source file
			modifiedContent := string(sourceContent) + "\n// Modified for test"
			err = os.WriteFile(tempSourcePath, []byte(modifiedContent), 0644)
			if err != nil {
				t.Fatalf("Failed to modify temp source file: %v", err)
			}

			// Copy again - should update
			err = copyWasmExecFile(tempSourcePath, destPath)
			if err != nil {
				t.Fatalf("Failed to copy modified file: %v", err)
			}

			// Check if destination was updated
			updatedContent, err := os.ReadFile(destPath)
			if err != nil {
				t.Fatalf("Failed to read updated file: %v", err)
			}

			if string(updatedContent) != modifiedContent {
				t.Errorf("File was not updated with new content")
			}
		})
	}
}

func TestWasmExecFileVersions(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "goflare_version_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a TinyWasm instance
	tw := New(&Config{
		AppRootDir:    tempDir,
		MainInputFile: "main.go",
	})

	// Test version checking
	tests := []struct {
		name         string
		getPathFunc  func() (string, error)
		expectedFile string
	}{
		{
			name:         "Go version check",
			getPathFunc:  tw.GetWasmExecJsPathGo,
			expectedFile: "wasm_exec_go.js",
		},
		{
			name:         "TinyGo version check",
			getPathFunc:  tw.GetWasmExecJsPathTinyGo,
			expectedFile: "wasm_exec_tinygo.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourcePath, err := tt.getPathFunc()
			if err != nil {
				if strings.Contains(err.Error(), "tinygo executable not found") {
					t.Skipf("Skipping test: %v", err)
				}
				t.Fatalf("Failed to get source path: %v", err)
			}

			destPath := filepath.Join(tempDir, tt.expectedFile)

			// Read original source content
			sourceContent, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatalf("Failed to read source: %v", err)
			}

			// Copy initial version
			err = copyWasmExecFile(sourcePath, destPath)
			if err != nil {
				t.Fatalf("Failed to copy initial file: %v", err)
			}

			// Get initial hash
			initialHash, err := getFileHash(destPath)
			if err != nil {
				t.Fatalf("Failed to get initial hash: %v", err)
			}

			// Create a temporary modified version for testing
			tempSourcePath := filepath.Join(tempDir, "temp_"+tt.expectedFile)
			modifiedContent := string(sourceContent) + "\n// Version update"
			err = os.WriteFile(tempSourcePath, []byte(modifiedContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create modified temp file: %v", err)
			}

			// Copy modified version - should detect change
			err = copyWasmExecFile(tempSourcePath, destPath)
			if err != nil {
				t.Fatalf("Failed to copy updated file: %v", err)
			}

			// Get updated hash
			updatedHash, err := getFileHash(destPath)
			if err != nil {
				t.Fatalf("Failed to get updated hash: %v", err)
			}

			// Hashes should be different
			if initialHash == updatedHash {
				t.Errorf("File hash should have changed after version update")
			}
		})
	}
}

// copyWasmExecFile copies a wasm_exec.js file from source to destination
func copyWasmExecFile(sourcePath, destPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// getFileHash returns MD5 hash of file content
func getFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// TestEnsureWasmExecFilesExists tests the main functionality:
// - Check if wasm_exec files exist in assets directory
// - Create them if they don't exist
// - Update them if versions have changed
func TestEnsureWasmExecFilesExists(t *testing.T) {
	// Use a temporary directory for assets
	tempDir := t.TempDir()
	assetsDir := filepath.Join(tempDir, "assets")

	// Create assets directory if it doesn't exist
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("Failed to create assets dir: %v", err)
	}

	// Create a TinyWasm instance
	tw := New(&Config{
		AppRootDir:    assetsDir,
		MainInputFile: "main.go",
	})

	// Test data
	tests := []struct {
		name         string
		getPathFunc  func() (string, error)
		expectedFile string
	}{
		{
			name:         "Go wasm_exec.js",
			getPathFunc:  tw.GetWasmExecJsPathGo,
			expectedFile: "wasm_exec_go.js",
		},
		{
			name:         "TinyGo wasm_exec.js",
			getPathFunc:  tw.GetWasmExecJsPathTinyGo,
			expectedFile: "wasm_exec_tinygo.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get source path
			sourcePath, err := tt.getPathFunc()
			if err != nil {
				if strings.Contains(err.Error(), "tinygo executable not found") {
					t.Skipf("Skipping test: %v", err)
				}
				t.Fatalf("Failed to get source path: %v", err)
			}

			// Verify source exists
			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				t.Fatalf("Source file does not exist: %s", sourcePath)
			}

			destPath := filepath.Join(assetsDir, tt.expectedFile)

			// Call ensure function
			err = ensureWasmExecFile(tt.getPathFunc, destPath)
			if err != nil {
				t.Fatalf("Failed to ensure file exists: %v", err)
			}

			// Test 1: File should exist now
			if _, err := os.Stat(destPath); os.IsNotExist(err) {
				t.Errorf("Expected file %s to exist after ensure", destPath)
			}

			// Test 2: If file didn't exist initially, or if it was updated, verify content
			finalHash, err := getFileHash(destPath)
			if err != nil {
				t.Fatalf("Failed to get final hash: %v", err)
			}

			sourceHash, err := getFileHash(sourcePath)
			if err != nil {
				t.Fatalf("Failed to get source hash: %v", err)
			}

			if finalHash != sourceHash {
				t.Errorf("Final file hash doesn't match source hash")
			}

			// Test 3: Call ensure again (should not change anything if source hasn't changed)
			err = ensureWasmExecFile(tt.getPathFunc, destPath)
			if err != nil {
				t.Fatalf("Failed to ensure file exists (second call): %v", err)
			}

			// Hash should still be the same
			sameHash, err := getFileHash(destPath)
			if err != nil {
				t.Fatalf("Failed to get same hash: %v", err)
			}

			if finalHash != sameHash {
				t.Errorf("File hash changed when it shouldn't have")
			}

			// Log that file is now in assets directory
			t.Logf("File %s is now available in assets directory", tt.expectedFile)
		})
	}
}

// ensureWasmExecFile ensures a wasm_exec file exists and is up to date
// This implements the main functionality requested by the user
func ensureWasmExecFile(getPathFunc func() (string, error), destPath string) error {
	// Get source path
	sourcePath, err := getPathFunc()
	if err != nil {
		return fmt.Errorf("failed to get source path: %w", err)
	}

	// Check if source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source file does not exist: %s", sourcePath)
	}

	// Check if destination exists
	destExists := true
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		destExists = false
	}

	// If destination doesn't exist, copy it
	if !destExists {
		return copyWasmExecFile(sourcePath, destPath)
	}

	// If destination exists, check if it needs updating
	sourceHash, err := getFileHash(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to get source hash: %w", err)
	}

	destHash, err := getFileHash(destPath)
	if err != nil {
		return fmt.Errorf("failed to get destination hash: %w", err)
	}

	// If hashes are different, update the file
	if sourceHash != destHash {
		return copyWasmExecFile(sourcePath, destPath)
	}

	// File is already up to date
	return nil
}
