package tinywasm

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
)

// NewFileEvent handles file events for WASM compilation
// fileName: name of the file (e.g., main.wasm.go)
// extension: file extension (e.g., .go)
// filePath: full path to the file (e.g., web/public/wasm/main.wasm.go, modules/users/wasm/users.wasm.go, modules/auth/f.logout.go)
// event: type of file event (e.g., create, remove, write, rename)
func (h *TinyWasm) NewFileEvent(fileName, extension, filePath, event string) error {
	const e = "NewFileEvent Wasm"

	if filePath == "" {
		return errors.New(e + "filePath is empty")
	}

	fmt.Fprint(h.Log, "Wasm", extension, event, "...", filePath)

	var inputFilePath, outputFilePath string
	// Check if the file is an independent wasm module
	if fileName != h.mainInputFile {

		moduleName, err := GetModuleName(filePath)
		if err != nil {
			return err
		}
		fmt.Fprint(h.Log, "Module Name: "+moduleName)

		// Create main module wasm path eg: modules/users/wasm/users.wasm.go
		inputFilePath = path.Join(h.ModulesFolder, moduleName, "wasm", moduleName+".wasm.go")
		outputFilePath = path.Join(h.wasmFilesOutputDirectory(), moduleName+".wasm")

	} else {
		// The file is the main wasm file main.wasm.go
		inputFilePath = path.Join(h.wasmFilesOutputDirectory(), h.mainInputFile)
		outputFilePath = h.OutputPathMainFileWasm()
	}
	// Check if the ...wasm/moduleName.wasm.go file exists
	if _, err := os.Stat(inputFilePath); err != nil {
		// WebAssembly file not found
	}

	if event != "write" {
		return nil
	}

	var cmd *exec.Cmd
	var flags string

	// Adjust compilation parameters according to configuration
	if h.TinyGoCompiler() {
		cmd = exec.Command("tinygo", "build", "-o", outputFilePath, "-target", "wasm", "--no-debug", "-ldflags", flags, inputFilePath)
	} else {
		cmd = exec.Command("go", "build", "-o", outputFilePath, "-tags", "dev", "-ldflags", flags, "-v", inputFilePath)
		cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	}

	output, er := cmd.CombinedOutput()
	if er != nil {
		return errors.New("compiling to WebAssembly error: " + er.Error() + " string(output):" + string(output))
	}

	// Check if the wasm file was created correctly
	if _, er := os.Stat(outputFilePath); er != nil {
		return errors.New("wasm file was not created: " + er.Error())
	}

	return nil
}

// OutputPathMainFileWasm returns the output path for the main WASM file e.g: web/public/wasm/main.wasm
func (w *TinyWasm) OutputPathMainFileWasm() string {
	return path.Join(w.wasmFilesOutputDirectory(), w.mainOutputFile)
}

// wasmFilesOutputDirectory returns the directory where WASM files are output e.g: web/public/wasm
func (w *TinyWasm) wasmFilesOutputDirectory() string {
	rootFolder, subfolder := w.WebFilesFolder()
	return path.Join(rootFolder, subfolder, "wasm")
}

// UnobservedFiles returns files that should not be watched for changes e.g: main.wasm
func (w *TinyWasm) UnobservedFiles() []string {
	return []string{
		w.mainOutputFile,
		// add wasm name modules here
	}
}
