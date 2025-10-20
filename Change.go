package tinywasm

import (
	"os"
	"path"

	. "github.com/cdvelop/tinystring"
)

func (w *TinyWasm) Shortcuts() []map[string]string {
	return []map[string]string{
		{w.BuildLargeSizeShortcut: Translate(D.Mode, "Large", "stLib").String()},
		{w.BuildMediumSizeShortcut: Translate(D.Mode, "Medium", "tinygo").String()},
		{w.BuildSmallSizeShortcut: Translate(D.Mode, "Small", "tinygo").String()},
	}
}

// Change updates the compiler mode for TinyWasm and reports progress via the provided callback.
// Implements the HandlerEdit interface: Change(newValue string, progress func(msgs ...any))
func (w *TinyWasm) Change(newValue string, progress func(msgs ...any)) {

	// Normalize input: trim spaces and convert to uppercase so users can
	// provide lowercase shortcuts (e.g., "l") without confusing Shortcuts().
	newValue = Convert(newValue).ToUpper().String()

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
	// Ensure mode is uppercase to match configured shortcuts which are
	// expected to be single uppercase letters by default.
	mode = Convert(mode).ToUpper().String()

	validModes := []string{
		Convert(w.Config.BuildLargeSizeShortcut).ToUpper().String(),
		Convert(w.Config.BuildMediumSizeShortcut).ToUpper().String(),
		Convert(w.Config.BuildSmallSizeShortcut).ToUpper().String(),
	}

	for _, valid := range validModes {
		if mode == valid {
			return nil
		}
	}

	return Err(D.Mode, ":", mode, D.Invalid, D.Valid, ":", validModes)
}

// getSuccessMessage returns appropriate success message for mode
func (w *TinyWasm) getSuccessMessage(mode string) string {

	switch mode {
	case w.Config.BuildLargeSizeShortcut:
		return Translate(D.Changed, D.To, D.Mode, "Large").String()
	case w.Config.BuildMediumSizeShortcut:
		return Translate(D.Changed, D.To, D.Mode, "Medium").String()
	case w.Config.BuildSmallSizeShortcut:
		return Translate(D.Changed, D.To, D.Mode, "Small").String()
	default:
		return Translate(D.Mode, ":", mode, D.Invalid).String()
	}

}

func (w *TinyWasm) GetLastOperationID() string   { return w.lastOpID }
func (w *TinyWasm) SetLastOperationID(id string) { w.lastOpID = id }
