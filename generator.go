package tinywasm

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/cdvelop/mdgo"
)

//go:embed templates/*
var embeddedFS embed.FS

// CreateDefaultWasmFileClientIfNotExist creates a default WASM main.go file from the embedded markdown template
// It never overwrites an existing file and returns the TinyWasm instance for method chaining.
func (t *TinyWasm) CreateDefaultWasmFileClientIfNotExist() *TinyWasm {
	// Build target path from Config
	targetPath := filepath.Join(t.AppRootDir, t.SourceDir, t.MainInputFile)

	// Never overwrite existing files
	if _, err := os.Stat(targetPath); err == nil {
		if t.Logger != nil {
			t.Logger("WASM file already exists at", targetPath, ", skipping generation")
		}
		return t
	}

	// Read embedded markdown (no template processing needed - static content)
	raw, errRead := embeddedFS.ReadFile("templates/basic_wasm_client.md")
	if errRead != nil {
		if t.Logger != nil {
			t.Logger("Error reading embedded template:", errRead)
		}
		return t
	}

	// Use mdgo to extract Go code from markdown
	writer := func(name string, data []byte) error {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}
		return os.WriteFile(name, data, 0o644)
	}

	// mdgo needs the full destination path
	destDir := filepath.Join(t.AppRootDir, t.SourceDir)

	m := mdgo.New(t.AppRootDir, destDir, writer).
		InputByte(raw)

	if t.Logger != nil {
		m.SetLogger(t.Logger)
	}

	// Extract to the main file
	if err := m.Extract(t.MainInputFile); err != nil {
		if t.Logger != nil {
			t.Logger("Error extracting go code from markdown:", err)
		}
		return t
	}

	if t.Logger != nil {
		t.Logger("Generated WASM file at", targetPath)
	}

	t.wasmProject = true

	return t
}
