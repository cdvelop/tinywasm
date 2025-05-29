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
	FrontendPrefix []string  // Prefixes used to identify frontend files (e.g., "f.", "front.")
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

// IsFrontendFile checks if a file should trigger WASM compilation based on frontend prefixes
func (w *TinyWasm) IsFrontendFile(filename string) bool {
	if len(filename) < 3 {
		return false
	}

	// Check frontend prefixes
	for _, prefix := range w.FrontendPrefix {
		if strings.HasPrefix(filename, prefix) {
			return true
		}
	}

	return false
}

// ShouldCompileToWasm determines if a file should trigger WASM compilation
func (w *TinyWasm) ShouldCompileToWasm(fileName, filePath string) bool {
	// Always compile main.wasm.go
	if fileName == w.mainInputFile {
		return true
	}

	// Always compile .wasm.go files in modules
	if strings.HasSuffix(fileName, ".wasm.go") {
		return true
	}

	// Check if it's a frontend file by prefix (only if configured)
	if len(w.FrontendPrefix) > 0 {
		for _, prefix := range w.FrontendPrefix {
			if strings.HasPrefix(fileName, prefix) {
				return true
			}
		}
	}

	// Go files in modules: check for unknown prefixes with dot
	if strings.HasSuffix(fileName, ".go") && (strings.Contains(filePath, "/modules/") || strings.Contains(filePath, "\\modules\\")) {
		// If file has a prefix with dot (prefix.name.go) and it's not in our known frontend prefixes,
		// assume it's a backend file and don't compile
		if strings.Contains(fileName, ".") {
			// Extract potential prefix (everything before first dot + dot)
			parts := strings.Split(fileName, ".")
			if len(parts) >= 3 { // prefix.name.go = 3 parts minimum
				potentialPrefix := parts[0] + "."

				// Check if this prefix is in our known frontend prefixes
				isKnownFrontend := false
				for _, prefix := range w.FrontendPrefix {
					if potentialPrefix == prefix {
						isKnownFrontend = true
						break
					}
				}

				// If it has a prefix with dot but it's not a known frontend prefix, don't compile
				if !isKnownFrontend {
					return false
				}
			}
		}

		// Regular Go files in modules without prefixes should compile
		return true
	}

	// All other files should be ignored
	return false
}
