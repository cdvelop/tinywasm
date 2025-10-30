package tinywasm

import (
	"strings"
	"testing"
)

func countSignatures(content string, sigs []string) int {
	c := 0
	for _, s := range sigs {
		if strings.Contains(content, s) {
			c++
		}
	}
	return c
}

func matchedSignatures(content string, sigs []string) []string {
	var out []string
	for _, s := range sigs {
		if strings.Contains(content, s) {
			out = append(out, s)
		}
	}
	return out
}

// TestJavascriptForInitializingSignatures verifies that JavascriptForInitializing()
// returns distinct JS content for Go vs TinyGo (at least one signature found each)
// and that the TinyGo usage flag changes between modes. The test is skipped
// if tinygo is not available in PATH per VerifyTinyGoInstallation requirement.
func TestJavascriptForInitializingSignatures(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		AppRootDir: tmpDir,
		Logger:     func(...any) {},
	}

	// Create TinyWasm instance with temp directory
	w := New(config)

	// Skip only if tinygo not present
	if err := w.VerifyTinyGoInstallation(); err != nil {
		t.Skipf("skipping test: tinygo not available: %v", err)
	}

	// Ensure we treat this project as a wasm project for the test
	w.wasmProject = true

	// --- TinyGo case ---
	// Set mode to debug (requires TinyGo)
	w.currentMode = w.Config.BuildMediumSizeShortcut
	// ensure installer flag
	w.tinyGoInstalled = true

	// Check that WasmProjectTinyGoJsUse reports TinyGo usage
	_, tinyUsedBefore := w.WasmProjectTinyGoJsUse()
	if !tinyUsedBefore {
		t.Fatalf("expected TinyGo usage in debug mode, WasmProjectTinyGoJsUse returned false")
	}

	tinyJs, err := w.JavascriptForInitializing()
	if err != nil {
		t.Fatalf("JavascriptForInitializing() failed for TinyGo case: %v", err)
	}

	tinyGoFound := countSignatures(tinyJs, wasm_execTinyGoSignatures())
	goFoundInTiny := countSignatures(tinyJs, wasm_execGoSignatures())
	t.Logf("TinyGo JS matched TinyGo sigs: %v", matchedSignatures(tinyJs, wasm_execTinyGoSignatures()))
	t.Logf("TinyGo JS matched Go sigs: %v", matchedSignatures(tinyJs, wasm_execGoSignatures()))

	if tinyGoFound == 0 {
		t.Fatalf("expected at least one TinyGo signature in TinyGo JS, found none")
	}
	if goFoundInTiny != 0 {
		t.Fatalf("expected no Go signatures in TinyGo JS, but found %d", goFoundInTiny)
	}

	// --- Go case ---
	// Set mode to coding (Go standard)
	w.currentMode = w.Config.BuildLargeSizeShortcut

	_, tinyUsedAfter := w.WasmProjectTinyGoJsUse()
	if tinyUsedAfter {
		t.Fatalf("expected TinyGo usage to be false in coding mode, but WasmProjectTinyGoJsUse returned true")
	}

	goJs, err := w.JavascriptForInitializing()
	if err != nil {
		t.Fatalf("JavascriptForInitializing() failed for Go case: %v", err)
	}

	goFound := countSignatures(goJs, wasm_execGoSignatures())
	tinyFoundInGo := countSignatures(goJs, wasm_execTinyGoSignatures())
	t.Logf("Go JS matched Go sigs: %v", matchedSignatures(goJs, wasm_execGoSignatures()))
	t.Logf("Go JS matched TinyGo sigs: %v", matchedSignatures(goJs, wasm_execTinyGoSignatures()))

	if goFound == 0 {
		t.Fatalf("expected at least one Go signature in Go JS, found none")
	}
	if tinyFoundInGo != 0 {
		t.Fatalf("expected no TinyGo signatures in Go JS, but found %d", tinyFoundInGo)
	}

	// Verify the TinyGo usage flag changes between the two calls
	if tinyUsedBefore == tinyUsedAfter {
		t.Fatalf("expected TinyGo usage flag to change between debug and coding modes, but it did not")
	}
}
