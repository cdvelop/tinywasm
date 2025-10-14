package tinywasm

import (
	"os"
	"path"

	. "github.com/cdvelop/tinystring"
)

func (w *TinyWasm) Shortcuts() map[string]string {
	return map[string]string{
		w.BuildFastShortcut:    Translate(D.Mode, "Fast", "stLib").String(),
		w.BuildBugShortcut:     Translate(D.Mode, "Debugging", "tinygo").String(),
		w.BuildMinimalShortcut: Translate(D.Mode, "Minimal", "tinygo").String(),
	}
}

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

	// Check if main WASM file exists before attempting compilation
	sourceDir := path.Join(w.AppRootDir, w.Config.SourceDir)
	mainWasmPath := path.Join(sourceDir, w.Config.MainInputFile)
	if _, err := os.Stat(mainWasmPath); err != nil {
		// File doesn't exist, just report success message without compilation
		progress(w.getSuccessMessage(newValue))
		return
	}

	// Auto-recompile with appropriate message format for MessageType detection
	if err := w.RecompileMainWasm(); err != nil {
		// Report warning message via progress (don't treat as fatal)
		warningMsg := Translate("Warning:", "auto", "compilation", "failed:", err).String()
		if warningMsg == "" {
			warningMsg = "Warning: auto compilation failed: " + err.Error()
		}
		progress(warningMsg)
		return
	}

	// Ensure wasm_exec.js is available before compilation. The method will
	// internally verify whether this is a WASM project and perform the write.
	w.wasmProjectWriteOrReplaceWasmExecJsOutput()

	// Report success
	progress(w.getSuccessMessage(newValue))
}

// RecompileMainWasm recompiles the main WASM file if it exists
func (w *TinyWasm) RecompileMainWasm() error {
	if w.activeBuilder == nil {
		return Err("builder not initialized")
	}
	sourceDir := path.Join(w.AppRootDir, w.Config.SourceDir)
	mainWasmPath := path.Join(sourceDir, w.Config.MainInputFile)

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
		w.Config.BuildFastShortcut,    // "c"
		w.Config.BuildBugShortcut,     // "d"
		w.Config.BuildMinimalShortcut, // "p"
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
	case w.Config.BuildFastShortcut:
		msg = Translate("Switching", "to", "coding", "mode").String()
	case w.Config.BuildBugShortcut:
		msg = Translate("Switching", "to", "debugging", "mode").String()
	case w.Config.BuildMinimalShortcut:
		msg = Translate("Switching", "to", "production", "mode").String()
	default:
		msg = Translate(D.Invalid, "mode").String()
	}

	// Fallback if Translate returns empty string
	if msg == "" {
		switch mode {
		case w.Config.BuildFastShortcut:
			msg = "Switching to coding mode"
		case w.Config.BuildBugShortcut:
			msg = "Switching to debugging mode"
		case w.Config.BuildMinimalShortcut:
			msg = "Switching to production mode"
		default:
			msg = "Invalid mode"
		}
	}

	return msg
}
