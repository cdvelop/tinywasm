package tinywasm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHeaderUpdateBugReproduction reproduces the specific bug where switching
// from mode "d" to mode "p" doesn't update the wasm_exec.js header correctly.
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

	// Write a minimal main.wasm.go
	mainWasmPath := filepath.Join(webDir, "main.wasm.go")
	wasmContent := `package main

func main() {
    println("hello wasm")
}
`
	if err := os.WriteFile(mainWasmPath, []byte(wasmContent), 0644); err != nil {
		t.Fatalf("failed to write main.wasm.go: %v", err)
	}

	// Prepare config to match real-world setup
	var logMessages []string
	cfg := NewConfig()
	cfg.AppRootDir = tmp
	cfg.WebFilesRootRelative = webDirName
	cfg.WebFilesSubRelative = "public"
	cfg.WebFilesSubRelativeJsOutput = filepath.Join("theme", "js") // Realistic path
	cfg.Logger = func(message ...any) {
		logMessages = append(logMessages, fmt.Sprint(message...))
	}

	w := New(cfg)
	w.tinyGoCompiler = true
	w.wasmProject = true

	wasmExecPath := filepath.Join(tmp, cfg.WebFilesRootRelative, cfg.WebFilesSubRelativeJsOutput, "wasm_exec.js")

	// Step 1: Initialize with coding mode and create initial file
	err := w.NewFileEvent("main.wasm.go", ".go", mainWasmPath, "create")
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
	verifyHeader("c", "After initial creation")

	// Step 2: Change to debugging mode
	t.Log("=== Changing to debugging mode ===")
	progressCb := func(msgs ...any) {
		// Just capture progress messages
	}
	w.Change(w.Config.DebuggingShortcut, progressCb)

	if w.Value() != "d" {
		t.Errorf("Expected mode 'd', got '%s'", w.Value())
	}
	verifyHeader("d", "After changing to debugging mode")

	// Step 3: THE CRITICAL TEST - Change to production mode
	t.Log("=== Changing to production mode (THE BUG TEST) ===")
	w.Change(w.Config.ProductionShortcut, progressCb)

	if w.Value() != "p" {
		t.Errorf("Expected mode 'p', got '%s'", w.Value())
	}

	// This is where the bug should be detected
	verifyHeader("p", "After changing to production mode (CRITICAL)")

	// Step 4: Test back and forth to ensure robustness
	t.Log("=== Testing mode switching robustness ===")

	// Back to debugging
	w.Change(w.Config.DebuggingShortcut, progressCb)
	verifyHeader("d", "Back to debugging mode")

	// Back to production
	w.Change(w.Config.ProductionShortcut, progressCb)
	verifyHeader("p", "Back to production mode (second time)")

	// Back to coding
	w.Change(w.Config.CodingShortcut, progressCb)
	verifyHeader("c", "Back to coding mode")
}
