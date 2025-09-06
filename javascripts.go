package tinywasm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func (h *TinyWasm) wasmDetectionFuncFromJsFileActive(fileName, extension, filePath, event string) {
	// Only care about create events for .js files
	if extension != ".js" || event != "create" {
		return
	}

	// Only analyze files under the configured web subfolder
	webSub := filepath.Join(h.AppRootDir, h.Config.WebFilesRootRelative, h.Config.WebFilesSubRelative)
	// Clean paths for reliable prefix checks
	cleanPath := filepath.Clean(filePath)
	cleanWeb := filepath.Clean(webSub)
	if !strings.HasPrefix(cleanPath, cleanWeb) {
		return
	}

	// Read file content (best-effort)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		h.Logger("wasm JS detection read error:", err)
		return
	}
	content := string(data)

	// Count signatures
	goCount := 0
	for _, s := range wasm_execGoSignatures() {
		if strings.Contains(content, s) {
			goCount++
		}
	}
	tinyCount := 0
	for _, s := range wasm_execTinyGoSignatures() {
		if strings.Contains(content, s) {
			tinyCount++
		}
	}

	detected := "none"
	if tinyCount > goCount && tinyCount > 0 {
		// TinyGo detected
		h.tinyGoCompiler = true
		h.wasmProject = true
		detected = "tinygo"
	} else if goCount > tinyCount && goCount > 0 {
		// Go detected
		h.tinyGoCompiler = false
		h.wasmProject = true
		detected = "go"
	} else if tinyCount > 0 && goCount == 0 {
		h.tinyGoCompiler = true
		h.wasmProject = true
		detected = "tinygo"
	} else if goCount > 0 && tinyCount == 0 {
		h.tinyGoCompiler = false
		h.wasmProject = true
		detected = "go"
	} else {
		// ambiguous or no detection
		h.Logger(fmt.Sprintf("DEBUG: JS detection ambiguous or no signatures: go=%d tiny=%d", goCount, tinyCount))
		return
	}

	h.Logger(fmt.Sprintf("DEBUG: JS detection: %s (goCount=%d tinyCount=%d)", detected, goCount, tinyCount))

	// Clear caches so the correct wasm_exec.js will be reloaded
	h.ClearJavaScriptCache()

	// Deactivate further detection handlers
	h.wasmDetectionFuncFromGoFile = h.wasmDetectionFuncFromGoFileInactive
	h.wasmDetectionFuncFromJsFile = h.wasmDetectionFuncFromJsFileInactive
}

func (h *TinyWasm) wasmDetectionFuncFromJsFileInactive(fileName, extension, filePath, event string) {

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
		return "", fmt.Errorf("activeBuilder not initialized")
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
