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

	. "github.com/cdvelop/tinystring"
)

// TinyWasm provides WebAssembly compilation capabilities with 3-mode compiler selection
type TinyWasm struct {
	*Config
	ModulesFolder string // default "modules". for test change eg: "test/modules"
	mainInputFile string // eg: main.wasm.go

	// RENAME & ADD: 4 builders for complete mode coverage
	builderCoding     *gobuild.GoBuild // Go standard - fast compilation
	builderDebug      *gobuild.GoBuild // TinyGo debug - easier debugging
	builderProduction *gobuild.GoBuild // TinyGo production - smallest size
	activeBuilder     *gobuild.GoBuild // Current active builder

	// EXISTING: Keep for installation detection (no compilerMode needed - activeBuilder handles state)
	tinyGoCompiler  bool // Enable TinyGo compiler
	wasmProject     bool // Automatically detected based on file structure
	tinyGoInstalled bool // Cached TinyGo installation status

	goWasmJsCache     string
	tinyGoWasmJsCache string

	// Function pointer for efficient WASM project detection
	wasmDetectionFunc func(string, string) // (fileName, filePath)
	rootDir           string               // Project root directory, default "."
}

// Config holds configuration for WASM compilation
type Config struct {
	// WebFilesFolder returns root web folder and subfolder eg: "web","public"
	WebFilesFolder func() (string, string)
	Writer         io.Writer // For logging output to external systems (e.g., TUI, console)
	FrontendPrefix []string  // Prefixes used to identify frontend files (e.g., "f.", "front.")
	TinyGoCompiler bool      // Enable TinyGo compiler (default: false for faster development)

	// NEW: Shortcut configuration (default: "c", "d", "p")
	CodingShortcut     string // default "c"
	DebuggingShortcut  string // default "d"
	ProductionShortcut string // default "p"

	// gobuild integration fields
	Callback           func(error)     // Optional callback for async compilation
	CompilingArguments func() []string // Build arguments for compilation (e.g., ldflags)
}

// NewConfig creates a TinyWasm Config with sensible defaults
func NewConfig() *Config {
	return &Config{
		CodingShortcut:     "c",
		DebuggingShortcut:  "d",
		ProductionShortcut: "p",
		TinyGoCompiler:     false, // Default to fast Go compilation
		FrontendPrefix:     []string{"f.", "front."},
	}
}

// New creates a new TinyWasm instance with the provided configuration
// Timeout is set to 40 seconds maximum as TinyGo compilation can be slow
// Default values: ModulesFolder="modules", mainInputFile="main.wasm.go"
func New(c *Config) *TinyWasm {
	w := &TinyWasm{
		Config:        c,
		ModulesFolder: "modules",
		mainInputFile: "main.wasm.go",
		rootDir:       ".", // Default root directory

		// Initialize dynamic fields
		tinyGoCompiler:  c.TinyGoCompiler, // Use config preference
		wasmProject:     false,            // Auto-detected later
		tinyGoInstalled: false,            // Verified on first use
	}

	// Initialize WASM detection function pointer (starts active)
	w.wasmDetectionFunc = w.updateWasmProjectDetectionActive

	// Check TinyGo installation status
	w.verifyTinyGoInstallationStatus()

	// Initialize gobuild instance with WASM-specific configuration
	w.initializeBuilder()

	return w
}

// Name returns the name of the WASM project
func (w *TinyWasm) Name() string {
	return "TinyWasm"
}

// WasmProjectTinyGoJsUse returns dynamic state based on current configuration
func (w *TinyWasm) WasmProjectTinyGoJsUse() (bool, bool) {
	return w.wasmProject, w.tinyGoCompiler
}

// TinyGoCompiler returns if TinyGo compiler should be used (dynamic based on configuration)
func (w *TinyWasm) TinyGoCompiler() bool {
	return w.tinyGoCompiler && w.tinyGoInstalled
}

// initializeBuilder configures 3 builders for WASM compilation modes
func (w *TinyWasm) initializeBuilder() {
	rootFolder, subFolder := w.WebFilesFolder()
	mainFilePath := path.Join(rootFolder, w.mainInputFile)
	outFolder := path.Join(rootFolder, subFolder)

	// Base configuration shared by all builders
	baseConfig := gobuild.Config{
		MainFilePath: mainFilePath,
		OutName:      "main", // Output will be main.wasm
		Extension:    ".wasm",
		OutFolder:    outFolder,
		Writer:       w.Writer,
		Timeout:      60 * time.Second, // 1 minute for all modes
		Callback:     w.Callback,
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

// getCurrentMode determines current mode based on activeBuilder
func (w *TinyWasm) getCurrentMode() string {
	switch w.activeBuilder {
	case w.builderCoding:
		return w.Config.CodingShortcut // "c"
	case w.builderDebug:
		return w.Config.DebuggingShortcut // "d"
	case w.builderProduction:
		return w.Config.ProductionShortcut // "p"
	default:
		return w.Config.CodingShortcut // fallback
	}
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

// requiresTinyGo checks if the mode requires TinyGo compiler
func (w *TinyWasm) requiresTinyGo(mode string) bool {
	return mode == w.Config.DebuggingShortcut || mode == w.Config.ProductionShortcut
}

// installTinyGo placeholder for future TinyGo installation
func (w *TinyWasm) installTinyGo() error {
	return Err("TinyGo", "installation", D.Not, "implemented")
}

// handleTinyGoMissing handles missing TinyGo installation
func (w *TinyWasm) handleTinyGoMissing() error {
	if err := w.installTinyGo(); err != nil {
		return Err("Error:", D.Cannot, "install TinyGo:", err.Error())
	}

	// Re-verify installation
	if err := w.VerifyTinyGoInstallation(); err != nil {
		return err
	}

	w.tinyGoInstalled = true
	return nil
}

// getSuccessMessage returns appropriate success message for mode
func (w *TinyWasm) getSuccessMessage(mode string) string {
	switch mode {
	case w.Config.CodingShortcut:
		return Translate("Switching", "to", "coding", "mode").String()
	case w.Config.DebuggingShortcut:
		return Translate("Switching", "to", "debugging", "mode").String()
	case w.Config.ProductionShortcut:
		return Translate("Switching", "to", "production", "mode").String()
	default:
		return Translate(D.Invalid, "mode").String()
	}
}

// RENAME: SetTinyGoCompiler -> Change (implements DevTUI FieldHandler interface)
func (w *TinyWasm) Change(newValue any) (string, error) {
	modeStr, ok := newValue.(string)
	if !ok {
		return "", Err(D.Invalid, "input", D.Type)
	}

	// Validate mode
	if err := w.validateMode(modeStr); err != nil {
		return "", err
	}

	// Check TinyGo installation for debug/production modes
	if w.requiresTinyGo(modeStr) && !w.tinyGoInstalled {
		if err := w.handleTinyGoMissing(); err != nil {
			return "", err
		}
	}

	// Update active builder
	w.updateCurrentBuilder(modeStr)

	// Auto-recompile with appropriate message format for MessageType detection
	if err := w.recompileMainWasm(); err != nil {
		// Return warning message - MessageType will detect "Warning:" keyword
		warningMsg := Translate("Warning:", "auto", "compilation", "failed:", err).String()
		return warningMsg, nil // Don't fail the mode change
	}

	return w.getSuccessMessage(modeStr), nil
}

// === DevTUI FieldHandler Interface Implementation ===

// Label returns the field label for DevTUI display
func (w *TinyWasm) Label() string {
	return "Compiler Mode"
}

// Value returns the current compiler mode shortcut (c, d, or p)
func (w *TinyWasm) Value() string {
	// Determine current mode based on activeBuilder
	if w.activeBuilder == w.builderCoding {
		return w.Config.CodingShortcut
	}
	if w.activeBuilder == w.builderDebug {
		return w.Config.DebuggingShortcut
	}
	if w.activeBuilder == w.builderProduction {
		return w.Config.ProductionShortcut
	}

	// Default to coding mode if no active builder
	return w.Config.CodingShortcut
}

// Editable returns true indicating this field can be modified by the user
func (w *TinyWasm) Editable() bool {
	return true
}

// Timeout returns no timeout (zero duration) for mode changes
func (w *TinyWasm) Timeout() time.Duration {
	return 0 // No timeout needed
}

// recompileMainWasm recompiles the main WASM file if it exists
func (w *TinyWasm) recompileMainWasm() error {
	if w.activeBuilder == nil {
		return errors.New("builder not initialized")
	}

	rootFolder, _ := w.WebFilesFolder()
	mainWasmPath := path.Join(rootFolder, w.mainInputFile)

	// Check if main.wasm.go exists
	if _, err := os.Stat(mainWasmPath); err != nil {
		return errors.New("main WASM file not found: " + mainWasmPath)
	}

	// Use gobuild to compile
	return w.activeBuilder.CompileProgram()
}

// verifyTinyGoInstallationStatus checks and caches TinyGo installation status
func (w *TinyWasm) verifyTinyGoInstallationStatus() {
	if err := w.VerifyTinyGoInstallation(); err != nil {
		w.tinyGoInstalled = false
		if w.Writer != nil {
			fmt.Fprintf(w.Writer, "Warning: TinyGo not available: %v\n", err)
		}
	} else {
		w.tinyGoInstalled = true

		// If TinyGo is installed, check its version
		version, err := w.GetTinyGoVersion()
		if err != nil {
			w.tinyGoInstalled = false
			if w.Writer != nil {
				fmt.Fprintf(w.Writer, "Warning: TinyGo version check failed: %v\n", err)
			}
		} else {
			w.tinyGoInstalled = true
			if w.Writer != nil {
				fmt.Fprintf(w.Writer, "Info: TinyGo installation verified %v  \n", version)
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
	if w.activeBuilder == nil {
		return "main.wasm" // fallback
	}
	rootFolder, subFolder := w.WebFilesFolder()
	return path.Join(rootFolder, subFolder, w.activeBuilder.MainOutputFileNameWithExtension())
}
