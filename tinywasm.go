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

	// RENAME & ADD: 4 builders for complete mode coverage
	builderLarge  *gobuild.GoBuild // Go standard - fast compilation
	builderMedium *gobuild.GoBuild // TinyGo debug - easier debugging
	builderSmall  *gobuild.GoBuild // TinyGo production - smallest size
	activeBuilder *gobuild.GoBuild // Current active builder

	// EXISTING: Keep for installation detection (no compilerMode needed - activeBuilder handles state)
	tinyGoCompiler  bool // Enable TinyGo compiler (default: false for faster development)
	wasmProject     bool // Automatically detected based on file structure
	tinyGoInstalled bool // Cached TinyGo installation status

	// NEW: Explicit mode tracking to fix Value() method
	currentMode string // Track current mode explicitly ("L", "M", "S")

	mode_large_go_wasm_exec_cache      string // cache wasm_exec.js file content per mode large
	mode_medium_tinygo_wasm_exec_cache string // cache wasm_exec.js file content per mode medium
	mode_small_tinygo_wasm_exec_cache  string // cache wasm_exec.js file content per mode small
}

// Config holds configuration for WASM compilation
type Config struct {

	// AppRootDir specifies the application root directory (absolute).
	// e.g., "/home/user/project". If empty, defaults to "." to preserve existing behavior.
	AppRootDir string

	// SourceDir specifies the directory containing the Go source for the webclient (relative to AppRootDir).
	// e.g., "src/cmd/webclient"
	SourceDir string

	// OutputDir specifies the directory for WASM and related assets (relative to AppRootDir).
	// e.g., "src/web/public"
	OutputDir string

	WasmExecJsOutputDir string // output dir for wasm_exec.js file (relative) eg: "src/web/ui/js", "theme/js"
	MainInputFile       string // main input file for WASM compilation (default: "main.wasm.go")
	OutputName          string // output name for WASM file (default: "main")
	Logger              func(message ...any)
	// TinyGoCompiler removed: tinyGoCompiler (private) in TinyWasm is used instead to avoid confusion

	BuildLargeSizeShortcut  string // "L" (Large) compile with go
	BuildMediumSizeShortcut string // "M" (Medium) compile with tinygo debug
	BuildSmallSizeShortcut  string // "S" (Small) compile with tinygo minimal binary size

	// gobuild integration fields
	Callback           func(error)     // Optional callback for async compilation
	CompilingArguments func() []string // Build arguments for compilation (e.g., ldflags)

	// DisableWasmExecJsOutput prevents automatic creation of wasm_exec.js file
	// Useful when embedding wasm_exec.js content inline (e.g., Cloudflare Pages Advanced Mode)
	DisableWasmExecJsOutput bool

	// LastOperationID tracks the last operation ID for progress reporting
	lastOpID string
}

// NewConfig creates a TinyWasm Config with sensible defaults
func NewConfig() *Config {
	return &Config{
		AppRootDir:              ".",
		SourceDir:               "src/cmd/webclient",
		OutputDir:               "src/web/public",
		WasmExecJsOutputDir:     "src/web/ui/js",
		MainInputFile:           "main.go",
		OutputName:              "main",
		BuildLargeSizeShortcut:  "L",
		BuildMediumSizeShortcut: "M",
		BuildSmallSizeShortcut:  "S",
		Logger: func(message ...any) {
			// Default logger: do nothing (silent operation)
		},
	}
}

// New creates a new TinyWasm instance with the provided configuration
// Timeout is set to 40 seconds maximum as TinyGo compilation can be slow
// Default values: MainInputFile in Config defaults to "main.wasm.go"
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
	if c.BuildLargeSizeShortcut == "" {
		c.BuildLargeSizeShortcut = defaults.BuildLargeSizeShortcut
	}
	if c.BuildMediumSizeShortcut == "" {
		c.BuildMediumSizeShortcut = defaults.BuildMediumSizeShortcut
	}
	if c.BuildSmallSizeShortcut == "" {
		c.BuildSmallSizeShortcut = defaults.BuildSmallSizeShortcut
	}
	if c.MainInputFile == "" {
		c.MainInputFile = defaults.MainInputFile
	}
	if c.OutputName == "" {
		c.OutputName = defaults.OutputName
	}

	w := &TinyWasm{
		Config: c,

		// Initialize dynamic fields
		tinyGoCompiler:  false, // Default to fast Go compilation; enable later via TinyWasm methods if desired
		wasmProject:     false, // Auto-detected later
		tinyGoInstalled: false, // Verified on first use

		// Initialize with default mode
		currentMode: c.BuildLargeSizeShortcut, // Start with coding mode
	}

	if w.currentMode == "" {
		w.currentMode = w.Config.BuildLargeSizeShortcut
	}

	// Set default for WasmExecJsOutputDir if not configured
	if w.Config.WasmExecJsOutputDir == "" {
		w.Config.WasmExecJsOutputDir = "src/web/ui/js"
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
		return w.Config.BuildLargeSizeShortcut // Default to coding mode
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
		w.currentMode = w.Config.BuildLargeSizeShortcut

		// Ensure wasm_exec.js is present in output (create/overwrite as needed)
		// This writes the initialization JS so downstream flows (tests/compile) have it.
		// Skip if DisableWasmExecJsOutput is set (e.g., for inline embedding scenarios)
		if !w.Config.DisableWasmExecJsOutput {
			w.wasmProjectWriteOrReplaceWasmExecJsOutput()
		}
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

		// Get relative path from AppRootDir for comparison
		relPath, err := filepath.Rel(w.Config.AppRootDir, path)
		if err != nil {
			relPath = path // Fallback to absolute path if relative fails
		}

		fileName := info.Name()

		// Check for main input file in the source directory (strong indicator of WASM project)
		expectedPath := filepath.Join(w.Config.SourceDir, w.Config.MainInputFile)
		if relPath == expectedPath {
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
