package tinywasm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateDefaultWasmFileCreatesFile(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := "src/cmd/webclient"
	fullSourcePath := filepath.Join(tmp, sourceDir)

	cfg := NewConfig()
	cfg.AppRootDir = tmp
	cfg.SourceDir = sourceDir
	cfg.MainInputFile = "main.go"
	cfg.Logger = func(messages ...any) {
		t.Log(messages...)
	}

	tw := &TinyWasm{
		Config:      cfg,
		wasmProject: false,
	}

	// Ensure no existing file
	target := filepath.Join(fullSourcePath, cfg.MainInputFile)
	if _, err := os.Stat(target); err == nil {
		t.Fatalf("expected no existing file at %s", target)
	}

	if err := tw.createDefaultWasmFile(); err != nil {
		t.Fatalf("createDefaultWasmFile failed: %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	contentStr := string(content)

	// Verify basic content
	if !strings.Contains(contentStr, "package main") {
		t.Errorf("generated file missing package main")
	}
	if !strings.Contains(contentStr, "syscall/js") {
		t.Errorf("generated file missing syscall/js import")
	}
	if !strings.Contains(contentStr, "Hello from WebAssembly!") {
		t.Errorf("generated file missing expected message")
	}
	if !strings.Contains(contentStr, `createElement`) {
		t.Errorf("generated file missing createElement call")
	}
	if !strings.Contains(contentStr, `select {}`) {
		t.Errorf("generated file missing select statement")
	}
}

func TestCreateDefaultWasmFileDoesNotOverwrite(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := "src/cmd/webclient"
	fullSourcePath := filepath.Join(tmp, sourceDir)
	if err := os.MkdirAll(fullSourcePath, 0755); err != nil {
		t.Fatalf("creating source dir: %v", err)
	}

	cfg := NewConfig()
	cfg.AppRootDir = tmp
	cfg.SourceDir = sourceDir
	cfg.MainInputFile = "main.go"

	tw := &TinyWasm{
		Config:      cfg,
		wasmProject: false,
	}

	target := filepath.Join(fullSourcePath, cfg.MainInputFile)

	// Create existing file with different content
	original := "// ORIGINAL CONTENT DO NOT OVERWRITE"
	if err := os.WriteFile(target, []byte(original), 0644); err != nil {
		t.Fatalf("writing original file: %v", err)
	}

	// Try to generate (should skip)
	if err := tw.createDefaultWasmFile(); err != nil {
		t.Fatalf("createDefaultWasmFile failed: %v", err)
	}

	// Verify original content is preserved
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading file after generate: %v", err)
	}

	if string(content) != original {
		t.Fatalf("file was overwritten, expected original content")
	}
}
