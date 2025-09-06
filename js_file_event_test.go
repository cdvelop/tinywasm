package tinywasm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestJSDetectionTinyGo verifies that creating a JS file under the web subfolder
// containing TinyGo signatures sets tinyGoCompiler, wasmProject and deactivates
// the detection functions.
func TestJSDetectionTinyGo(t *testing.T) {
	root := t.TempDir()
	webDir := "pwa"
	sub := "public"

	// create web/sub directories
	fullDir := filepath.Join(root, webDir, sub)
	if err := os.MkdirAll(fullDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var out bytes.Buffer
	cfg := &Config{
		AppRootDir:           root,
		WebFilesRootRelative: webDir,
		WebFilesSubRelative:  sub,
		Logger: func(message ...any) {
			fmt.Fprintln(&out, message...)
		},
	}

	w := New(cfg)

	// Ensure starting state
	w.wasmProject = false
	w.tinyGoCompiler = false

	// Write a JS file with a TinyGo signature
	jsPath := filepath.Join(fullDir, "main.js")
	content := []byte(wasm_execTinyGoSignatures()[0] + "\n")
	if err := os.WriteFile(jsPath, content, 0644); err != nil {
		t.Fatalf("write js: %v", err)
	}

	// Trigger create event
	if err := w.NewFileEvent("main.js", ".js", jsPath, "create"); err != nil {
		t.Fatalf("NewFileEvent: %v", err)
	}

	// After detection, wasmProject and tinyGoCompiler should be set
	if !w.wasmProject {
		t.Fatalf("expected wasmProject true after TinyGo JS detection")
	}
	if !w.tinyGoCompiler {
		t.Fatalf("expected tinyGoCompiler true after TinyGo JS detection")
	}

	// Reset flags and call detection function again; it should be inactive
	w.wasmProject = false
	w.tinyGoCompiler = false

	w.wasmDetectionFuncFromJsFile("main.js", ".js", jsPath, "create")

	if w.wasmProject || w.tinyGoCompiler {
		t.Fatalf("expected detection funcs to be inactive after first detection")
	}
}

// TestJSDetectionGo verifies the same behavior when Go signatures are present
func TestJSDetectionGo(t *testing.T) {
	root := t.TempDir()
	webDir := "pwa"
	sub := "public"

	fullDir := filepath.Join(root, webDir, sub)
	if err := os.MkdirAll(fullDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var out bytes.Buffer
	cfg := &Config{
		AppRootDir:           root,
		WebFilesRootRelative: webDir,
		WebFilesSubRelative:  sub,
		Logger: func(message ...any) {
			fmt.Fprintln(&out, message...)
		},
	}

	w := New(cfg)

	w.wasmProject = false
	w.tinyGoCompiler = true // default true to detect change to false

	// Write a JS file with a Go signature
	jsPath := filepath.Join(fullDir, "main.js")
	content := []byte(wasm_execGoSignatures()[0] + "\n")
	if err := os.WriteFile(jsPath, content, 0644); err != nil {
		t.Fatalf("write js: %v", err)
	}

	// Trigger create event
	if err := w.NewFileEvent("main.js", ".js", jsPath, "create"); err != nil {
		t.Fatalf("NewFileEvent: %v", err)
	}

	// After detection, wasmProject should be true and tinyGoCompiler false
	if !w.wasmProject {
		t.Fatalf("expected wasmProject true after Go JS detection")
	}
	if w.tinyGoCompiler {
		t.Fatalf("expected tinyGoCompiler false after Go JS detection")
	}

	// Reset flags and call detection function again; it should be inactive
	w.wasmProject = false
	w.tinyGoCompiler = true

	w.wasmDetectionFuncFromJsFile("main.js", ".js", jsPath, "create")

	if w.wasmProject || !w.tinyGoCompiler {
		t.Fatalf("expected detection funcs to be inactive after first detection (Go)")
	}
}
