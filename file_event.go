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
// filePath: full path to the file (e.g., web/public/wasm/main.wasm.go, modules/users/wasm/users.wasm.go, modules/auth/f.logout.go)
// event: type of file event (e.g., create, remove, write, rename)
func (w *TinyWasm) NewFileEvent(fileName, extension, filePath, event string) error {
	const e = "NewFileEvent Wasm"

	if filePath == "" {
		return errors.New(e + "filePath is empty")
	}

	// Auto-detect WASM project based on file structure
	w.updateWasmProjectDetection(fileName, filePath)

	fmt.Fprint(w.Log, "Wasm", extension, event, "...", filePath)
	// Check if this file should trigger WASM compilation
	if !w.ShouldCompileToWasm(fileName, filePath) {
		// File should be ignored (backend file or unknown type)
		return nil
	}
	if event != "write" {
		return nil
	}

	// Use gobuild for compilation instead of direct exec.Command
	if w.builder == nil {
		return errors.New("builder not initialized")
	}

	// Update builder configuration in case compiler settings have changed
	w.updateBuilderConfig()

	// Compile using gobuild
	if err := w.builder.CompileProgram(); err != nil {
		return errors.New("compiling to WebAssembly error: " + err.Error())
	}

	return nil
}

// OutputPathMainFileWasm returns the output path for the main WASM file e.g: web/public/wasm/main.wasm
func (w *TinyWasm) OutputPathMainFileWasm() string {
	return w.MainOutputFile()
}

// wasmFilesOutputDirectory returns the directory where WASM files are output e.g: web/public
func (w *TinyWasm) wasmFilesOutputDirectory() string {
	rootFolder, subfolder := w.WebFilesFolder()
	return path.Join(rootFolder, subfolder)
}

// UnobservedFiles returns files that should not be watched for changes e.g: main.wasm
func (w *TinyWasm) UnobservedFiles() []string {
	filename := "main.wasm" // default fallback
	if w.builder != nil {
		filename = w.builder.MainOutputFileNameWithExtension()
	}
	return []string{
		filename, // main.wasm - generated file, should not be watched
		// main.wasm.go should be watched as developers can modify it
	}
}

// updateWasmProjectDetection automatically detects if this is a WASM project based on file structure
func (w *TinyWasm) updateWasmProjectDetection(fileName, filePath string) {
	// Check for main.wasm.go file (strong indicator of WASM project)
	if fileName == w.mainInputFile {
		if !w.wasmProject {
			w.wasmProject = true
			if w.Log != nil {
				fmt.Fprintf(w.Log, "Auto-detected WASM project: found %s\n", fileName)
			}
		}
		return
	}

	// Check for .wasm.go files in modules (another strong indicator)
	if strings.HasSuffix(fileName, ".wasm.go") {
		if !w.wasmProject {
			w.wasmProject = true
			if w.Log != nil {
				fmt.Fprintf(w.Log, "Auto-detected WASM project: found WASM module %s\n", fileName)
			}
		}
		return
	}

	// Check for frontend files in modules directory
	if w.IsFrontendFile(fileName) && (strings.Contains(filePath, "/modules/") || strings.Contains(filePath, "\\modules\\")) {
		if !w.wasmProject {
			w.wasmProject = true
			if w.Log != nil {
				fmt.Fprintf(w.Log, "Auto-detected WASM project: found frontend file %s\n", fileName)
			}
		}
	}
}
