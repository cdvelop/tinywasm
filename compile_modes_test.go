package tinywasm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCompileAllModes attempts to compile the WASM main file to disk
// using the three supported modes: fast (go), debugging (tinygo), minimal (tinygo).
// Simulates the real integration flow: InitialRegistration -> NewFileEvent -> Change modes.
// If tinygo is not present in PATH, the tinygo modes are skipped.
func TestCompileAllModes(t *testing.T) {
	// Create isolated temp workspace
	tmp := t.TempDir()
	webDirName := "web"
	webDir := filepath.Join(tmp, webDirName)
	publicDir := filepath.Join(webDir, "public")
	jsDir := filepath.Join(webDir, "theme", "js")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatalf("failed to create test dirs: %v", err)
	}
	if err := os.MkdirAll(jsDir, 0755); err != nil {
		t.Fatalf("failed to create test dirs: %v", err)
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

	// Prepare config with logger to prevent nil pointer dereference
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
	// Allow tests to enable tinygo detection by setting the private field
	w.tinyGoCompiler = true

	// Debug: Check initial state
	if w.Value() != w.Config.BuildLargeSizeShortcut {
		t.Fatalf("Initial mode should be '%s', got '%s'", w.Config.BuildLargeSizeShortcut, w.Value())
	}

	// Check tinygo availability
	_, err := exec.LookPath("tinygo")
	tinygoPresent := err == nil

	// Step 1: Simulate InitialRegistration flow - notify about existing file
	err = w.NewFileEvent("main.go", ".go", mainWasmPath, "create")
	if err != nil {
		t.Fatalf("NewFileEvent with create event failed: %v", err)
	}

	outPath := func() string {
		return filepath.Join(tmp, cfg.OutputDir, "main.wasm")
	}

	// Initial compile in coding mode to get a baseline file size
	fi, err := os.Stat(outPath())
	if err != nil {
		t.Fatalf("coding mode: expected output file at %s, got error: %v", outPath(), err)
	}
	codingModeFileSize := fi.Size()
	if codingModeFileSize == 0 {
		t.Fatalf("coding mode: output file exists but is empty: %s", outPath())
	}

	// Test JavaScript generation for initial coding mode (Go compiler)
	goJS, err := w.JavascriptForInitializing()
	if err != nil {
		t.Errorf("coding mode: JavascriptForInitializing failed: %v", err)
	} else {
		if len(goJS) == 0 {
			t.Errorf("coding mode: JavascriptForInitializing returned empty JavaScript")
		}
	}

	// Test cases for mode switching
	tests := []struct {
		mode         string
		name         string
		requiresTiny bool
		assertSize   func(t *testing.T, size int64)
	}{
		{
			mode: w.Config.BuildMediumSizeShortcut, name: "debugging", requiresTiny: true,
			assertSize: func(t *testing.T, size int64) {
				if size == codingModeFileSize {
					t.Errorf("debugging mode file size (%d) should be different from coding mode size (%d)", size, codingModeFileSize)
				}
			},
		},
		{
			mode: w.Config.BuildSmallSizeShortcut, name: "production", requiresTiny: true,
			assertSize: func(t *testing.T, size int64) {
				if size == codingModeFileSize {
					t.Errorf("production mode file size (%d) should be different from coding mode size (%d)", size, codingModeFileSize)
				}
				// Production should be smaller than debug, but let's check against coding for simplicity
				if size >= codingModeFileSize {
					t.Errorf("production mode file size (%d) should be smaller than coding mode size (%d)", size, codingModeFileSize)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.requiresTiny && !tinygoPresent {
				t.Skipf("tinygo not in PATH; skipping %s mode", tc.name)
			}

			// Clear all JavaScript caches before each subtest to ensure clean state
			w.ClearJavaScriptCache()

			// Step 2: Change compilation mode
			progressChan := make(chan string, 5)
			var progressMsg string
			done := make(chan bool)

			go func() {
				for msg := range progressChan {
					progressMsg = msg // Capture the last message
				}
				done <- true
			}()

			w.Change(tc.mode, progressChan)
			<-done // Wait for the goroutine to finish

			// Assert that the internal mode has changed
			if w.Value() != tc.mode {
				t.Fatalf("After Change, expected mode '%s', got '%s'", tc.mode, w.Value())
			}

			// CRITICAL: Verify that the wasm_exec.js header reflects the new mode
			wasmExecPath := filepath.Join(tmp, cfg.WasmExecJsOutputDir, "wasm_exec.js")
			if data, err := os.ReadFile(wasmExecPath); err != nil {
				t.Errorf("Failed to read wasm_exec.js after mode change to %s: %v", tc.name, err)
			} else {
				content := string(data)
				expectedHeader := fmt.Sprintf("// TinyWasm: mode=%s", tc.mode)
				if !strings.Contains(content, expectedHeader) {
					// This is the bug we're looking for
					lines := strings.Split(content, "\n")
					actualFirstLine := ""
					if len(lines) > 0 {
						actualFirstLine = lines[0]
					}
					t.Errorf("Mode %s: wasm_exec.js header mismatch. Expected: '%s', got first line: '%s'",
						tc.name, expectedHeader, actualFirstLine)
				}
			}

			// Test JavaScript generation after mode change
			modeJS, err := w.JavascriptForInitializing()
			if err != nil {
				t.Errorf("%s mode: JavascriptForInitializing failed: %v", tc.name, err)
				return
			}

			if len(modeJS) == 0 {
				t.Errorf("%s mode: JavascriptForInitializing returned empty JavaScript", tc.name)
				return
			}

			// Clear cache to test fresh generation
			w.ClearJavaScriptCache()

			// Test again to verify cache clearing works
			freshJS, freshErr := w.JavascriptForInitializing()
			if freshErr != nil {
				t.Errorf("%s mode: JavascriptForInitializing after cache clear failed: %v", tc.name, freshErr)
			} else if modeJS != freshJS {
				t.Errorf("%s mode: JavaScript differs after cache clear (length %d vs %d)", tc.name, len(modeJS), len(freshJS))
			}

			// Step 3: Simulate file modification event to trigger re-compilation
			err = w.NewFileEvent("main.go", ".go", mainWasmPath, "write")
			if err != nil {
				t.Fatalf("mode %s: NewFileEvent with write event failed: %v; progress: %s", tc.name, err, progressMsg)
			}

			// Step 4: Verify output file and its size
			fi, err := os.Stat(outPath())
			if err != nil {
				t.Fatalf("mode %s: expected output file at %s, got error: %v; progress: %s", tc.name, outPath(), err, progressMsg)
			}

			// Use the specific assertion for the test case
			tc.assertSize(t, fi.Size())

		})
	}

	// Verify that Go and TinyGo generate different JavaScript
	if tinygoPresent {
		// Switch to a TinyGo mode to get TinyGo JavaScript
		progressChan := make(chan string, 1)
		done := make(chan bool)
		go func() {
			<-progressChan // Drain the channel
			done <- true
		}()
		w.Change(w.Config.BuildMediumSizeShortcut, progressChan)
		<-done

		tinygoJS, err := w.JavascriptForInitializing()
		if err != nil {
			t.Errorf("Failed to get TinyGo JavaScript: %v", err)
		} else if len(tinygoJS) > 0 && len(goJS) > 0 {
			if goJS == tinygoJS {
				t.Errorf("Go and TinyGo should generate different JavaScript but they are identical (lengths: Go=%d, TinyGo=%d)", len(goJS), len(tinygoJS))
			} else {
				t.Logf("SUCCESS: Go and TinyGo generate different JavaScript (lengths: Go=%d, TinyGo=%d)", len(goJS), len(tinygoJS))
			}
		}
	}
}
