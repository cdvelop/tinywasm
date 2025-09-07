package tinywasm

import (
	"os"
	"path"

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
	tinyGoCompiler  bool // Enable TinyGo compiler (default: false for faster development)
	wasmProject     bool // Automatically detected based on file structure
	tinyGoInstalled bool // Cached TinyGo installation status

	// NEW: Explicit mode tracking to fix Value() method
	currentMode string // Track current mode explicitly ("c", "d", "p")

	goWasmJsCache     string
	tinyGoWasmJsCache string

	// Function pointer for efficient WASM project detection
	wasmDetectionFuncFromGoFile func(string, string) // (fileName, filePath)
	wasmDetectionFuncFromJsFile func(fileName, extension, filePath, event string)
}

// Config holds configuration for WASM compilation
type Config struct {

	// AppRootDir specifies the application root directory (absolute).
	// e.g., "/home/user/project". If empty, defaults to "." to preserve existing behavior.
	AppRootDir           string
	WebFilesRootRelative string // root web folder (relative) eg: "web"
	WebFilesSubRelative  string // subfolder under root (relative) eg: "public"
	Logger               func(message ...any)
	// TinyGoCompiler removed: tinyGoCompiler (private) in TinyWasm is used instead to avoid confusion

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
		AppRootDir:         ".",
		CodingShortcut:     "c",
		DebuggingShortcut:  "d",
		ProductionShortcut: "p",
	}
}

// New creates a new TinyWasm instance with the provided configuration
// Timeout is set to 40 seconds maximum as TinyGo compilation can be slow
// Default values: mainInputFile="main.wasm.go"
func New(c *Config) *TinyWasm {
	// Ensure we have a config and a default AppRootDir
	if c == nil {
		c = NewConfig()
	}
	if c.AppRootDir == "" {
		c.AppRootDir = "."
	}

	w := &TinyWasm{
		Config:        c,
		mainInputFile: "main.wasm.go",

		// Initialize dynamic fields
		tinyGoCompiler:  false, // Default to fast Go compilation; enable later via TinyWasm methods if desired
		wasmProject:     false, // Auto-detected later
		tinyGoInstalled: false, // Verified on first use

		// Initialize with default mode
		currentMode: c.CodingShortcut, // Start with coding mode
	}

	if w.currentMode == "" {
		w.currentMode = w.Config.CodingShortcut
	}

	// Initialize WASM detection function pointer (starts active)

	// FROM JS FILE
	w.wasmDetectionFuncFromJsFile = w.wasmDetectionFuncFromJsFileActive
	// FROM GO FILE
	w.wasmDetectionFuncFromGoFile = w.wasmDetectionFuncFromGoFileActive

	// Check TinyGo installation status
	w.verifyTinyGoInstallationStatus()

	// Initialize gobuild instance with WASM-specific configuration
	w.initializeBuilder()

	return w
}

func (w *TinyWasm) SupportedExtensions() []string {
	return []string{".js", ".go"}
}

// Name returns the name of the WASM project
func (w *TinyWasm) Name() string {
	return "TinyWasm"
}

// WasmProjectTinyGoJsUse returns dynamic state based on current configuration
func (w *TinyWasm) WasmProjectTinyGoJsUse() (bool, bool) {
	// Update TinyGo compiler state based on current mode
	currentMode := w.Value()
	useTinyGo := w.requiresTinyGo(currentMode) && w.tinyGoInstalled

	return w.wasmProject, useTinyGo
}

// TinyGoCompiler returns if TinyGo compiler should be used (dynamic based on configuration)
func (w *TinyWasm) TinyGoCompiler() bool {
	return w.tinyGoCompiler && w.tinyGoInstalled
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
		progress(err)
		return
	}

	// Check TinyGo installation for debug/production modes
	if w.requiresTinyGo(newValue) && !w.tinyGoInstalled {
		// handleTinyGoMissing returns an error with descriptive message
		progress(w.handleTinyGoMissing().Error())
		return
	}

	// Update active builder
	w.updateCurrentBuilder(newValue)

	// Check if main WASM file exists before attempting compilation
	rootFolder := path.Join(w.AppRootDir, w.Config.WebFilesRootRelative)
	mainWasmPath := path.Join(rootFolder, w.mainInputFile)
	if _, err := os.Stat(mainWasmPath); err != nil {
		// File doesn't exist, just report success message without compilation
		progress(w.getSuccessMessage(newValue))
		return
	}

	// Auto-recompile with appropriate message format for MessageType detection
	if err := w.recompileMainWasm(); err != nil {
		// Report warning message via progress (don't treat as fatal)
		warningMsg := Translate("Warning:", "auto", "compilation", "failed:", err).String()
		if warningMsg == "" {
			warningMsg = "Warning: auto compilation failed: " + err.Error()
		}
		progress(warningMsg)
		return
	}

	// Report success
	progress(w.getSuccessMessage(newValue))
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
		return Err("builder not initialized")
	}
	rootFolder := path.Join(w.AppRootDir, w.Config.WebFilesRootRelative)
	mainWasmPath := path.Join(rootFolder, w.mainInputFile)

	// Check if main.wasm.go exists
	if _, err := os.Stat(mainWasmPath); err != nil {
		return Err("main WASM file not found:", mainWasmPath)
	}

	// Use gobuild to compile
	return w.activeBuilder.CompileProgram()
}

// (Deprecated field FrontendPrefix removed) frontend detection is no longer supported.

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
