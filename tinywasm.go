package tinywasm

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cdvelop/gobuild"
)

// TinyWasm provides WebAssembly compilation capabilities with dynamic compiler selection
type TinyWasm struct {
	*Config
	ModulesFolder string // default "modules". for test change eg: "test/modules"
	mainInputFile string // eg: main.wasm.go

	// Dynamic compiler selection fields
	tinyGoCompiler  bool // Use TinyGo compiler when true, Go standard when false
	wasmProject     bool // Automatically detected based on file structure
	tinyGoInstalled bool // Cached TinyGo installation status

	goWasmJsCache     string
	tinyGoWasmJsCache string

	// gobuild integration - dual builder architecture
	builderTinyGo *gobuild.GoBuild // TinyGo builder for production/optimized builds
	builderGo     *gobuild.GoBuild // Go standard builder for development/fast builds
	builder       *gobuild.GoBuild // Current active builder (points to one of the above)
}

// Config holds configuration for WASM compilation
type Config struct {
	// WebFilesFolder returns root web folder and subfolder eg: "web","public"
	WebFilesFolder func() (string, string)
	Log            io.Writer // For logging output to external systems (e.g., TUI, console)
	FrontendPrefix []string  // Prefixes used to identify frontend files (e.g., "f.", "front.")
	TinyGoCompiler bool      // Enable TinyGo compiler (default: false for faster development)

	// gobuild integration fields
	Callback           func(error)     // Optional callback for async compilation
	CompilingArguments func() []string // Build arguments for compilation (e.g., ldflags)
}

// New creates a new TinyWasm instance with the provided configuration
// Timeout is set to 40 seconds maximum as TinyGo compilation can be slow
// Default values: ModulesFolder="modules", mainInputFile="main.wasm.go"
func New(c *Config) *TinyWasm {
	w := &TinyWasm{
		Config:        c,
		ModulesFolder: "modules",
		mainInputFile: "main.wasm.go",

		// Initialize dynamic fields
		tinyGoCompiler:  c.TinyGoCompiler, // Use config preference
		wasmProject:     false,            // Auto-detected later
		tinyGoInstalled: false,            // Verified on first use
	}

	// Check TinyGo installation status
	w.verifyTinyGoInstallationStatus()

	// Initialize gobuild instance with WASM-specific configuration
	w.initializeBuilder()

	return w
}

// WasmProjectTinyGoJsUse returns dynamic state based on current configuration
func (w *TinyWasm) WasmProjectTinyGoJsUse() (bool, bool) {
	return w.wasmProject, w.tinyGoCompiler
}

// TinyGoCompiler returns if TinyGo compiler should be used (dynamic based on configuration)
func (w *TinyWasm) TinyGoCompiler() bool {
	return w.tinyGoCompiler && w.tinyGoInstalled
}

// initializeBuilder configures both TinyGo and Go builders for WASM compilation
func (w *TinyWasm) initializeBuilder() {
	rootFolder, subFolder := w.WebFilesFolder()
	mainFilePath := path.Join(rootFolder, w.mainInputFile)
	outFolder := path.Join(rootFolder, subFolder)

	// Base configuration shared by both builders
	baseConfig := gobuild.Config{
		MainFilePath: mainFilePath,
		OutName:      "main", // Output will be main.wasm
		Extension:    ".wasm",
		OutFolder:    outFolder,
		Log:          w.Log,
		Timeout:      40 * time.Second, // TinyGo can be slow, allow up to 40 seconds
		Callback:     w.Callback,
	}

	// Configure TinyGo builder (production/optimized)
	tinyGoConfig := baseConfig
	tinyGoConfig.Command = "tinygo"
	tinyGoConfig.CompilingArguments = func() []string {
		// TinyGo specific arguments (fixed args first, then user args)
		args := []string{"-target", "wasm", "--no-debug"}
		if w.CompilingArguments != nil {
			args = append(args, w.CompilingArguments()...)
		}
		return args
	}
	w.builderTinyGo = gobuild.New(&tinyGoConfig)

	// Configure Go standard builder (development/fast)
	goConfig := baseConfig
	goConfig.Command = "go"
	goConfig.Env = []string{"GOOS=js", "GOARCH=wasm"} // Required for WASM compilation
	goConfig.CompilingArguments = func() []string {
		// Go standard specific arguments (fixed args first, then user args)
		args := []string{"-tags", "dev"}
		if w.CompilingArguments != nil {
			args = append(args, w.CompilingArguments()...)
		}
		return args
	}
	w.builderGo = gobuild.New(&goConfig)

	// Set current builder based on compiler selection
	w.getCurrentBuilder()
}

// getCurrentBuilder sets the current active builder based on TinyGoCompiler setting
func (w *TinyWasm) getCurrentBuilder() {
	if w.TinyGoCompiler() {
		w.builder = w.builderTinyGo
	} else {
		w.builder = w.builderGo
	}
}

// getCompilerCommand returns the appropriate compiler command based on current settings
func (w *TinyWasm) getCompilerCommand() string {
	if w.TinyGoCompiler() {
		return "tinygo"
	}
	return "go"
}

// updateBuilderConfig updates the current builder when compiler settings change
func (w *TinyWasm) updateBuilderConfig() {
	if w.builder != nil {
		// Cancel any ongoing compilation
		w.builder.Cancel()
	}

	// Update current builder selection
	w.getCurrentBuilder()
}

// SetTinyGoCompiler validates and sets the TinyGo compiler preference
// Automatically recompiles main.wasm.go when compiler type changes
func (w *TinyWasm) SetTinyGoCompiler(newValue any) (string, error) {
	boolValue, ok := newValue.(bool)
	if !ok {
		err := fmt.Errorf("TinyGoCompiler expects boolean value, got %T", newValue)
		if w.Log != nil {
			fmt.Fprintf(w.Log, "Error: %v\n", err)
		}
		return "", err
	}

	// Store previous compiler state to detect changes
	previousCompiler := w.tinyGoCompiler

	// If trying to enable TinyGo, verify it's installed
	if boolValue && !w.tinyGoInstalled {
		if err := w.VerifyTinyGoInstallation(); err != nil {
			errMsg := fmt.Sprintf("Cannot enable TinyGo compiler: %v", err)
			if w.Log != nil {
				fmt.Fprintf(w.Log, "Error: %s\n", errMsg)
			}
			return errMsg, errors.New(errMsg)
		}
		w.tinyGoInstalled = true
	}

	w.tinyGoCompiler = boolValue

	status := "disabled"
	if boolValue {
		status = "enabled"
	}

	msg := fmt.Sprintf("TinyGo compiler %s", status)
	if w.Log != nil {
		fmt.Fprintf(w.Log, "Info: %s\n", msg)
	}

	// If compiler type changed, update builder config and recompile
	if previousCompiler != w.tinyGoCompiler {
		w.updateBuilderConfig()

		// Automatically recompile main.wasm.go if it exists
		if err := w.recompileMainWasm(); err != nil {
			if w.Log != nil {
				fmt.Fprintf(w.Log, "Warning: Auto-recompilation failed: %v\n", err)
			}
		} else {
			if w.Log != nil {
				fmt.Fprintf(w.Log, "Info: Auto-recompilation completed with %s\n", status)
			}
		}
	}

	return msg, nil
}

// recompileMainWasm recompiles the main WASM file if it exists
func (w *TinyWasm) recompileMainWasm() error {
	if w.builder == nil {
		return errors.New("builder not initialized")
	}

	rootFolder, _ := w.WebFilesFolder()
	mainWasmPath := path.Join(rootFolder, w.mainInputFile)

	// Check if main.wasm.go exists
	if _, err := os.Stat(mainWasmPath); err != nil {
		return errors.New("main WASM file not found: " + mainWasmPath)
	}

	// Use gobuild to compile
	return w.builder.CompileProgram()
}

// verifyTinyGoInstallationStatus checks and caches TinyGo installation status
func (w *TinyWasm) verifyTinyGoInstallationStatus() {
	if err := w.VerifyTinyGoInstallation(); err != nil {
		w.tinyGoInstalled = false
		if w.Log != nil {
			fmt.Fprintf(w.Log, "Warning: TinyGo not available: %v\n", err)
		}
	} else {
		w.tinyGoInstalled = true

		// If TinyGo is installed, check its version
		version, err := w.GetTinyGoVersion()
		if err != nil {
			w.tinyGoInstalled = false
			if w.Log != nil {
				fmt.Fprintf(w.Log, "Warning: TinyGo version check failed: %v\n", err)
			}
		} else {
			w.tinyGoInstalled = true
			if w.Log != nil {
				fmt.Fprintf(w.Log, "Info: TinyGo installation verified %v  \n", version)
			}
		}
	}
}

// getWasmExecJsPathTinyGo returns the path to TinyGo's wasm_exec.js file
func (w *TinyWasm) getWasmExecJsPathTinyGo() (string, error) {
	path, err := exec.LookPath("tinygo")
	if err != nil {
		return "", err
	}
	// Get installation directory
	tinyGoDir := filepath.Dir(path)
	// Clean path and remove "\bin"
	tinyGoDir = strings.TrimSuffix(tinyGoDir, "\\bin")
	// Build complete path to wasm_exec.js file
	return filepath.Join(tinyGoDir, "targets", "wasm_exec.js"), nil
}

// getWasmExecJsPathGo returns the path to Go's wasm_exec.js file
func (w *TinyWasm) getWasmExecJsPathGo() (string, error) {
	// Get Go installation directory path from GOROOT environment variable
	path, er := exec.LookPath("go")
	if er != nil {
		return "", er
	}
	// Get installation directory
	GoDir := filepath.Dir(path)
	// Clean path and remove "\bin"
	GoDir = strings.TrimSuffix(GoDir, "\\bin")
	// Build complete path to wasm_exec.js file
	return filepath.Join(GoDir, "misc", "wasm", "wasm_exec.js"), nil
}

// IsFrontendFile checks if a file should trigger WASM compilation based on frontend prefixes
func (w *TinyWasm) IsFrontendFile(filename string) bool {
	if len(filename) < 3 {
		return false
	}

	// Check frontend prefixes
	for _, prefix := range w.FrontendPrefix {
		if strings.HasPrefix(filename, prefix) {
			return true
		}
	}

	return false
}

// ShouldCompileToWasm determines if a file should trigger WASM compilation
func (w *TinyWasm) ShouldCompileToWasm(fileName, filePath string) bool {
	// Always compile main.wasm.go
	if fileName == w.mainInputFile {
		return true
	}

	// Always compile .wasm.go files in modules
	if strings.HasSuffix(fileName, ".wasm.go") {
		return true
	}

	// Check if it's a frontend file by prefix (only if configured)
	if len(w.FrontendPrefix) > 0 {
		for _, prefix := range w.FrontendPrefix {
			if strings.HasPrefix(fileName, prefix) {
				return true
			}
		}
	}

	// Go files in modules: check for unknown prefixes with dot
	if strings.HasSuffix(fileName, ".go") && (strings.Contains(filePath, "/modules/") || strings.Contains(filePath, "\\modules\\")) {
		// If file has a prefix with dot (prefix.name.go) and it's not in our known frontend prefixes,
		// assume it's a backend file and don't compile
		if strings.Contains(fileName, ".") {
			// Extract potential prefix (everything before first dot + dot)
			parts := strings.Split(fileName, ".")
			if len(parts) >= 3 { // prefix.name.go = 3 parts minimum
				potentialPrefix := parts[0] + "."

				// Check if this prefix is in our known frontend prefixes
				isKnownFrontend := false
				for _, prefix := range w.FrontendPrefix {
					if potentialPrefix == prefix {
						isKnownFrontend = true
						break
					}
				}

				// If it has a prefix with dot but it's not a known frontend prefix, don't compile
				if !isKnownFrontend {
					return false
				}
			}
		}

		// Regular Go files in modules without prefixes should compile
		return true
	}

	// All other files should be ignored
	return false
}

// MainOutputFile returns the complete path to the main WASM output file
func (w *TinyWasm) MainOutputFile() string {
	if w.builder == nil {
		return "main.wasm" // fallback
	}
	rootFolder, subFolder := w.WebFilesFolder()
	return path.Join(rootFolder, subFolder, w.builder.MainOutputFileNameWithExtension())
}
