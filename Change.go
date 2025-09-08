package tinywasm

import (
	"os"
	"path"

	. "github.com/cdvelop/tinystring"
)

// Change updates the compiler mode for TinyWasm and reports progress via the provided callback.
// Implements the HandlerEdit interface: Change(newValue string, progress func(msgs ...any))
func (w *TinyWasm) Change(newValue string, progress func(msgs ...any)) {

	// Validate mode
	if err := w.validateMode(newValue); err != nil {
		progress(err)
		return
	}

	// Check TinyGo installation for debug/production modes
	if w.requiresTinyGo(newValue) && !w.tinyGoInstalled {
		// handleTinyGoMissing returns an error with descriptive message
		progress(w.handleTinyGoMissing().Error())
		return
	}

	// Update active builder
	w.updateCurrentBuilder(newValue)

	// Ensure wasm_exec.js is available before compilation. The method will
	// internally verify whether this is a WASM project and perform the write.
	w.wasmProjectWriteOrReplaceWasmExecJsOutput()

	// Check if main WASM file exists before attempting compilation
	rootFolder := path.Join(w.AppRootDir, w.Config.WebFilesRootRelative)
	mainWasmPath := path.Join(rootFolder, w.mainInputFile)
	if _, err := os.Stat(mainWasmPath); err != nil {
		// File doesn't exist, just report success message without compilation
		progress(w.getSuccessMessage(newValue))
		return
	}

	// Auto-recompile with appropriate message format for MessageType detection
	if err := w.recompileMainWasm(); err != nil {
		// Report warning message via progress (don't treat as fatal)
		warningMsg := Translate("Warning:", "auto", "compilation", "failed:", err).String()
		if warningMsg == "" {
			warningMsg = "Warning: auto compilation failed: " + err.Error()
		}
		progress(warningMsg)
		return
	}

	// Report success
	progress(w.getSuccessMessage(newValue))
}

// recompileMainWasm recompiles the main WASM file if it exists
func (w *TinyWasm) recompileMainWasm() error {
	if w.activeBuilder == nil {
		return Err("builder not initialized")
	}
	rootFolder := path.Join(w.AppRootDir, w.Config.WebFilesRootRelative)
	mainWasmPath := path.Join(rootFolder, w.mainInputFile)

	// Check if main.wasm.go exists
	if _, err := os.Stat(mainWasmPath); err != nil {
		return Err("main WASM file not found:", mainWasmPath)
	}

	// Use gobuild to compile
	return w.activeBuilder.CompileProgram()
}

// validateMode validates if the provided mode is supported
func (w *TinyWasm) validateMode(mode string) error {
	validModes := []string{
		w.Config.CodingShortcut,     // "c"
		w.Config.DebuggingShortcut,  // "d"
		w.Config.ProductionShortcut, // "p"
	}

	for _, valid := range validModes {
		if mode == valid {
			return nil
		}
	}

	return Err(D.Invalid, "mode", mode, "valid modes:", validModes)
}

// getSuccessMessage returns appropriate success message for mode
func (w *TinyWasm) getSuccessMessage(mode string) string {
	var msg string
	switch mode {
	case w.Config.CodingShortcut:
		msg = Translate("Switching", "to", "coding", "mode").String()
	case w.Config.DebuggingShortcut:
		msg = Translate("Switching", "to", "debugging", "mode").String()
	case w.Config.ProductionShortcut:
		msg = Translate("Switching", "to", "production", "mode").String()
	default:
		msg = Translate(D.Invalid, "mode").String()
	}

	// Fallback if Translate returns empty string
	if msg == "" {
		switch mode {
		case w.Config.CodingShortcut:
			msg = "Switching to coding mode"
		case w.Config.DebuggingShortcut:
			msg = "Switching to debugging mode"
		case w.Config.ProductionShortcut:
			msg = "Switching to production mode"
		default:
			msg = "Invalid mode"
		}
	}

	return msg
}
