package tinywasm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHeaderUpdateBugReproduction reproduces the specific bug where switching
// from mode "b" to mode "m" doesn't update the wasm_exec.js header correctly.
// This simulates the real-world scenario where AssetMin reads the stale header.
func TestHeaderUpdateBugReproduction(t *testing.T) {
	// Create isolated temp workspace
	tmp := t.TempDir()
	webDirName := "web"
	webDir := filepath.Join(tmp, webDirName)
	publicDir := filepath.Join(webDir, "public")
	jsDir := filepath.Join(webDir, "theme", "js")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatalf("failed to create public dir: %v", err)
	}
	if err := os.MkdirAll(jsDir, 0755); err != nil {
		t.Fatalf("failed to create js dir: %v", err)
	}

	// Write a minimal go.mod
	goModContent := `module test

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Write a minimal main.go
	mainWasmPath := filepath.Join(webDir, "main.go")
	wasmContent := `package main

func main() {
    println("hello wasm")
}
`
	if err := os.WriteFile(mainWasmPath, []byte(wasmContent), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// Prepare config to match real-world setup
	var logMessages []string
	cfg := NewConfig()
	cfg.AppRootDir = tmp
	cfg.SourceDir = webDirName
	cfg.OutputDir = filepath.Join(webDirName, "public")
	cfg.WasmExecJsOutputDir = filepath.Join(webDirName, "theme", "js")
	cfg.Logger = func(message ...any) {
		logMessages = append(logMessages, fmt.Sprint(message...))
	}

	w := New(cfg)
	w.tinyGoCompiler = true
	w.wasmProject = true

	wasmExecPath := filepath.Join(tmp, cfg.WasmExecJsOutputDir, "wasm_exec.js")

	// Step 1: Initialize with coding mode and create initial file
	err := w.NewFileEvent("main.go", ".go", mainWasmPath, "create")
	if err != nil {
		t.Fatalf("NewFileEvent with create event failed: %v", err)
	}

	// Verify initial mode header
	verifyHeader := func(expectedMode string, step string) {
		t.Helper()
		if data, err := os.ReadFile(wasmExecPath); err != nil {
			t.Errorf("%s: Failed to read wasm_exec.js: %v", step, err)
		} else {
			content := string(data)
			expectedHeader := fmt.Sprintf("// TinyWasm: mode=%s", expectedMode)
			lines := strings.Split(content, "\n")
			actualFirstLine := ""
			if len(lines) > 0 {
				actualFirstLine = strings.TrimSpace(lines[0])
			}

			if !strings.Contains(actualFirstLine, expectedHeader) {
				t.Errorf("%s: Header mismatch. Expected: '%s', got: '%s'",
					step, expectedHeader, actualFirstLine)
			} else {
				t.Logf("%s: Header correctly shows: %s", step, actualFirstLine)
			}
		}
	}

	// Initial state should be coding mode
	verifyHeader("f", "After initial creation")

	// Step 2: Change to debugging mode
	t.Log("=== Changing to debugging mode ===")
	progressCb := func(msgs ...any) {
		// Just capture progress messages
	}
	w.Change(w.Config.BuildBugShortcut, progressCb)

	if w.Value() != "b" {
		t.Errorf("Expected mode 'b', got '%s'", w.Value())
	}
	verifyHeader("b", "After changing to debugging mode")

	// Step 3: THE CRITICAL TEST - Change to production mode
	t.Log("=== Changing to production mode (THE BUG TEST) ===")
	w.Change(w.Config.BuildMinimalShortcut, progressCb)

	if w.Value() != "m" {
		t.Errorf("Expected mode 'm', got '%s'", w.Value())
	}

	// This is where the bug should be detected
	verifyHeader("m", "After changing to production mode (CRITICAL)")

	// Step 4: Test back and forth to ensure robustness
	t.Log("=== Testing mode switching robustness ===")

	// Back to debugging
	w.Change(w.Config.BuildBugShortcut, progressCb)
	verifyHeader("b", "Back to debugging mode")

	// Back to production
	w.Change(w.Config.BuildMinimalShortcut, progressCb)
	verifyHeader("m", "Back to production mode (second time)")

	// Back to coding
	w.Change(w.Config.BuildFastShortcut, progressCb)
	verifyHeader("f", "Back to coding mode")
}
