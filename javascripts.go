package tinywasm

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/cdvelop/tinystring"
)

// wasm_execGoSignatures returns signatures expected in Go's wasm_exec.js
func wasm_execGoSignatures() []string {
	return []string{
		"runtime.scheduleTimeoutEvent",
		"runtime.clearTimeoutEvent",
		"runtime.wasmExit",
		// note: removed shared or ambiguous signatures such as syscall/js.valueGet
	}
}

// wasm_execTinyGoSignatures returns signatures expected in TinyGo's wasm_exec.js
func wasm_execTinyGoSignatures() []string {
	return []string{
		"runtime.sleepTicks",
		"runtime.ticks",
		"$runtime.alloc",
		"tinygo_js",
	}
}

// JavascriptForInitializing returns the JavaScript code needed to initialize WASM
func (h *TinyWasm) JavascriptForInitializing() (js string, err error) {
	// Load wasm js code
	wasmType, TinyGoCompiler := h.WasmProjectTinyGoJsUse()
	if !wasmType {
		return
	}

	// Return appropriate cached content if available
	if TinyGoCompiler && h.tinyGoWasmJsCache != "" {
		return h.tinyGoWasmJsCache, nil
	} else if !TinyGoCompiler && h.goWasmJsCache != "" {
		return h.goWasmJsCache, nil
	}

	var wasmExecJsPath string
	if TinyGoCompiler {
		wasmExecJsPath, err = h.GetWasmExecJsPathTinyGo()
	} else {
		wasmExecJsPath, err = h.GetWasmExecJsPathGo()
	}
	if err != nil {
		return "", err
	}

	// Read wasm js code
	wasmJs, err := os.ReadFile(wasmExecJsPath)
	if err != nil {
		return "", err
	}

	stringWasmJs := string(wasmJs)

	// Verify activeBuilder is initialized before accessing it
	if h.activeBuilder == nil {
		return "", Errf("activeBuilder not initialized")
	}

	// add code webassebly here
	stringWasmJs += `
		const go = new Go();
		WebAssembly.instantiateStreaming(fetch("` + h.activeBuilder.MainOutputFileNameWithExtension() + `"), go.importObject).then((result) => {
			go.run(result.instance);
		});
	`

	// Store in appropriate cache
	if TinyGoCompiler {
		h.tinyGoWasmJsCache = stringWasmJs
	} else {
		h.goWasmJsCache = stringWasmJs
	}

	return stringWasmJs, nil
}

// ClearJavaScriptCache clears both cached JavaScript strings to force regeneration
func (h *TinyWasm) ClearJavaScriptCache() {
	h.goWasmJsCache = ""
	h.tinyGoWasmJsCache = ""
}

// GetWasmExecJsPathTinyGo returns the path to TinyGo's wasm_exec.js file
func (w *TinyWasm) GetWasmExecJsPathTinyGo() (string, error) {
	// Method 1: Try standard lib location pattern
	libPaths := []string{
		"/usr/local/lib/tinygo/targets/wasm_exec.js",
		"/opt/tinygo/targets/wasm_exec.js",
	}

	for _, path := range libPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Method 2: Derive from tinygo executable path
	tinygoPath, err := exec.LookPath("tinygo")
	if err != nil {
		return "", Errf("tinygo executable not found: %v", err)
	}

	// Get directory where tinygo is located
	tinyGoDir := filepath.Dir(tinygoPath)

	// Common installation patterns
	patterns := []string{
		// For /usr/local/bin/tinygo -> /usr/local/lib/tinygo/targets/wasm_exec.js
		filepath.Join(filepath.Dir(tinyGoDir), "lib", "tinygo", "targets", "wasm_exec.js"),
		// For /usr/bin/tinygo -> /usr/lib/tinygo/targets/wasm_exec.js
		filepath.Join(filepath.Dir(tinyGoDir), "lib", "tinygo", "targets", "wasm_exec.js"),
		// For portable installation: remove bin and add targets
		filepath.Join(filepath.Dir(tinyGoDir), "targets", "wasm_exec.js"),
	}

	for _, wasmExecPath := range patterns {
		if _, err := os.Stat(wasmExecPath); err == nil {
			return wasmExecPath, nil
		}
	}

	return "", Errf("TinyGo wasm_exec.js not found. Searched paths: %v", append(libPaths, patterns...))
}

// GetWasmExecJsPathGo returns the path to Go's wasm_exec.js file
func (w *TinyWasm) GetWasmExecJsPathGo() (string, error) {
	// Method 1: Try GOROOT environment variable (most reliable)
	goRoot := os.Getenv("GOROOT")
	if goRoot != "" {
		patterns := []string{
			filepath.Join(goRoot, "misc", "wasm", "wasm_exec.js"), // Traditional location
			filepath.Join(goRoot, "lib", "wasm", "wasm_exec.js"),  // Modern location
		}
		for _, wasmExecPath := range patterns {
			if _, err := os.Stat(wasmExecPath); err == nil {
				return wasmExecPath, nil
			}
		}
	}

	// Method 2: Derive from go executable path
	goPath, err := exec.LookPath("go")
	if err != nil {
		return "", Errf("go executable not found: %v", err)
	}

	// Get installation directory (parent of bin directory)
	goDir := filepath.Dir(goPath)

	// Remove bin directory from path (cross-platform)
	if filepath.Base(goDir) == "bin" {
		goDir = filepath.Dir(goDir)
	}

	// Try multiple patterns for different Go versions
	patterns := []string{
		filepath.Join(goDir, "misc", "wasm", "wasm_exec.js"), // Traditional location
		filepath.Join(goDir, "lib", "wasm", "wasm_exec.js"),  // Modern location (Go 1.21+)
	}

	for _, wasmExecPath := range patterns {
		if _, err := os.Stat(wasmExecPath); err == nil {
			return wasmExecPath, nil
		}
	}

	return "", Errf("go wasm_exec.js not found. Searched: GOROOT=%s, patterns=%v", goRoot, patterns)
}
