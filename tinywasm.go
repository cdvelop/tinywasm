package tinywasm

import (
	"os"
	"path/filepath"

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

	modeC_go_wasm_exec_cache     string // cache wasm_exec.js file content per mode coding
	modeD_tinygo_wasm_exec_cache string // cache wasm_exec.js file content per mode debug
	modeP_tinygo_wasm_exec_cache string // cache wasm_exec.js file content per mode production
}

// Config holds configuration for WASM compilation
type Config struct {

	// AppRootDir specifies the application root directory (absolute).
	// e.g., "/home/user/project". If empty, defaults to "." to preserve existing behavior.
	AppRootDir                  string
	WebFilesRootRelative        string // root web folder (relative) eg: "web"
	WebFilesSubRelative         string // subfolder under root (relative) eg: "public"
	WebFilesSubRelativeJsOutput string // output path for js files (relative) eg: "theme/js"
	Logger                      func(message ...any)
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
		AppRootDir:                  ".",
		WebFilesRootRelative:        "web",
		WebFilesSubRelativeJsOutput: "theme/js",
		CodingShortcut:              "c",
		DebuggingShortcut:           "d",
		ProductionShortcut:          "p",
		Logger: func(message ...any) {
			// Default logger: do nothing (silent operation)
		},
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

	// Set default logger if not provided
	if c.Logger == nil {
		c.Logger = func(message ...any) {
			// Default logger: do nothing (silent operation)
		}
	}

	// Ensure shortcut defaults are set even when a partial config is passed
	// Use NewConfig() as the authoritative source of defaults and copy any
	// missing shortcut values from it.
	defaults := NewConfig()
	if c.CodingShortcut == "" {
		c.CodingShortcut = defaults.CodingShortcut
	}
	if c.DebuggingShortcut == "" {
		c.DebuggingShortcut = defaults.DebuggingShortcut
	}
	if c.ProductionShortcut == "" {
		c.ProductionShortcut = defaults.ProductionShortcut
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

	// Set default for WebFilesSubRelativeJsOutput if not configured
	if w.Config.WebFilesSubRelativeJsOutput == "" {
		w.Config.WebFilesSubRelativeJsOutput = "theme/js"
	}

	// Check TinyGo installation status
	w.verifyTinyGoInstallationStatus()

	// Initialize gobuild instance with WASM-specific configuration
	w.builderWasmInit()

	// Perform one-time detection at the end
	w.detectProjectConfiguration()

	return w
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

// detectProjectConfiguration performs one-time detection during initialization
func (w *TinyWasm) detectProjectConfiguration() {
	// Priority 1: Check for existing wasm_exec.js (definitive source)
	if w.detectFromExistingWasmExecJs() {
		//w.Logger("DEBUG: WASM project detected from existing wasm_exec.js")
		return
	}

	// Priority 2: Check for .wasm.go files (confirms WASM project)
	if w.detectFromGoFiles() {
		//w.Logger("DEBUG: WASM project detected from .wasm.go files, defaulting to Go compiler")
		w.wasmProject = true
		w.tinyGoCompiler = false
		w.currentMode = w.Config.CodingShortcut

		// Ensure wasm_exec.js is present in output (create/overwrite as needed)
		// This writes the initialization JS so downstream flows (tests/compile) have it.
		w.wasmProjectWriteOrReplaceWasmExecJsOutput()
		return
	}

	w.Logger("No WASM project detected")
}

// detectFromGoFiles checks for .wasm.go files to confirm WASM project
func (w *TinyWasm) detectFromGoFiles() bool {
	// Walk the project directory to find .wasm.go files
	wasmFilesFound := false

	err := filepath.Walk(w.Config.AppRootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking even if there's an error
		}

		if info.IsDir() {
			return nil // Continue walking directories
		}

		fileName := info.Name()

		// Check for main.wasm.go file (strong indicator of WASM project)
		if fileName == w.mainInputFile {
			wasmFilesFound = true
			return filepath.SkipAll // Found main file, can stop walking
		}

		// Check for .wasm.go files in modules (another strong indicator)
		if HasSuffix(fileName, ".wasm.go") {
			wasmFilesFound = true
			return filepath.SkipAll // Found wasm file, can stop walking
		}

		return nil
	})

	if err != nil {
		w.Logger("Error walking directory for WASM file detection:", err)
		return false
	}

	return wasmFilesFound
}
