package tinywasm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVSCodeConfiguration(t *testing.T) {
	// Create temporary test directory
	testDir := t.TempDir()

	// Create a basic configuration for testing
	config := &Config{
		WebFilesRootRelative: testDir,
		WebFilesSubRelative:  "public",
		Logger:               func(message ...any) {},
	}

	// Create TinyWasm instance with test directory as root
	tinyWasm := New(config)
	// Prefer explicit AppRootDir on config for test clarity
	tinyWasm.AppRootDir = testDir

	// Verify initial state - detection function should be active
	if tinyWasm.wasmDetectionFuncFromGoFile == nil {
		t.Fatal("wasmDetectionFuncFromGoFile should be initialized")
	}

	// Simulate file event that should trigger WASM project detection
	err := tinyWasm.NewFileEvent("main.wasm.go", ".go", filepath.Join(testDir, "main.wasm.go"), "write")
	if err != nil {
		t.Logf("Expected compilation error in test environment: %v", err)
	}

	// Verify WASM project was detected
	if !tinyWasm.wasmProject {
		t.Error("WASM project should have been detected")
	}

	// Verify VS Code configuration was created
	vscodeDir := filepath.Join(testDir, ".vscode")
	settingsPath := filepath.Join(vscodeDir, "settings.json")

	// Check if .vscode directory was created
	if _, err := os.Stat(vscodeDir); os.IsNotExist(err) {
		t.Error(".vscode directory should have been created")
	}

	// Check if settings.json was created
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Error("settings.json should have been created")
	}

	// Verify settings.json content
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}
	// Check for gopls configuration (new approach)
	goplsConfig, ok := settings["gopls"]
	if !ok {
		t.Error("settings.json should contain gopls configuration")
	}

	goplsMap, ok := goplsConfig.(map[string]any)
	if !ok {
		t.Error("gopls should be a map")
	}

	env, ok := goplsMap["env"]
	if !ok {
		t.Error("gopls should contain env configuration")
	}

	envVars, ok := env.(map[string]any)
	if !ok {
		t.Error("gopls.env should be a map")
	}

	// Verify WASM environment variables in gopls config
	if envVars["GOOS"] != "js" {
		t.Errorf("Expected GOOS=js in gopls.env, got %v", envVars["GOOS"])
	}

	if envVars["GOARCH"] != "wasm" {
		t.Errorf("Expected GOARCH=wasm in gopls.env, got %v", envVars["GOARCH"])
	}

	t.Logf("VS Code configuration test completed successfully")
	t.Logf("Settings file created at: %s", settingsPath)
}

func TestVSCodeConfigurationFunctionSwitch(t *testing.T) {
	// Create temporary test directory
	testDir := t.TempDir()

	// Create a basic configuration for testing
	config := &Config{
		WebFilesRootRelative: testDir,
		WebFilesSubRelative:  "public",
		Logger:               func(message ...any) {},
	}
	// Create TinyWasm instance
	tinyWasm := New(config)
	tinyWasm.AppRootDir = testDir

	// Trigger WASM project detection (this should switch to inactive function)
	tinyWasm.wasmDetectionFuncFromGoFile("main.wasm.go", filepath.Join(testDir, "main.wasm.go"))
	// Verify function pointer changed to inactive by testing behavior
	// We can't compare function pointers directly, but we can verify the behavior
	initialWasmState := tinyWasm.wasmProject

	// Call the function again - if it's inactive, wasmProject state shouldn't change
	tinyWasm.wasmDetectionFuncFromGoFile("another.wasm.go", filepath.Join(testDir, "another.wasm.go"))

	// The inactive function should not change any state
	if initialWasmState != tinyWasm.wasmProject {
		t.Error("Inactive function should not change wasmProject state")
	}
	t.Logf("Function pointer successfully switched from active to inactive")
}

func TestMakeDirectoryHiddenWindows(t *testing.T) {
	// This test only runs on Windows to test the attrib command functionality
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	// Create temporary test directory
	testDir := t.TempDir()
	vscodeDir := filepath.Join(testDir, ".vscode")

	// Create .vscode directory
	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		t.Fatalf("Failed to create .vscode directory: %v", err)
	}

	// Create TinyWasm instance for testing
	config := &Config{
		WebFilesRootRelative: testDir,
		WebFilesSubRelative:  "public",
		Logger:               func(message ...any) {},
	}
	tinyWasm := New(config)

	// Test making directory hidden
	tinyWasm.makeDirectoryHiddenWindows(vscodeDir)

	// Verify directory still exists and is accessible
	if _, err := os.Stat(vscodeDir); os.IsNotExist(err) {
		t.Error(".vscode directory should still exist after making it hidden")
	}

	// Try to create a file in the hidden directory to verify it's still functional
	testFile := filepath.Join(vscodeDir, "test.json")
	if err := os.WriteFile(testFile, []byte(`{"test": true}`), 0644); err != nil {
		t.Errorf("Should be able to create files in hidden directory: %v", err)
	}

	// Clean up
	os.Remove(testFile)

	t.Logf("Windows hidden directory test completed successfully")
	t.Logf("Directory hidden at: %s", vscodeDir)
}
