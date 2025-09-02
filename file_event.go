package tinywasm

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// NewFileEvent handles file events for WASM compilation with automatic project detection
// fileName: name of the file (e.g., main.wasm.go)
// extension: file extension (e.g., .go)
// filePath: full path to the file (e.g., web/public/wasm/main.wasm.go)
// event: type of file event (e.g., create, remove, write, rename)
func (w *TinyWasm) NewFileEvent(fileName, extension, filePath, event string) error {
	const e = "NewFileEvent Wasm"

	if filePath == "" {
		return errors.New(e + "filePath is empty")
	}
	// Auto-detect WASM project based on file structure
	w.wasmDetectionFunc(fileName, filePath)

	fmt.Fprint(w.Logger, "Wasm", extension, event, "...", filePath)
	// Check if this file should trigger WASM compilation
	if !w.ShouldCompileToWasm(fileName, filePath) {
		// File should be ignored (backend file or unknown type)
		return nil
	}
	if event != "write" && event != "create" {
		return nil
	}

	// Use gobuild for compilation instead of direct exec.Command
	if w.activeBuilder == nil {
		return errors.New("builder not initialized")
	}

	// Compile using gobuild
	if err := w.activeBuilder.CompileProgram(); err != nil {
		return errors.New("compiling to WebAssembly error: " + err.Error())
	}

	return nil
}

// MainInputFileRelativePath returns the relative path to the main WASM input file (e.g. "main.wasm.go").
func (w *TinyWasm) MainInputFileRelativePath() string {
	// The input lives under the web root (WebFilesRootRelative) by convention.
	// Return full path including AppRootDir for callers that expect absolute paths
	return path.Join(w.Config.WebFilesRootRelative, w.mainInputFile)
}

// UnobservedFiles returns files that should not be watched for changes e.g: main.wasm
func (w *TinyWasm) UnobservedFiles() []string {
	return w.activeBuilder.UnobservedFiles()
}

// updateWasmProjectDetectionActive automatically detects if this is a WASM project and configures VS Code
func (w *TinyWasm) updateWasmProjectDetectionActive(fileName, filePath string) {
	wasmDetected := false

	// Check for main.wasm.go file (strong indicator of WASM project)
	if fileName == w.mainInputFile {
		if !w.wasmProject {
			w.wasmProject = true
			wasmDetected = true
		}
	}

	// Check for .wasm.go files in modules (another strong indicator)
	if strings.HasSuffix(fileName, ".wasm.go") {
		if !w.wasmProject {
			w.wasmProject = true
			wasmDetected = true
		}
	}

	// Note: frontend prefix detection was removed; rely on main.wasm.go and .wasm.go files
	// If WASM project detected, configure VS Code and switch to inactive function
	if wasmDetected {
		w.VisualStudioCodeWasmEnvConfig()
		w.wasmDetectionFunc = w.updateWasmProjectDetectionInactive
	}
}

// updateWasmProjectDetectionInactive is a no-op function used after VS Code is configured
func (w *TinyWasm) updateWasmProjectDetectionInactive(fileName, filePath string) {
	// Do nothing - VS Code already configured
}
