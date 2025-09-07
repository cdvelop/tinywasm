package tinywasm

import (
	"path"
	"time"

	"github.com/cdvelop/gobuild"
	. "github.com/cdvelop/tinystring"
)

// initializeBuilder configures 3 builders for WASM compilation modes
func (w *TinyWasm) initializeBuilder() {
	rootFolder := path.Join(w.AppRootDir, w.Config.WebFilesRootRelative)
	subFolder := w.Config.WebFilesSubRelative
	mainInputFileRelativePath := path.Join(rootFolder, w.mainInputFile)
	outFolder := path.Join(rootFolder, subFolder)

	// Base configuration shared by all builders
	baseConfig := gobuild.Config{
		MainInputFileRelativePath: mainInputFileRelativePath,
		OutName:                   "main", // Output will be main.wasm
		Extension:                 ".wasm",
		OutFolderRelativePath:     outFolder,
		Logger:                    w.Logger,
		Timeout:                   60 * time.Second, // 1 minute for all modes
		Callback:                  w.Callback,
	}

	// Configure Coding builder (Go standard)
	codingConfig := baseConfig
	codingConfig.Command = "go"
	codingConfig.Env = []string{"GOOS=js", "GOARCH=wasm"}
	codingConfig.CompilingArguments = func() []string {
		args := []string{"-tags", "dev"}
		if w.CompilingArguments != nil {
			args = append(args, w.CompilingArguments()...)
		}
		return args
	}
	w.builderCoding = gobuild.New(&codingConfig)

	// Configure Debug builder (TinyGo debug-friendly)
	debugConfig := baseConfig
	debugConfig.Command = "tinygo"
	debugConfig.CompilingArguments = func() []string {
		args := []string{"-target", "wasm", "-opt=1"} // Keep debug symbols
		if w.CompilingArguments != nil {
			args = append(args, w.CompilingArguments()...)
		}
		return args
	}
	w.builderDebug = gobuild.New(&debugConfig)

	// Configure Production builder (TinyGo optimized)
	prodConfig := baseConfig
	prodConfig.Command = "tinygo"
	prodConfig.CompilingArguments = func() []string {
		args := []string{"-target", "wasm", "-opt=z", "-no-debug", "-panic=trap"}
		if w.CompilingArguments != nil {
			args = append(args, w.CompilingArguments()...)
		}
		return args
	}
	w.builderProduction = gobuild.New(&prodConfig)

	// Set initial mode and active builder (default to coding mode)
	w.activeBuilder = w.builderCoding // Default: fast development
}

// updateCurrentBuilder sets the activeBuilder based on mode and cancels ongoing operations
func (w *TinyWasm) updateCurrentBuilder(mode string) {
	// 1. Cancel any ongoing compilation
	if w.activeBuilder != nil {
		w.activeBuilder.Cancel()
	}

	// 2. Set activeBuilder based on mode
	switch mode {
	case w.Config.CodingShortcut: // "c"
		w.activeBuilder = w.builderCoding
	case w.Config.DebuggingShortcut: // "d"
		w.activeBuilder = w.builderDebug
	case w.Config.ProductionShortcut: // "p"
		w.activeBuilder = w.builderProduction
	default:
		w.activeBuilder = w.builderCoding // fallback to coding mode
	}

	// 3. Update current mode tracking
	w.currentMode = mode
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
