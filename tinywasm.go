package tinywasm

import (
	"os"
	"path"
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

// detectFromExistingWasmExecJs checks for existing wasm_exec.js file
func (w *TinyWasm) detectFromExistingWasmExecJs() bool {
	wasmExecPath := w.getWasmExecJsOutputPath()

	// Check if file exists
	if _, err := os.Stat(wasmExecPath); err != nil {
		return false
	}

	// Analyze content to determine compiler type
	return w.analyzeWasmExecJsContent(wasmExecPath)
}

// getWasmExecJsOutputPath returns the output path for wasm_exec.js
func (w *TinyWasm) getWasmExecJsOutputPath() string {
	return path.Join(w.Config.AppRootDir, w.Config.WebFilesRootRelative, w.Config.WebFilesSubRelativeJsOutput, "wasm_exec.js")
}

// analyzeWasmExecJsContent analyzes existing wasm_exec.js to determine compiler type
func (w *TinyWasm) analyzeWasmExecJsContent(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		w.Logger("Error reading wasm_exec.js for detection:", err)
		return false
	}

	content := string(data)

	// Count signatures (reuse existing logic from wasmDetectionFuncFromJsFileActive)
	goCount := 0
	for _, s := range wasm_execGoSignatures() {
		if Contains(content, s) {
			goCount++
		}
	}

	tinyCount := 0
	for _, s := range wasm_execTinyGoSignatures() {
		if Contains(content, s) {
			tinyCount++
		}
	}

	// Determine configuration based on signatures
	if tinyCount > goCount && tinyCount > 0 {
		w.tinyGoCompiler = true
		w.wasmProject = true
		//w.Logger("DEBUG: Detected TinyGo compiler from wasm_exec.js")
	} else if goCount > tinyCount && goCount > 0 {
		w.tinyGoCompiler = false
		w.wasmProject = true
		//w.Logger("DEBUG: Detected Go compiler from wasm_exec.js")
	} else if tinyCount > 0 || goCount > 0 {
		// Single-sided detection
		w.tinyGoCompiler = tinyCount > 0
		w.wasmProject = true
		//compiler := map[bool]string{true: "TinyGo", false: "Go"}[w.tinyGoCompiler]
		//w.Logger("DEBUG: Detected WASM project, compiler:", compiler)
	} else {
		//w.Logger("DEBUG: No valid WASM signatures found in wasm_exec.js")
		return false
	}

	// After detecting runtime signatures, try to recover last-used mode from header
	// This gives priority to the user's explicit mode choice over signature defaults
	if mode, ok := w.getModeFromWasmExecJsHeader(content); ok {
		w.currentMode = mode
		// Set activeBuilder according to recovered mode
		if w.requiresTinyGo(mode) {
			w.activeBuilder = w.builderDebug
		} else {
			w.activeBuilder = w.builderCoding
		}
		//w.Logger("DEBUG: Restored mode from wasm_exec.js header:", mode)
	} else {
		// No header found, use signature-based defaults
		if w.tinyGoCompiler {
			w.activeBuilder = w.builderDebug
			w.currentMode = w.Config.DebuggingShortcut
		} else {
			w.activeBuilder = w.builderCoding
			w.currentMode = w.Config.CodingShortcut
		}
		//w.Logger("DEBUG: Using signature-based default mode:", w.currentMode)
	}

	return true

	//w.Logger("DEBUG: No valid WASM signatures found in wasm_exec.js")
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

// wasmProjectWriteOrReplaceWasmExecJsOutput writes (or overwrites) the
// wasm_exec.js initialization file into the configured web output folder for
// WASM projects. If the receiver is not a WASM project the function returns
// false immediately. On success or on any write attempt it returns true; any
// filesystem or generation errors are logged via w.Logger and treated as
// non-fatal so callers can continue their workflow.
func (w *TinyWasm) wasmProjectWriteOrReplaceWasmExecJsOutput() {
	// Only perform actions for recognized WASM projects
	if !w.wasmProject {
		w.Logger("DEBUG: Not a WASM project, skipping wasm_exec.js write")
		return
	}

	outputPath := w.getWasmExecJsOutputPath()

	w.Logger("DEBUG: Writing/overwriting wasm_exec.js to output path:", outputPath)

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		w.Logger("Failed to create output directory:", err)
		return // We did attempt the operation (project), but treat errors as non-fatal
	}

	// Get the complete JavaScript initialization code (includes WASM setup)
	jsContent, err := w.javascriptForInitializing()
	if err != nil {
		w.Logger("Failed to generate JavaScript initialization code:", err)
		return
	}

	// Write the complete JavaScript to output location, always overwrite
	if err := os.WriteFile(outputPath, []byte(jsContent), 0644); err != nil {
		w.Logger("Failed to write JavaScript initialization file:", err)
		return
	}

	w.Logger(" DEBUG: Wrote/overwrote JavaScript initialization file in output directory")
}
