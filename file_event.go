package tinywasm

import (
	"path"

	. "github.com/cdvelop/tinystring"
)

func (w *TinyWasm) SupportedExtensions() []string {
	return []string{".go"}
}

// NewFileEvent handles file events for WASM compilation with automatic project detection
// fileName: name of the file (e.g., main.wasm.go)
// extension: file extension (e.g., .go)
// filePath: full path to the file (e.g., ./home/userName/ProjectName/web/public/main.wasm.go)
// event: type of file event (e.g., create, remove, write, rename)
func (w *TinyWasm) NewFileEvent(fileName, extension, filePath, event string) error {
	const e = "NewFileEvent Wasm"

	if filePath == "" {
		return Err(e, "filePath is empty")
	}

	w.Logger(extension, event, "...", filePath)

	// Only process Go files for compilation triggers
	if extension != ".go" {
		return nil
	}

	// Only process write/create events
	if event != "write" && event != "create" {
		return nil
	}

	// Check if this file should trigger compilation
	if !w.ShouldCompileToWasm(fileName, filePath) {
		// File should be ignored (backend file or unknown type)
		return nil
	}

	// Compile using current active builder
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
	if fileName == w.Config.MainInputFile {
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
	return path.Join(w.Config.WebFilesRootRelative, w.Config.MainInputFile)
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
