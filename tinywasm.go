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

	// NEW: Explicit mode tracking to fix Value() method
	currentMode string // Track current mode explicitly ("c", "d", "p")

	goWasmJsCache     string
	tinyGoWasmJsCache string

	// Function pointer for efficient WASM project detection
	wasmDetectionFunc func(string, string) // (fileName, filePath)
	rootDir           string               // Project root directory, default "."
}

// Config holds configuration for WASM compilation
type Config struct {
	WebFilesRootRelative string    // root web folder (relative) eg: "web"
	WebFilesSubRelative  string    // subfolder under root (relative) eg: "public"
	Logger               io.Writer // For logging output to external systems (e.g., TUI, console)
	TinyGoCompiler       bool      // Enable TinyGo compiler (default: false for faster development)

	// NEW: Shortcut configuration (default: "c", "d", "p")
	CodingShortcut     string // coding "c" compile fast with go
	DebuggingShortcut  string // debugging "d" compile with tinygo debug
	ProductionShortcut string // production "p" compile with tinygo minimal binary size

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
	}
}

// New creates a new TinyWasm instance with the provided configuration
// Timeout is set to 40 seconds maximum as TinyGo compilation can be slow
// Default values: mainInputFile="main.wasm.go"
func New(c *Config) *TinyWasm {
	w := &TinyWasm{
		Config:        c,
		mainInputFile: "main.wasm.go",
		rootDir:       ".", // Default root directory

		// Initialize dynamic fields
		tinyGoCompiler:  c.TinyGoCompiler, // Use config preference
		wasmProject:     false,            // Auto-detected later
		tinyGoInstalled: false,            // Verified on first use

		// Initialize with default mode
		currentMode: c.CodingShortcut, // Start with coding mode
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
	rootFolder := w.Config.WebFilesRootRelative
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
	// installTinyGo always returns a non-nil error (not implemented)
	err := w.installTinyGo()
	return Err("Error:", D.Cannot, "install TinyGo:", err.Error())
}

// getSuccessMessage returns appropriate success message for mode
func (w *TinyWasm) getSuccessMessage(mode string) string {
	var msg string
	switch mode {
	case w.Config.CodingShortcut:
		msg = Translate("Switching", "to", "coding", "mode").String()
	case w.Config.DebuggingShortcut:
		msg = Translate("Switching", "to", "debugging", "mode").String()
	case w.Config.ProductionShortcut:
		msg = Translate("Switching", "to", "production", "mode").String()
	default:
		msg = Translate(D.Invalid, "mode").String()
	}

	// Fallback if Translate returns empty string
	if msg == "" {
		switch mode {
		case w.Config.CodingShortcut:
			msg = "Switching to coding mode"
		case w.Config.DebuggingShortcut:
			msg = "Switching to debugging mode"
		case w.Config.ProductionShortcut:
			msg = "Switching to production mode"
		default:
			msg = "Invalid mode"
		}
	}

	return msg
}

// Change updates the compiler mode for TinyWasm and reports progress via the provided callback.
// Implements the HandlerEdit interface: Change(newValue string, progress func(msgs ...any))
func (w *TinyWasm) Change(newValue string, progress func(msgs ...any)) {
	// Validate mode
	if err := w.validateMode(newValue); err != nil {
		if progress != nil {
			progress(err.Error())
		}
		return
	}

	// Check TinyGo installation for debug/production modes
	if w.requiresTinyGo(newValue) && !w.tinyGoInstalled {
		if progress != nil {
			// handleTinyGoMissing returns an error with descriptive message
			progress(w.handleTinyGoMissing().Error())
		}
		return
	}

	// Update active builder
	w.updateCurrentBuilder(newValue)

	// Check if main WASM file exists before attempting compilation
	rootFolder := w.Config.WebFilesRootRelative
	mainWasmPath := path.Join(rootFolder, w.mainInputFile)
	if _, err := os.Stat(mainWasmPath); err != nil {
		// File doesn't exist, just report success message without compilation
		if progress != nil {
			progress(w.getSuccessMessage(newValue))
		}
		return
	}

	// Auto-recompile with appropriate message format for MessageType detection
	if err := w.recompileMainWasm(); err != nil {
		// Report warning message via progress (don't treat as fatal)
		warningMsg := Translate("Warning:", "auto", "compilation", "failed:", err).String()
		if warningMsg == "" {
			warningMsg = "Warning: auto compilation failed: " + err.Error()
		}
		if progress != nil {
			progress(warningMsg)
		}
		return
	}

	// Report success
	if progress != nil {
		progress(w.getSuccessMessage(newValue))
	}
}

// === DevTUI FieldHandler Interface Implementation ===

// Label returns the field label for DevTUI display
func (w *TinyWasm) Label() string {
	return "Compiler Mode"
}

// Value returns the current compiler mode shortcut (c, d, or p)
func (w *TinyWasm) Value() string {
	// Use explicit mode tracking instead of pointer comparison
	if w.currentMode == "" {
		return w.Config.CodingShortcut // Default to coding mode
	}
	return w.currentMode
}

// recompileMainWasm recompiles the main WASM file if it exists
func (w *TinyWasm) recompileMainWasm() error {
	if w.activeBuilder == nil {
		return errors.New("builder not initialized")
	}
	rootFolder := w.Config.WebFilesRootRelative
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
		if w.Logger != nil {
			fmt.Fprintf(w.Logger, "Warning: TinyGo not available: %v\n", err)
		}
	} else {
		w.tinyGoInstalled = true

		// If TinyGo is installed, check its version
		version, err := w.GetTinyGoVersion()
		if err != nil {
			w.tinyGoInstalled = false
			if w.Logger != nil {
				fmt.Fprintf(w.Logger, "Warning: TinyGo version check failed: %v\n", err)
			}
		} else {
			w.tinyGoInstalled = true
			if w.Logger != nil {
				fmt.Fprintf(w.Logger, "Info: TinyGo installation verified %v  \n", version)
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

// (Deprecated field FrontendPrefix removed) frontend detection is no longer supported.

// ShouldCompileToWasm determines if a file should trigger WASM compilation
func (w *TinyWasm) ShouldCompileToWasm(fileName, filePath string) bool {
	// Always compile main.wasm.go
	if fileName == w.mainInputFile {
		return true
	}

	// Any .wasm.go file should trigger compilation
	if strings.HasSuffix(fileName, ".wasm.go") {
		return true
	}

	// All other files should be ignored
	return false
}
