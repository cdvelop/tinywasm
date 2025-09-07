# Refactor: Centralized WASM Compiler Detection & wasm_exec.js Management

## Executive Summary

This refactoring proposal aims to simplify and centralize WASM project detection by:

1. **Removing dynamic detection function pointers** - Eliminate `wasmDetectionFuncFromGoFile` and `wasmDetectionFuncFromJsFile` fields
2. **Single-time detection during initialization** - Perform all detection logic once at the end of `New()` constructor
3. **Centralized wasm_exec.js management** - Ensure `wasm_exec.js` files are properly managed and updated in the output directory
4. **Simplified NewFileEvent** - Focus only on compilation triggers for `.go` files with "write"/"create" events

## Current State Analysis

### Current Detection Mechanism
- **Function Pointers**: Two dynamic function pointers that switch between "active" and "inactive" states
  - `wasmDetectionFuncFromGoFile func(string, string)` - detects `.wasm.go` files
  - `wasmDetectionFuncFromJsFile func(fileName, extension, filePath, event string)` - analyzes existing `wasm_exec.js` signatures
- **Runtime Switching**: Functions become no-ops after first successful detection to prevent reconfiguration
- **Event-Driven**: Detection triggered during file events in `NewFileEvent()`

### Current wasm_exec.js Handling
- **Source Location Detection**: Complex logic to find Go/TinyGo installation paths
- **Signature Analysis**: Pattern matching in existing JS files to determine compiler type
- **Cache Management**: In-memory caching with manual cache clearing
- **No Output Management**: Missing automatic copying/updating of `wasm_exec.js` to output directory

### Integration Points
- **DevWatch Integration**: `NewFileEvent()` called for all file changes
- **Change.go Integration**: Mode switching triggers recompilation but doesn't update `wasm_exec.js`
- **AssetMin Integration**: Relies on `JavascriptForInitializing()` for JS content

## Updated Architecture (Final Implementation)

### 1. Initialization-Time Detection (Final)

```go
func New(c *Config) *TinyWasm {
    // ... existing initialization ...
    
    // Set default for WebFilesSubRelativeJsOutput if not configured
    if c.WebFilesSubRelativeJsOutput == "" {
        c.WebFilesSubRelativeJsOutput = "theme/js"
    }
    
    // Perform one-time detection at the end
    w.detectProjectConfiguration()
    
    return w
}

func (w *TinyWasm) detectProjectConfiguration() {
    // Priority 1: Check for existing wasm_exec.js (definitive source)
    if w.detectFromExistingWasmExecJs() {
        w.Logger("WASM project detected from existing wasm_exec.js")
        return
    }
    
    // Priority 2: Check for .wasm.go files (confirms WASM project)
    if w.detectFromGoFiles() {
        w.Logger("WASM project detected from .wasm.go files, defaulting to Go compiler")
        w.wasmProject = true
        w.tinyGoCompiler = false
        w.currentMode = w.Config.CodingShortcut
        return
    }
    
    w.Logger("No WASM project detected")
}
```

### 2. Simplified File Event Handling (Final)

```go
func (w *TinyWasm) NewFileEvent(fileName, extension, filePath, event string) error {
    // Only process Go files for compilation triggers
    if extension != ".go" {
        return nil
    }
    
    // Only process write/create events
    if event != "write" && event != "create" {
        return nil
    }
    
    // Check if this file should trigger compilation
    if !w.ShouldCompileToWasm(fileName, filePath) {
        return nil
    }
    
    // Compile using current active builder
    if w.activeBuilder == nil {
        return Err("builder not initialized")
    }
    
    return w.activeBuilder.CompileProgram()
}
```

### 3. wasm_exec.js File Management (Final)

```go
func (w *TinyWasm) getWasmExecJsOutputPath() string {
    return path.Join(w.Config.AppRootDir, w.Config.WebFilesRootRelative, w.Config.WebFilesSubRelativeJsOutput, "wasm_exec.js")
}

func (w *TinyWasm) detectFromExistingWasmExecJs() bool {
    wasmExecPath := w.getWasmExecJsOutputPath()
    
    // Check if file exists
    if _, err := os.Stat(wasmExecPath); err != nil {
        return false
    }
    
    // Analyze content to determine compiler type
    return w.analyzeWasmExecJsContent(wasmExecPath)
}

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
        w.Logger("Detected TinyGo compiler from wasm_exec.js")
        return true
    } else if goCount > tinyCount && goCount > 0 {
        w.tinyGoCompiler = false
        w.wasmProject = true
        w.Logger("Detected Go compiler from wasm_exec.js")
        return true
    } else if tinyCount > 0 || goCount > 0 {
        // Single-sided detection
        w.tinyGoCompiler = tinyCount > 0
        w.wasmProject = true
        compiler := map[bool]string{true: "TinyGo", false: "Go"}[w.tinyGoCompiler]
        w.Logger("Detected WASM project, compiler:", compiler)
        return true
    }
    
    w.Logger("No valid WASM signatures found in wasm_exec.js")
    return false
}

// ensureWasmExecJsInOutput creates wasm_exec.js in output directory ONLY if it doesn't exist
// This respects existing user customizations and creates only when needed
func (w *TinyWasm) ensureWasmExecJsInOutput() error {
    outputPath := w.getWasmExecJsOutputPath()
    
    // Check if file already exists - do not overwrite
    if _, err := os.Stat(outputPath); err == nil {
        w.Logger("wasm_exec.js already exists, skipping creation")
        return nil
    }
    
    // Create output directory if it doesn't exist
    outputDir := filepath.Dir(outputPath)
    if err := os.MkdirAll(outputDir, 0755); err != nil {
        w.Logger("Failed to create output directory:", err)
        return nil // Non-fatal, just log
    }
    
    // Get source path based on current mode
    var sourcePath string
    var err error
    if w.tinyGoCompiler {
        sourcePath, err = w.GetWasmExecJsPathTinyGo()
    } else {
        sourcePath, err = w.GetWasmExecJsPathGo()
    }
    
    if err != nil {
        w.Logger("Failed to locate wasm_exec.js source:", err)
        return nil // Non-fatal, just log
    }
    
    // Copy file to output location
    if err := w.copyFile(sourcePath, outputPath); err != nil {
        w.Logger("Failed to copy wasm_exec.js:", err)
        return nil // Non-fatal, just log
    }
    
    w.Logger("Created wasm_exec.js in output directory")
    return nil
}

func (w *TinyWasm) copyFile(src, dst string) error {
    sourceData, err := os.ReadFile(src)
    if err != nil {
        return err
    }
    
    return os.WriteFile(dst, sourceData, 0644)
}
```

### 4. Enhanced Change.go Integration (Final)

```go
func (w *TinyWasm) Change(newValue string, progress func(msgs ...any)) {
    // ... existing validation and builder update ...
    
    // Update active builder
    w.updateCurrentBuilder(newValue)
    
    // Ensure wasm_exec.js is available before compilation (only if .wasm.go files exist)
    if w.wasmProject {
        if err := w.ensureWasmExecJsInOutput(); err != nil {
            // Error already logged in ensureWasmExecJsInOutput, continue execution
        }
        
        // Clear JavaScript cache to force regeneration with new mode
        w.ClearJavaScriptCache()
    }
    
    // Check if main WASM file exists before attempting compilation
    rootFolder := path.Join(w.AppRootDir, w.Config.WebFilesRootRelative)
    mainWasmPath := path.Join(rootFolder, w.mainInputFile)
    if _, err := os.Stat(mainWasmPath); err != nil {
        progress(w.getSuccessMessage(newValue))
        return
    }
    
    // ... existing compilation logic ...
}
```

### 5. JavascriptForInitializing() - NO CHANGES ✅

The existing `JavascriptForInitializing()` method already works correctly:
- Reads from Go/TinyGo installation paths
- Adds WASM startup code
- Generates content for output to `WebFilesSubRelativeJsOutput/wasm_exec.js`
- Uses existing cache mechanism

**No modifications needed** - this function is perfect as-is.

## Implementation Plan (Final)

### Phase 1: Core Structure Refactoring ✅
1. **Remove function pointer fields** from `TinyWasm` struct:
   - Remove `wasmDetectionFuncFromGoFile func(string, string)`
   - Remove `wasmDetectionFuncFromJsFile func(fileName, extension, filePath, event string)`
2. **Add default configuration** in `New()`: Set `WebFilesSubRelativeJsOutput = "theme/js"` if empty
3. **Create initialization detection methods** reusing logic from `wasmDetectionFuncFromJsFileActive`
4. **Update New() constructor** to call detection at the end
5. **Add file management utilities** for wasm_exec.js (non-overwriting)

### Phase 2: Event Handling & File Management ✅  
1. **Simplify NewFileEvent()** to handle only Go file compilation triggers
2. **Remove all JS event handling** from NewFileEvent (no longer needed)
3. **Update Change.go** to call `ensureWasmExecJsInOutput()` before compilation
4. **Implement non-overwriting file creation** (respect existing customizations)

### Phase 3: Testing & Validation ✅
1. **Remove obsolete tests**:
   - `js_file_event_test.go` (event-driven detection no longer used)
   - Any other event-driven detection tests
2. **Create new initialization tests**:
   - Test detection from existing `wasm_exec.js` files
   - Test detection from `.wasm.go` files
   - Test default configuration handling
3. **Add file management tests**:
   - Test `ensureWasmExecJsInOutput()` creates files when needed
   - Test it doesn't overwrite existing files
   - Test Change.go integration

### Phase 4: Cleanup & Documentation ✅
1. **Remove unused code**:
   - `wasmDetectionFuncFromJsFileActive` and `wasmDetectionFuncFromJsFileInactive`
   - `wasmDetectionFuncFromGoFileActive` and `wasmDetectionFuncFromGoFileInactive`
   - Related function pointer initialization in `New()`
2. **Keep JavascriptForInitializing() unchanged** (already works correctly)
3. **Update documentation** for new initialization-based approach

### Key Changes Summary:
- **Struct Fields**: Remove 2 function pointer fields
- **New()**: Add default config + call detection once
- **NewFileEvent()**: Simplify to Go-only compilation
- **Change.go**: Add wasm_exec.js management before compilation
- **Tests**: Complete restructuring focused on initialization
- **JavascriptForInitializing()**: NO CHANGES (works as-is)

## Decisions Made (Final)

### 1. **Output Directory Strategy - RESOLVED** ✅
- **Decision**: `WebFilesSubRelativeJsOutput` is the **single source of truth** for wasm_exec.js location
- **Default Configuration**: If `WebFilesSubRelativeJsOutput` is empty → default to `"theme/js"` within `WebFilesRootRelative`
- **Rationale**: Avoids conflicts with minified output in `WebFilesSubRelative` and provides clear separation
- **Implementation**: All detection and file management will use this path exclusively

### 2. **Detection Priority - RESOLVED** ✅
- **Decision**: Hierarchical detection approach:
  1. **Primary**: Check for existing `wasm_exec.js` in `WebFilesSubRelativeJsOutput` → determines both project type AND compiler type
  2. **Secondary**: If no `wasm_exec.js`, check for `.wasm.go` files → confirms WASM project, defaults to Go compiler
  3. **Fallback**: If neither exists → not a WASM project
- **Rationale**: Existing `wasm_exec.js` provides the most accurate current configuration

### 3. **JavascriptForInitializing() Function - CLARIFIED** ✅
- **Current Behavior**: Creates JavaScript output for `WebFilesSubRelativeJsOutput/wasm_exec.js` by adding WASM startup code
- **Decision**: **NO CHANGES NEEDED** to this function - it already works correctly
- **Rationale**: Function already generates the correct output, just need to ensure the source file management works properly

### 4. **File Update Strategy - RESOLVED** ✅
- **Decision**: Update `wasm_exec.js` in `Change.go` **before compilation**, only if required by existing `*.wasm.go` files
- **Behavior**: Create only if needed, **do not overwrite** if file already exists
- **Timing**: During mode changes in `Change.go`, before WASM compilation
- **Rationale**: Respect existing user customizations, create only when necessary

### 5. **Cache Strategy - RESOLVED** ✅
- **Decision**: **NO CHANGES** to existing cache mechanism
- **Rationale**: Current cache in `JavascriptForInitializing()` already works correctly for in-memory content generation
- **Implementation**: Keep existing `goWasmJsCache` and `tinyGoWasmJsCache` as-is

### 6. **Error Handling - RESOLVED** ✅
- **Decision**: Non-fatal error handling with logging only
- **Implementation**: All detection and file management failures will use `Logger()` only, no fatal errors
- **Rationale**: Graceful degradation, project continues to work even if file operations fail

### 7. **Code Maintenance - RESOLVED** ✅
- **Decision**: Remove all obsolete code, keep only what serves the new approach
- **Scope**: Eliminate function pointers, event-driven detection, and related tests
- **Rationale**: Clean architecture, no legacy code maintenance burden

## All Questions Resolved ✅

### ~~1. JavascriptForInitializing() Integration~~ - CLARIFIED ✅
- **Resolution**: NO CHANGES needed to `JavascriptForInitializing()`
- **Understanding**: Function creates JS output for `WebFilesSubRelativeJsOutput/wasm_exec.js` by adding WASM startup code
- **Current behavior is correct**: Reads from installation paths, adds startup code, uses existing cache

### ~~2. Default Configuration Handling~~ - RESOLVED ✅
- **Resolution**: Default to `"theme/js"` if `WebFilesSubRelativeJsOutput` is empty
- **Implementation**: Set in `New()` constructor before detection

### ~~3. File Update Strategy~~ - RESOLVED ✅
- **Resolution**: Update in `Change.go` before compilation, create only if needed (don't overwrite existing)
- **Behavior**: Respect existing user customizations, create only when `*.wasm.go` files exist

### ~~4. Cache Strategy Alignment~~ - RESOLVED ✅
- **Resolution**: NO CHANGES to existing cache mechanism
- **Rationale**: Current cache works correctly for in-memory content generation

### ~~5. Error Recovery Strategy~~ - RESOLVED ✅
- **Resolution**: Non-fatal logging only, graceful degradation
- **Implementation**: All file operations failures use `Logger()`, continue execution

### No Remaining Questions - Ready for Implementation ✅

## Risks & Mitigation (Updated)

### High Risk - RESOLVED ✅
- **Breaking Changes**: Existing code depending on detection timing
  - *Resolution*: Clean break approach approved, no backward compatibility needed
- **File System Operations**: Race conditions in wasm_exec.js management
  - *Mitigation*: Atomic file operations (write to temp, then rename)

### Medium Risk
- **Detection Accuracy**: New logic might miss edge cases from current system
  - *Mitigation*: Reuse proven detection logic from `wasmDetectionFuncFromJsFileActive`
  - *Mitigation*: Comprehensive test coverage with various project layouts
- **Configuration Complexity**: `WebFilesSubRelativeJsOutput` handling
  - *Mitigation*: Sensible defaults, clear validation messages
- **Performance Impact**: More I/O during initialization
  - *Mitigation*: Efficient file operations, minimal required checks only

### Low Risk
- **Cache Inconsistency**: In-memory cache vs output file mismatch
  - *Mitigation*: Clear cache when updating output files
- **Integration Disruption**: DevWatch/AssetMin workflow changes
  - *Mitigation*: Maintain existing integration APIs where possible

### New Risks Identified
- **Dependency on File System**: Output directory creation failures
  - *Mitigation*: Graceful fallback, detailed logging, directory creation with proper permissions
- **Source File Availability**: Go/TinyGo wasm_exec.js not found
  - *Mitigation*: Robust path detection, clear error messages, fallback to existing behavior if needed

## Success Criteria (Final)

1. **Functional Requirements** ✅:
   - ✅ WASM projects detected correctly during initialization (from existing wasm_exec.js or .wasm.go files)
   - ✅ `wasm_exec.js` created in `WebFilesSubRelativeJsOutput` only when needed (don't overwrite existing)
   - ✅ Mode switching in `Change.go` ensures `wasm_exec.js` availability before compilation
   - ✅ File events trigger compilation only for relevant Go files
   - ✅ `JavascriptForInitializing()` continues working unchanged

2. **Architecture Requirements** ✅:
   - ✅ Function pointer fields removed from struct
   - ✅ Event-driven detection eliminated
   - ✅ Single-time detection during initialization
   - ✅ Non-fatal error handling with logging only

3. **Performance Requirements** ✅:
   - ✅ Minimal initialization overhead (just file existence checks)
   - ✅ No runtime detection overhead after initialization
   - ✅ Existing cache mechanism preserved

4. **Quality Requirements** ✅:
   - ✅ Backward compatibility maintained (existing projects continue working)
   - ✅ Clean architecture with no obsolete code
   - ✅ Comprehensive test coverage for new detection approach

## Ready for Implementation ✅

**All decisions finalized, all questions resolved. Implementation can proceed immediately with:**

1. Remove function pointer fields from `TinyWasm` struct
2. Add initialization-time detection using existing logic  
3. Simplify `NewFileEvent()` to Go-only compilation
4. Update `Change.go` for file management
5. Refactor tests for new architecture
6. Keep `JavascriptForInitializing()` unchanged

**Estimated Implementation Time**: 1-2 days
**Risk Level**: Low (reusing existing proven logic)
**Breaking Changes**: None (clean internal refactoring)

## Migration Strategy

### For Existing Projects
1. **Automatic Migration**: Detection logic should work transparently
2. **Configuration Updates**: May need to set `WebFilesSubRelativeJsOutput` if custom layouts are used
3. **Cache Clearing**: Existing caches will be automatically updated

### For Integration Points
1. **DevWatch**: Will receive fewer detection calls, only compilation events
2. **AssetMin**: Should continue working without changes
3. **Tests**: Will need updates to reflect new detection timing

## Next Steps

1. **Review & Approval**: Get feedback on this proposal
2. **Prototype**: Create minimal implementation to validate approach
3. **Detailed Design**: Flesh out specific implementation details
4. **Implementation**: Execute the planned phases
5. **Testing**: Comprehensive testing across all integration points
6. **Documentation**: Update all relevant documentation and examples

---

**Filed by**: Development Team  
**Date**: 2025-09-07  
**Priority**: High  
**Estimated Effort**: 2-3 weeks  
**Dependencies**: None identified
