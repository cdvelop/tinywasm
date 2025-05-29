package tinywasm

import "os"

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
		wasmExecJsPath, err = h.getWasmExecJsPathTinyGo()
	} else {
		wasmExecJsPath, err = h.getWasmExecJsPathGo()
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

	// Store in appropriate cache
	if TinyGoCompiler {
		h.tinyGoWasmJsCache = stringWasmJs
	} else {
		h.goWasmJsCache = stringWasmJs
	}

	return stringWasmJs, nil
}
