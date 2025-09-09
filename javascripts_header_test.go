package tinywasm

import (
	"os"
	"path/filepath"
	"testing"
)

// TestJavascriptHeaderRoundtrip ensures the generated wasm_exec.js contains a
// TinyWasm header with the getSuccessMessage text and that analyzeWasmExecJsContent
// can read it back and restore the mode.
func TestJavascriptHeaderRoundtrip(t *testing.T) {
	w := New(nil)
	w.wasmProject = true

	// Test all three supported shortcuts: coding, debugging, production
	shortcuts := []string{
		w.Config.CodingShortcut,
		w.Config.DebuggingShortcut,
		w.Config.ProductionShortcut,
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "wasm_exec.js")

	for _, mode := range shortcuts {
		// Use a fresh TinyWasm instance per mode to avoid shared state
		w := New(nil)
		w.wasmProject = true
		// Set mode and ensure TinyGo installed flag is true for modes that may require it
		w.currentMode = mode
		w.tinyGoInstalled = true

		js, err := w.javascriptForInitializing()
		if err != nil {
			t.Fatalf("failed to generate js for mode %q: %v", mode, err)
		}

		if err := os.WriteFile(outPath, []byte(js), 0644); err != nil {
			t.Fatalf("failed to write temp wasm_exec.js for mode %q: %v", mode, err)
		}

		// Reset currentMode to ensure detection reads the header
		w.currentMode = ""

		if !w.analyzeWasmExecJsContent(outPath) {
			t.Fatalf("analyzeWasmExecJsContent failed to detect header for mode %q", mode)
		}

		if w.Value() != mode {
			t.Fatalf("expected recovered mode %q, got %q", mode, w.Value())
		}
	}
}
