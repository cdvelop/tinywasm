package tinywasm

import (
	. "github.com/cdvelop/tinystring"
	"path"
)

// NewFileEvent handles file events for WASM compilation with automatic project detection
// fileName: name of the file (e.g., main.wasm.go)
// extension: file extension (e.g., .go)
// filePath: full path to the file (e.g., web/public/wasm/main.wasm.go)
// event: type of file event (e.g., create, remove, write, rename)
func (w *TinyWasm) NewFileEvent(fileName, extension, filePath, event string) error {
	const e = "NewFileEvent Wasm"

	if filePath == "" {
		return Err(e, "filePath is empty")
	}

	// Auto-detect WASM project base on js file eg: pwa/public/main.js
	w.wasmDetectionFuncFromJsFile(fileName, extension, filePath, event)

	// Auto-detect WASM project based on file structure
	w.wasmDetectionFuncFromGoFile(fileName, filePath)

	w.Logger(extension, event, "...", filePath)
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
		return Err("builder not initialized")
	}

	// Compile using gobuild
	if err := w.activeBuilder.CompileProgram(); err != nil {
		return Err("compiling to WebAssembly error: ", err)
	}

	return nil
}

// ShouldCompileToWasm determines if a file should trigger WASM compilation
func (w *TinyWasm) ShouldCompileToWasm(fileName, filePath string) bool {
	// Always compile main.wasm.go
	if fileName == w.mainInputFile {
		return true
	}

	// Any .wasm.go file should trigger compilation
	if HasSuffix(fileName, ".wasm.go") {
		return true
	}

	// All other files should be ignored
	return false
}

// MainInputFileRelativePath returns the relative path to the main WASM input file (e.g. "main.wasm.go").
func (w *TinyWasm) MainInputFileRelativePath() string {
	// The input lives under the web root (WebFilesRootRelative) by convention.
	// Return full path including AppRootDir for callers that expect absolute paths
	return path.Join(w.Config.WebFilesRootRelative, w.mainInputFile)
}

// MainOutputFileAbsolutePath returns the absolute path to the main WASM output file (e.g. "main.wasm").
func (w *TinyWasm) MainOutputFileAbsolutePath() string {
	// The output file is created in OutFolderRelativePath which is:
	// AppRootDir/WebFilesRootRelative/WebFilesSubRelative/main.wasm
	return path.Join(w.Config.AppRootDir, w.Config.WebFilesRootRelative, w.Config.WebFilesSubRelative, "main.wasm")
}

// UnobservedFiles returns files that should not be watched for changes e.g: main.wasm
func (w *TinyWasm) UnobservedFiles() []string {
	return w.activeBuilder.UnobservedFiles()
}

// wasmDetectionFuncFromGoFileActive automatically detects if this is a WASM project and configures VS Code
func (w *TinyWasm) wasmDetectionFuncFromGoFileActive(fileName, filePath string) {
	wasmDetected := false

	// Check for main.wasm.go file (strong indicator of WASM project)
	if fileName == w.mainInputFile {
		if !w.wasmProject {
			w.wasmProject = true
			wasmDetected = true
		}
	}

	// Check for .wasm.go files in modules (another strong indicator)
	if HasSuffix(fileName, ".wasm.go") {
		if !w.wasmProject {
			w.wasmProject = true
			wasmDetected = true
		}
	}

	// If WASM project detected, configure VS Code and switch to inactive function
	if wasmDetected {
		w.VisualStudioCodeWasmEnvConfig()
		w.wasmDetectionFuncFromGoFile = w.wasmDetectionFuncFromGoFileInactive
	}
}

// wasmDetectionFuncFromGoFileInactive is a no-op function used after VS Code is configured
func (w *TinyWasm) wasmDetectionFuncFromGoFileInactive(fileName, filePath string) {
	// Do nothing - VS Code already configured
}
