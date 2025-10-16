package tinywasm

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	. "github.com/cdvelop/tinystring"
)

//go:embed assets/wasm_exec_go.js
var embeddedWasmExecGo []byte

//go:embed assets/wasm_exec_tinygo.js
var embeddedWasmExecTinyGo []byte

// wasm_execGoSignatures returns signatures expected in Go's wasm_exec.js
func wasm_execGoSignatures() []string {
	return []string{
		"runtime.scheduleTimeoutEvent",
		"runtime.clearTimeoutEvent",
		"runtime.wasmExit",
		// note: removed shared or ambiguous signatures such as syscall/js.valueGet
	}
}

// wasm_execTinyGoSignatures returns signatures expected in TinyGo's wasm_exec.js
func wasm_execTinyGoSignatures() []string {
	return []string{
		"runtime.sleepTicks",
		"runtime.ticks",
		"$runtime.alloc",
		"tinygo_js",
	}
}

// WasmExecJsOutputPath returns the output path for wasm_exec.js
func (w *TinyWasm) WasmExecJsOutputPath() string {
	return path.Join(w.Config.AppRootDir, w.Config.WasmExecJsOutputDir, "wasm_exec.js")
}

// getWasmExecContent returns the raw wasm_exec.js content for the current compiler configuration.
// This method returns the unmodified content from embedded assets without any headers or caching.
// It relies on TinyWasm's internal state (via WasmProjectTinyGoJsUse) to determine which
// compiler (Go vs TinyGo) to use.
//
// The returned content is suitable for:
//   - Direct file output
//   - Integration into build tools
//   - Embedding in worker scripts
//
// Note: This method does NOT add mode headers or perform caching. Those responsibilities
// belong to JavascriptForInitializing() which is used for the internal initialization flow.
func (w *TinyWasm) getWasmExecContent() ([]byte, error) {
	// Determine project type and compiler from TinyWasm state
	wasmType, TinyGoCompiler := w.WasmProjectTinyGoJsUse()
	if !wasmType {
		return nil, Errf("not a WASM project")
	}

	// Return appropriate embedded content based on compiler configuration
	if TinyGoCompiler {
		return embeddedWasmExecTinyGo, nil
	}
	return embeddedWasmExecGo, nil
}

// JavascriptForInitializing returns the JavaScript code needed to initialize WASM.
//
// Parameters (variadic):
//   - customizations[0]: Custom header string to prepend to wasm_exec.js content.
//     If not provided, defaults to "// TinyWasm: mode=<current_mode>\n"
//   - customizations[1]: Custom footer string to append after wasm_exec.js content.
//     If not provided, defaults to WebAssembly initialization code with fetch and instantiate.
//
// Examples:
//   - JavascriptForInitializing() - Uses default header and footer
//   - JavascriptForInitializing("// Custom Header\n") - Custom header, default footer
//   - JavascriptForInitializing("// Custom Header\n", "console.log('loaded');") - Both custom
func (h *TinyWasm) JavascriptForInitializing(customizations ...string) (js string, err error) {
	// Load wasm js code
	wasmType, _ := h.WasmProjectTinyGoJsUse()
	if !wasmType {
		return
	}

	// Determine current mode shortcut and pick the right cache variable.
	mode := h.Value()

	// Return appropriate cached content if available for each explicit mode.
	// Coding mode -> modeC_go_wasm_exec_cache
	// Debugging mode -> modeD_tinygo_wasm_exec_cache
	// Production mode -> modeP_tinygo_wasm_exec_cache
	if mode == h.Config.BuildFastShortcut && h.modeC_go_wasm_exec_cache != "" {
		return h.modeC_go_wasm_exec_cache, nil
	}
	if mode == h.Config.BuildBugShortcut && h.modeD_tinygo_wasm_exec_cache != "" {
		return h.modeD_tinygo_wasm_exec_cache, nil
	}
	if mode == h.Config.BuildMinimalShortcut && h.modeP_tinygo_wasm_exec_cache != "" {
		return h.modeP_tinygo_wasm_exec_cache, nil
	}

	// Get raw content from embedded assets instead of system paths
	wasmJs, err := h.getWasmExecContent()
	if err != nil {
		return "", err
	}

	stringWasmJs := string(wasmJs)

	// Determine header: use custom if provided, otherwise default
	var header string
	if len(customizations) > 0 {
		header = customizations[0]
	} else {
		// Default header: minimal comment with current mode for detection
		currentModeAtGeneration := h.Value()
		header = fmt.Sprintf("// TinyWasm: mode=%s\n", currentModeAtGeneration)
	}

	stringWasmJs = header + stringWasmJs

	// Verify activeBuilder is initialized before accessing it
	if h.activeBuilder == nil {
		return "", Errf("activeBuilder not initialized")
	}

	// Determine footer: use custom if provided, otherwise default
	var footer string
	if len(customizations) > 1 {
		footer = customizations[1]
	} else {
		// Default footer: WebAssembly initialization code
		footer = `
		const go = new Go();
		WebAssembly.instantiateStreaming(fetch("` + h.activeBuilder.MainOutputFileNameWithExtension() + `"), go.importObject).then((result) => {
			go.run(result.instance);
		});
	`
	}
	stringWasmJs += footer

	// Normalize JS output to avoid accidental differences between cached and
	// freshly-generated content (line endings, trailing spaces).
	normalized := normalizeJs(stringWasmJs)

	// Store in appropriate cache based on mode
	switch mode {
	case h.Config.BuildFastShortcut:
		h.modeC_go_wasm_exec_cache = normalized
	case h.Config.BuildBugShortcut:
		h.modeD_tinygo_wasm_exec_cache = normalized
	case h.Config.BuildMinimalShortcut:
		h.modeP_tinygo_wasm_exec_cache = normalized
	default:
		// Fallback: if TinyGo compiler in use write to tinyGo cache, otherwise go cache
		if h.tinyGoCompiler {
			h.modeD_tinygo_wasm_exec_cache = normalized
		} else {
			h.modeC_go_wasm_exec_cache = normalized
		}
	}

	return normalized, nil
}

// normalizeJs applies deterministic normalization to JS content so cached
// and regenerated outputs are identical: convert CRLF to LF and trim trailing
// whitespace from each line.
func normalizeJs(s string) string {
	// Normalize CRLF -> LF
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// Trim trailing whitespace on each line
	lines := strings.Split(s, "\n")
	for i, L := range lines {
		lines[i] = strings.TrimRight(L, " \t")
	}
	return strings.Join(lines, "\n")
}

// ClearJavaScriptCache clears both cached JavaScript strings to force regeneration
func (h *TinyWasm) ClearJavaScriptCache() {
	h.modeC_go_wasm_exec_cache = ""
	h.modeD_tinygo_wasm_exec_cache = ""
	h.modeP_tinygo_wasm_exec_cache = ""
}

// GetWasmExecJsPathTinyGo returns the path to TinyGo's wasm_exec.js file
func (w *TinyWasm) GetWasmExecJsPathTinyGo() (string, error) {
	// Method 1: Try standard lib location pattern
	libPaths := []string{
		"/usr/local/lib/tinygo/targets/wasm_exec.js",
		"/opt/tinygo/targets/wasm_exec.js",
	}

	for _, path := range libPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Method 2: Derive from tinygo executable path
	tinygoPath, err := exec.LookPath("tinygo")
	if err != nil {
		return "", Errf("tinygo executable not found: %v", err)
	}

	// Get directory where tinygo is located
	tinyGoDir := filepath.Dir(tinygoPath)

	// Common installation patterns
	patterns := []string{
		// For /usr/local/bin/tinygo -> /usr/local/lib/tinygo/targets/wasm_exec.js
		filepath.Join(filepath.Dir(tinyGoDir), "lib", "tinygo", "targets", "wasm_exec.js"),
		// For /usr/bin/tinygo -> /usr/lib/tinygo/targets/wasm_exec.js
		filepath.Join(filepath.Dir(tinyGoDir), "lib", "tinygo", "targets", "wasm_exec.js"),
		// For portable installation: remove bin and add targets
		filepath.Join(filepath.Dir(tinyGoDir), "targets", "wasm_exec.js"),
	}

	for _, wasmExecPath := range patterns {
		if _, err := os.Stat(wasmExecPath); err == nil {
			return wasmExecPath, nil
		}
	}

	return "", Errf("TinyGo wasm_exec.js not found. Searched paths: %v", append(libPaths, patterns...))
}

// GetWasmExecJsPathGo returns the path to Go's wasm_exec.js file
func (w *TinyWasm) GetWasmExecJsPathGo() (string, error) {
	// Method 1: Try GOROOT environment variable (most reliable)
	goRoot := os.Getenv("GOROOT")
	if goRoot != "" {
		patterns := []string{
			filepath.Join(goRoot, "misc", "wasm", "wasm_exec.js"), // Traditional location
			filepath.Join(goRoot, "lib", "wasm", "wasm_exec.js"),  // Modern location
		}
		for _, wasmExecPath := range patterns {
			if _, err := os.Stat(wasmExecPath); err == nil {
				return wasmExecPath, nil
			}
		}
	}

	// Method 2: Derive from go executable path
	goPath, err := exec.LookPath("go")
	if err != nil {
		return "", Errf("go executable not found: %v", err)
	}

	// Get installation directory (parent of bin directory)
	goDir := filepath.Dir(goPath)

	// Remove bin directory from path (cross-platform)
	if filepath.Base(goDir) == "bin" {
		goDir = filepath.Dir(goDir)
	}

	// Try multiple patterns for different Go versions
	patterns := []string{
		filepath.Join(goDir, "misc", "wasm", "wasm_exec.js"), // Traditional location
		filepath.Join(goDir, "lib", "wasm", "wasm_exec.js"),  // Modern location (Go 1.21+)
	}

	for _, wasmExecPath := range patterns {
		if _, err := os.Stat(wasmExecPath); err == nil {
			return wasmExecPath, nil
		}
	}

	return "", Errf("go wasm_exec.js not found. Searched: GOROOT=%s, patterns=%v", goRoot, patterns)
}

// getModeFromWasmExecJsHeader extracts the mode shortcut from a wasm_exec.js
// header comment emitted by JavascriptForInitializing. The header format is
// expected to be: "// TinyWasm: <message>" where <message> is the success
// message returned by getSuccessMessage. We return the matching shortcut and true
// when a match is found.
func (h *TinyWasm) getModeFromWasmExecJsHeader(content string) (string, bool) {
	const prefix = "// TinyWasm: "

	// Only check start of file for header
	firstLine := content
	if idx := strings.Index(content, "\n"); idx != -1 {
		firstLine = content[:idx]
	}

	if !strings.HasPrefix(firstLine, prefix) {
		return "", false
	}

	rest := strings.TrimSpace(strings.TrimPrefix(firstLine, prefix))

	// look for mode=<shortcut> in rest
	lower := strings.ToLower(rest)
	if mIdx := strings.Index(lower, "mode="); mIdx != -1 {
		after := rest[mIdx+len("mode="):]
		// stop at semicolon or whitespace
		end := strings.IndexAny(after, "; \t")
		var modeVal string
		if end == -1 {
			modeVal = strings.TrimSpace(after)
		} else {
			modeVal = strings.TrimSpace(after[:end])
		}
		if modeVal != "" {
			return modeVal, true
		}
	}

	return "", false
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

	outputPath := w.WasmExecJsOutputPath()

	w.Logger("DEBUG: Writing/overwriting wasm_exec.js to output path:", outputPath)

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		w.Logger("Failed to create output directory:", err)
		return // We did attempt the operation (project), but treat errors as non-fatal
	}

	// Get the complete JavaScript initialization code (includes WASM setup)
	jsContent, err := w.JavascriptForInitializing()
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
			w.currentMode = w.Config.BuildBugShortcut
		} else {
			w.activeBuilder = w.builderCoding
			w.currentMode = w.Config.BuildFastShortcut
		}
		//w.Logger("DEBUG: Using signature-based default mode:", w.currentMode)
	}

	return true

	//w.Logger("DEBUG: No valid WASM signatures found in wasm_exec.js")
}

// detectFromExistingWasmExecJs checks for existing wasm_exec.js file
func (w *TinyWasm) detectFromExistingWasmExecJs() bool {
	wasmExecPath := w.WasmExecJsOutputPath()

	// Check if file exists
	if _, err := os.Stat(wasmExecPath); err != nil {
		return false
	}

	// Analyze content to determine compiler type
	return w.analyzeWasmExecJsContent(wasmExecPath)
}
