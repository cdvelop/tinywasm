package tinywasm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCompileAllModes attempts to compile the WASM main file to disk
// using the three supported modes: coding (go), debugging (tinygo), production (tinygo).
// Simulates the real integration flow: InitialRegistration -> NewFileEvent -> Change modes.
// If tinygo is not present in PATH, the tinygo modes are skipped.
func TestCompileAllModes(t *testing.T) {
	// Create isolated temp workspace
	tmp := t.TempDir()
	webDir := filepath.Join(tmp, "web")
	publicDir := filepath.Join(webDir, "public")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatalf("failed to create test dirs: %v", err)
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

	// Prepare config with logger to prevent nil pointer dereference
	var outputBuffer bytes.Buffer
	cfg := &Config{
		WebFilesRootRelative: webDir,
		WebFilesSubRelative:  "public",
		TinyGoCompiler:       true, // allow tinygo when present
		Logger:               &outputBuffer,
	}

	w := New(cfg)

	// Debug: Check initial state
	t.Logf("After New() - Initial mode: %s, shortcuts: c=%s, d=%s, p=%s",
		w.Value(), w.Config.CodingShortcut, w.Config.DebuggingShortcut, w.Config.ProductionShortcut)

	// Check tinygo availability
	_, err := exec.LookPath("tinygo")
	tinygoPresent := err == nil

	// Step 1: Simulate InitialRegistration flow - notify about existing file
	// This is what devwatch does during startup
	err = w.NewFileEvent("main.wasm.go", ".go", mainWasmPath, "create")
	if err != nil {
		t.Fatalf("NewFileEvent with create event failed: %v", err)
	}
	t.Logf("After NewFileEvent create - Logger output: %s", outputBuffer.String())
	outputBuffer.Reset() // Clear buffer for next operations

	tests := []struct {
		mode         string
		name         string
		requiresTiny bool
	}{
		{mode: w.Config.CodingShortcut, name: "coding", requiresTiny: false},
		{mode: w.Config.DebuggingShortcut, name: "debugging", requiresTiny: true},
		{mode: w.Config.ProductionShortcut, name: "production", requiresTiny: true},
	}

	outPath := func() string {
		return filepath.Join(webDir, cfg.WebFilesSubRelative, "main.wasm")
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.requiresTiny && !tinygoPresent {
				t.Skipf("tinygo not in PATH; skipping %s mode", tc.name)
			}

			// Clear any previous output
			_ = os.Remove(outPath())

			// Step 2: Change compilation mode (this is how users switch modes in DevTUI)
			t.Logf("Before Change - Current mode: %s, Active builder: %T", w.Value(), w.activeBuilder)
			var progressMsg string
			w.Change(tc.mode, func(msgs ...any) {
				if len(msgs) > 0 {
					progressMsg = fmt.Sprint(msgs...)
				}
			})
			t.Logf("After Change to %s mode - Current mode: %s, Active builder: %T, Progress: %s, Logger: %s",
				tc.name, w.Value(), w.activeBuilder, progressMsg, outputBuffer.String())
			outputBuffer.Reset() // Clear buffer for next operations

			// Step 3: Simulate file modification event (this triggers recompilation)
			// This is what devwatch does when user modifies main.wasm.go
			err := w.NewFileEvent("main.wasm.go", ".go", mainWasmPath, "write")
			if err != nil {
				t.Fatalf("mode %s: NewFileEvent with write event failed: %v; progress: %s", tc.name, err, progressMsg)
			}
			t.Logf("After NewFileEvent write - Logger output: %s", outputBuffer.String())
			outputBuffer.Reset() // Clear buffer for next operations

			// Step 4: Verify output file exists on disk
			fi, err := os.Stat(outPath())
			if err != nil {
				t.Fatalf("mode %s: expected output file at %s, got error: %v; progress: %s", tc.name, outPath(), err, progressMsg)
			}
			if fi.Size() == 0 {
				t.Fatalf("mode %s: output file exists but is empty: %s", tc.name, outPath())
			}

			t.Logf("mode %s: successfully compiled %d bytes to %s; progress: %s", tc.name, fi.Size(), outPath(), progressMsg)
		})
	}
}
