package tinywasm

import (
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cdvelop/gobuild"
)

// builderWasmInit configures 3 builders for WASM compilation modes
func (w *TinyWasm) builderWasmInit() {
	sourceDir := path.Join(w.AppRootDir, w.Config.SourceDir)
	outputDir := path.Join(w.AppRootDir, w.Config.OutputDir)
	mainInputFileRelativePath := path.Join(sourceDir, w.Config.MainInputFile)

	// Base configuration shared by all builders
	baseConfig := gobuild.Config{
		MainInputFileRelativePath: mainInputFileRelativePath,
		OutName:                   w.Config.OutputName, // Output will be {OutputName}.wasm
		Extension:                 ".wasm",
		OutFolderRelativePath:     outputDir,
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
	case w.Config.BuildFastShortcut: // "f"
		w.activeBuilder = w.builderCoding
	case w.Config.BuildBugShortcut: // "b"
		w.activeBuilder = w.builderDebug
	case w.Config.BuildMinimalShortcut: // "m"
		w.activeBuilder = w.builderProduction
	default:
		w.activeBuilder = w.builderCoding // fallback to coding mode
	}

	// 3. Update current mode tracking
	w.currentMode = mode
}

// OutputRelativePath returns the RELATIVE path to the final output file
// eg: "deploy/edgeworker/app.wasm" (relative to AppRootDir)
// This is used by file watchers to identify output files that should be ignored.
// The returned path always uses forward slashes (/) for consistency across platforms.
func (w *TinyWasm) OutputRelativePath() string {
	// FinalOutputPath() returns absolute path like: /tmp/test/deploy/edgeworker/app.wasm
	// We need to extract the relative portion: deploy/edgeworker/app.wasm
	fullPath := w.activeBuilder.FinalOutputPath()

	// Remove AppRootDir prefix to get relative path
	if strings.HasPrefix(fullPath, w.Config.AppRootDir) {
		relPath := strings.TrimPrefix(fullPath, w.Config.AppRootDir)
		// Remove leading separator (/ or \)
		relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
		relPath = strings.TrimPrefix(relPath, "/")  // Handle Unix paths
		relPath = strings.TrimPrefix(relPath, "\\") // Handle Windows paths
		// Normalize to forward slashes for consistency (replace all backslashes)
		return strings.ReplaceAll(relPath, "\\", "/")
	}

	// Fallback: construct from config values (which are already relative)
	// Normalize to forward slashes for consistency
	result := filepath.Join(w.Config.OutputDir, w.Config.OutputName+".wasm")
	return strings.ReplaceAll(result, "\\", "/")
}
