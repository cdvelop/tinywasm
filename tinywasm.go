package tinywasm

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

// TinyWasm provides WebAssembly compilation capabilities with dynamic compiler selection
type TinyWasm struct {
	*WasmConfig
	ModulesFolder  string // default "modules". for test change eg: "test/modules"
	mainInputFile  string // eg: main.wasm.go
	mainOutputFile string // eg: main.wasm

	// Dynamic compiler selection fields
	tinyGoCompiler  bool // Use TinyGo compiler when true, Go standard when false
	wasmProject     bool // Automatically detected based on file structure
	tinyGoInstalled bool // Cached TinyGo installation status

	goWasmJsCache     string
	tinyGoWasmJsCache string
}

// WasmConfig holds configuration for WASM compilation
type WasmConfig struct {
	// WebFilesFolder returns root web folder and subfolder eg: "web","public"
	WebFilesFolder func() (string, string)
	Log            io.Writer // For logging output to external systems (e.g., TUI, console)
	FrontendPrefix []string  // Prefixes used to identify frontend files (e.g., "f.", "front.")
	TinyGoCompiler bool      // Enable TinyGo compiler (default: false for faster development)
}

// New creates a new TinyWasm instance with the provided configuration
func New(c *WasmConfig) *TinyWasm {
	w := &TinyWasm{
		WasmConfig:     c,
		ModulesFolder:  "modules",
		mainInputFile:  "main.wasm.go",
		mainOutputFile: "main.wasm",

		// Initialize dynamic fields
		tinyGoCompiler:  c.TinyGoCompiler, // Use config preference
		wasmProject:     false,            // Auto-detected later
		tinyGoInstalled: false,            // Verified on first use
	}

	// Check TinyGo installation status
	w.verifyTinyGoInstallationStatus()

	return w
}

// WasmProjectTinyGoJsUse returns dynamic state based on current configuration
func (w *TinyWasm) WasmProjectTinyGoJsUse() (bool, bool) {
	return w.wasmProject, w.tinyGoCompiler
}

// TinyGoCompiler returns if TinyGo compiler should be used (dynamic based on configuration)
func (w *TinyWasm) TinyGoCompiler() bool {
	return w.tinyGoCompiler && w.tinyGoInstalled
}

// SetTinyGoCompiler validates and sets the TinyGo compiler preference
func (w *TinyWasm) SetTinyGoCompiler(newValue any) (string, error) {
	boolValue, ok := newValue.(bool)
	if !ok {
		err := fmt.Errorf("TinyGoCompiler expects boolean value, got %T", newValue)
		if w.Log != nil {
			fmt.Fprintf(w.Log, "Error: %v\n", err)
		}
		return "", err
	}

	// If trying to enable TinyGo, verify it's installed
	if boolValue && !w.tinyGoInstalled {
		if err := w.VerifyTinyGoInstallation(); err != nil {
			errMsg := fmt.Sprintf("Cannot enable TinyGo compiler: %v", err)
			if w.Log != nil {
				fmt.Fprintf(w.Log, "Error: %s\n", errMsg)
			}
			return errMsg, errors.New(errMsg)
		}
		w.tinyGoInstalled = true
	}

	w.tinyGoCompiler = boolValue

	status := "disabled"
	if boolValue {
		status = "enabled"
	}

	msg := fmt.Sprintf("TinyGo compiler %s", status)
	if w.Log != nil {
		fmt.Fprintf(w.Log, "Info: %s\n", msg)
	}

	return msg, nil
}

// verifyTinyGoInstallationStatus checks and caches TinyGo installation status
func (w *TinyWasm) verifyTinyGoInstallationStatus() {
	if err := w.VerifyTinyGoInstallation(); err != nil {
		w.tinyGoInstalled = false
		if w.Log != nil {
			fmt.Fprintf(w.Log, "Warning: TinyGo not available: %v\n", err)
		}
	} else {
		w.tinyGoInstalled = true

		// If TinyGo is installed, check its version
		version, err := w.GetTinyGoVersion()
		if err != nil {
			w.tinyGoInstalled = false
			if w.Log != nil {
				fmt.Fprintf(w.Log, "Warning: TinyGo version check failed: %v\n", err)
			}
		} else {
			w.tinyGoInstalled = true
			if w.Log != nil {
				fmt.Fprintf(w.Log, "Info: TinyGo installation verified %v  \n", version)
			}
		}
	}
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
