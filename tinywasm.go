package tinywasm

import (
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

// TinyWasm provides WebAssembly compilation capabilities using TinyGo
type TinyWasm struct {
	*WasmConfig
	ModulesFolder  string // default "modules". for test change eg: "test/modules"
	mainInputFile  string // eg: main.wasm.go
	mainOutputFile string // eg: main.wasm

	goWasmJsCache     string
	tinyGoWasmJsCache string
}

// WasmConfig holds configuration for WASM compilation
type WasmConfig struct {
	// WebFilesFolder returns root web folder and subfolder eg: "web","public"
	WebFilesFolder func() (string, string)
	Log            io.Writer // For logging output to external systems (e.g., TUI, console)
}

// New creates a new TinyWasm instance with the provided configuration
func New(c *WasmConfig) *TinyWasm {
	w := &TinyWasm{
		WasmConfig:     c,
		ModulesFolder:  "modules",
		mainInputFile:  "main.wasm.go",
		mainOutputFile: "main.wasm",
	}

	return w
}

// WasmProjectTinyGoJsUse returns whether TinyGo JS should be used
func (w *TinyWasm) WasmProjectTinyGoJsUse() (bool, bool) {
	return true, true // Always use TinyGo for this package
}

// TinyGoCompiler returns if TinyGo compiler should be used (always true for this package)
func (w *TinyWasm) TinyGoCompiler() bool {
	return true // Always use TinyGo by default in the tinywasm package
}

// getWasmExecJsPathTinyGo returns the path to TinyGo's wasm_exec.js file
func (w *TinyWasm) getWasmExecJsPathTinyGo() (string, error) {
	path, err := exec.LookPath("tinygo")
	if err != nil {
		return "", err
	}
	// Get installation directory
	tinyGoDir := filepath.Dir(path)
	// Clean path and remove "\bin"
	tinyGoDir = strings.TrimSuffix(tinyGoDir, "\\bin")
	// Build complete path to wasm_exec.js file
	return filepath.Join(tinyGoDir, "targets", "wasm_exec.js"), nil
}

// getWasmExecJsPathGo returns the path to Go's wasm_exec.js file
func (w *TinyWasm) getWasmExecJsPathGo() (string, error) {
	// Get Go installation directory path from GOROOT environment variable
	path, er := exec.LookPath("go")
	if er != nil {
		return "", er
	}
	// Get installation directory
	GoDir := filepath.Dir(path)
	// Clean path and remove "\bin"
	GoDir = strings.TrimSuffix(GoDir, "\\bin")
	// Build complete path to wasm_exec.js file
	return filepath.Join(GoDir, "misc", "wasm", "wasm_exec.js"), nil
}
