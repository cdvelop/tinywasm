package tinywasm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDebugWasmExecGeneration helps debug the wasm_exec.js generation issue
func TestDebugWasmExecGeneration(t *testing.T) {
	// Create temporary test directory
	testDir := t.TempDir()

	// Create a .wasm.go file
	wasmGoPath := filepath.Join(testDir, "module.wasm.go")
	content := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(wasmGoPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test .wasm.go file: %v", err)
	}

	// Create TinyWasm instance with verbose logging
	messages := []string{}
	config := &Config{
		AppRootDir:          testDir,
		SourceDir:           "web",
		OutputDir:           "theme/js",
		WasmExecJsOutputDir: "theme/js",
		Logger: func(message ...any) {
			msg := fmt.Sprint(message...)
			messages = append(messages, msg)
			t.Log("LOG:", msg)
		},
	}

	tinyWasm := New(config)

	// Print all debug messages
	t.Log("=== All debug messages ===")
	for i, msg := range messages {
		t.Logf("%d: %s", i, msg)
	}

	// Check if wasm_exec.js was created
	wasmExecPath := filepath.Join(testDir, config.OutputDir, "wasm_exec.js")
	t.Logf("Checking path: %s", wasmExecPath)

	info, err := os.Stat(wasmExecPath)
	if err != nil {
		t.Fatalf("wasm_exec.js not created: %v", err)
	}

	t.Logf("File size: %d bytes", info.Size())

	// Read and log the content
	data, err := os.ReadFile(wasmExecPath)
	if err != nil {
		t.Fatalf("Failed to read wasm_exec.js: %v", err)
	}

	content = string(data)
	t.Logf("Content length: %d", len(content))
	t.Logf("Content preview (first 500 chars):\n%s", content[:min(500, len(content))])

	// Debug the state
	t.Logf("=== TinyWasm State ===")
	t.Logf("wasmProject: %v", tinyWasm.wasmProject)
	t.Logf("tinyGoCompiler: %v", tinyWasm.tinyGoCompiler)
	t.Logf("tinyGoInstalled: %v", tinyWasm.tinyGoInstalled)
	t.Logf("currentMode: %s", tinyWasm.currentMode)

	// Check WasmProjectTinyGoJsUse
	isWasm, useTinyGo := tinyWasm.WasmProjectTinyGoJsUse()
	t.Logf("WasmProjectTinyGoJsUse: wasmProject=%v, useTinyGo=%v", isWasm, useTinyGo)

	// Check requiresTinyGo for current mode
	requiresTiny := tinyWasm.requiresTinyGo(tinyWasm.currentMode)
	t.Logf("requiresTinyGo(%s): %v", tinyWasm.currentMode, requiresTiny)

	// Check for Go signatures
	goSignatures := wasm_execGoSignatures()
	t.Logf("Checking for %d Go signatures: %v", len(goSignatures), goSignatures)

	for _, signature := range goSignatures {
		found := strings.Contains(content, signature)
		t.Logf("Signature '%s': %v", signature, found)
	}

	// Try to get the raw wasm_exec.js path
	goPath, err := tinyWasm.GetWasmExecJsPathGo()
	if err != nil {
		t.Fatalf("Failed to get Go wasm_exec.js path: %v", err)
	}
	t.Logf("Go wasm_exec.js path: %s", goPath)

	// Read the original Go file
	originalData, err := os.ReadFile(goPath)
	if err != nil {
		t.Fatalf("Failed to read original Go wasm_exec.js: %v", err)
	}

	originalContent := string(originalData)
	t.Logf("Original file length: %d", len(originalContent))

	// Check for signatures in original file
	for _, signature := range goSignatures {
		found := strings.Contains(originalContent, signature)
		t.Logf("Original signature '%s': %v", signature, found)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
