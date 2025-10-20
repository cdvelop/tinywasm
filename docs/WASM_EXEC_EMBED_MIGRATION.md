# WASM Exec Embed Migration Plan

## Executive Summary

**Objective**: Migrate responsibility for managing `wasm_exec.js` files from `goflare` to `tinywasm` by embedding static copies of both Go and TinyGo versions, enabling reliable offline operation and eliminating runtime system path dependencies.

**Key Changes**:
- ✅ **COMPLETED**: Modified `JavascriptForInitializing(customizations ...string)` to accept variadic parameters for custom header/footer
- Embed `wasm_exec_go.js` and `wasm_exec_tinygo.js` in `tinywasm/assets/`
- Replace system path logic with embedded file access directly in `JavascriptForInitializing()`
- Create public `GetWasmExecContent() ([]byte, error)` method that:
  - Returns raw embedded content based on compiler type
  - Provides reusable access for external packages (goflare, custom tools)
- Remove duplicate logic from `goflare` package
- Add comprehensive tests to ensure embedded files exist before compilation

**Note**: We no longer need to create a separate method for header/footer customization since `JavascriptForInitializing` now accepts variadic parameters.

---

## 1. Motivation

### Current Issues
- **Dual Responsibility**: Both `goflare` and `tinywasm` manage wasm_exec.js retrieval
- **System Dependencies**: Runtime lookups via `GetWasmExecJsPathGo()` and `GetWasmExecJsPathTinyGo()` can fail if Go/TinyGo locations change
- **Code Duplication**: Similar logic exists in both packages
- **No Offline Guarantee**: System paths must be accessible at runtime

### Benefits of Embedding
- **Single Source of Truth**: `tinywasm` owns wasm_exec management
- **Offline Operation**: Embedded files work without system dependencies
- **Version Stability**: Known versions embedded at build time
- **Simplified Testing**: Predictable file access
- **Better Distribution**: Package is self-contained

---

## 2. Architecture Design

### 2.1 File Structure

```
tinywasm/
├── assets/
│   ├── wasm_exec_go.js       # Embedded Go version
│   └── wasm_exec_tinygo.js   # Embedded TinyGo version
├── javascripts.go            # Contains GetWasmExecContent() and embed directives
├── wasm_exec_test.go         # Tests to populate assets/ and verify embed
└── docs/
    └── WASM_EXEC_EMBED_MIGRATION.md  # This document
```

### 2.2 Embed Strategy

**Implementation in `javascripts.go`**:
```go
package tinywasm

import (
    _ "embed"
    "fmt"
)

//go:embed assets/wasm_exec_go.js
var embeddedWasmExecGo []byte

//go:embed assets/wasm_exec_tinygo.js
var embeddedWasmExecTinyGo []byte
```

**Design Notes**:
- Use `//go:embed` with `[]byte` type for efficient memory usage
- Files must exist in `assets/` directory at build time
- Test suite ensures files are populated before any build attempts

---

## 3. Public API Specification

### 3.1 GetWasmExecContent() Method

**Signature**:
```go
func (w *TinyWasm) GetWasmExecContent() ([]byte, error)
```

**Purpose**:
Returns the raw `wasm_exec.js` content based on the TinyWasm instance's current compiler configuration. This method is designed to be reusable from external packages (like goflare) that need access to the appropriate wasm_exec.js file without managing embedded resources themselves.

**Behavior**:
- Returns raw `wasm_exec.js` content WITHOUT any header modifications or caching
- Determines compiler type (Go vs TinyGo) by calling `w.WasmProjectTinyGoJsUse()`
- Selects appropriate embedded file based on that determination
- Returns error if not a WASM project
- Uses embedded files exclusively, NOT system paths
- Stateless operation - no side effects on TinyWasm instance

**Implementation**:
```go
// GetWasmExecContent returns the raw wasm_exec.js content for the current compiler configuration.
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
func (w *TinyWasm) GetWasmExecContent() ([]byte, error) {
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
```

**Key Design Decisions**:
- ✅ **No parameters**: Uses TinyWasm's internal state via `WasmProjectTinyGoJsUse()`
- ✅ **Stateless**: Doesn't modify TinyWasm instance or cache state
- ✅ **Raw output**: No headers, no modifications - pure embedded content
- ✅ **Reusable**: Can be called multiple times safely
- ✅ **Public API**: Accessible from external packages (goflare, custom tools)
- ✅ **Embedded only**: Never falls back to system paths

---

## 4. Refactoring JavascriptForInitializing()

### 4.1 Current Implementation Status

✅ **COMPLETED**: The method signature has been updated to:
```go
func (h *TinyWasm) JavascriptForInitializing(customizations ...string) (js string, err error)
```

**Parameters (variadic)**:
- `customizations[0]`: Custom header (optional, defaults to `"// TinyWasm: mode=<mode>\n"`)
- `customizations[1]`: Custom footer (optional, defaults to WebAssembly initialization code)

### 4.2 Remaining Refactoring Tasks

**What Needs to Change**:
- **Lines 82-91** (system path determination and file reading) → Replace with embedded file access
- Replace:
  ```go
  var wasmExecJsPath string
  if TinyGoCompiler {
      wasmExecJsPath, err = h.GetWasmExecJsPathTinyGo()
  } else {
      wasmExecJsPath, err = h.GetWasmExecJsPathGo()
  }
  if err != nil {
      return "", err
  }

  // Read wasm js code
  wasmJs, err := os.ReadFile(wasmExecJsPath)
  ```

**With**:
  ```go
  // Get raw content from embedded assets
  wasmJs, err := h.GetWasmExecContent()
  if err != nil {
      return "", err
  }
  ```

**What Stays**:
- Cache checking logic (lines 57-75)
- Header/footer logic (lines 93-133, now with variadic parameter support)
- Normalization and caching logic (lines 135-152)

**Refactored Code Preview**:
```go
func (h *TinyWasm) JavascriptForInitializing(customizations ...string) (js string, err error) {
    // Load wasm js code
    wasmType, TinyGoCompiler := h.WasmProjectTinyGoJsUse()
    if !wasmType {
        return
    }

    // Determine current mode shortcut and pick the right cache variable
    mode := h.Value()

    // Return appropriate cached content if available
    if mode == h.Config.BuildLargeSizeShortcut && h.mode_large_go_wasm_exec_cache != "" {
        return h.mode_large_go_wasm_exec_cache, nil
    }
    if mode == h.Config.BuildMediumSizeShortcut && h.mode_medium_tinygo_wasm_exec_cache != "" {
        return h.mode_medium_tinygo_wasm_exec_cache, nil
    }
    if mode == h.Config.BuildSmallSizeShortcut && h.mode_small_tinygo_wasm_exec_cache != "" {
        return h.mode_small_tinygo_wasm_exec_cache, nil
    }

    // ✨ NEW: Get raw content from embedded assets instead of system paths
    wasmJs, err := h.GetWasmExecContent()
    if err != nil {
        return "", err
    }

    stringWasmJs := string(wasmJs)

    // Determine header: use custom if provided, otherwise default
    var header string
    if len(customizations) > 0 && customizations[0] != "" {
        header = customizations[0]
    } else {
        currentModeAtGeneration := h.Value()
        header = fmt.Sprintf("// TinyWasm: mode=%s\n", currentModeAtGeneration)
    }
    stringWasmJs = header + stringWasmJs

    // ... rest remains unchanged (activeBuilder check, footer, normalization, caching)
}
```

---

## 5. Test Requirements

### 5.1 Test Migration: wasm_exec_test.go

**Source**: `/home/cesar/Dev/Pkg/Mine/goflare/wasm_exec_test.go`  
**Destination**: `/home/cesar/Dev/Pkg/Mine/tinywasm/wasm_exec_test.go`

**Required Adaptations**:

1. **Change Package Declaration**:
   ```go
   package tinywasm  // was: package goflare
   ```

2. **Update Test Purpose**: Focus on ensuring `assets/` files exist

3. **Key Test Function**: `TestEnsureWasmExecFilesExists`
   - Verify `assets/wasm_exec_go.js` exists, create from system path if missing
   - Verify `assets/wasm_exec_tinygo.js` exists, create from system path if missing
   - Validate file integrity using hash comparison
   - Update files if system versions have changed

4. **New Test**: `TestGetWasmExecContent`
   - Verify `GetWasmExecContent()` returns non-empty content
   - Verify correct file selected based on compiler configuration
   - Verify returns error for non-WASM projects

5. **Helper Functions** (keep from original):
   - `copyWasmExecFile()`: Copy from system to assets
   - `getFileHash()`: MD5 hash for version detection
   - `ensureWasmExecFile()`: Main logic to populate assets

### 5.2 Test Execution Order

```bash
# Before first build, populate assets/
cd /home/cesar/Dev/Pkg/Mine/tinywasm
go test -v -run TestEnsureWasmExecFilesExists

# This creates:
# - assets/wasm_exec_go.js
# - assets/wasm_exec_tinygo.js

# Then normal builds work with embedded files
go build .
```

---

## 6. Goflare Migration

### 6.1 Remove Duplicate Code

**Files to Update in `/home/cesar/Dev/Pkg/Mine/goflare/`**:

1. **Remove `getWasmExecContent()` method** (current implementation)
2. **Replace usages** with calls to `tinywasm.GetWasmExecContent()`

**Example Refactor**:
```go
// OLD (goflare code)
content, err := g.getWasmExecContent(compilerType)

// NEW (use tinywasm)
content, err := g.tinywasmInstance.GetWasmExecContent()
```

### 6.2 Update Dependencies

Ensure `goflare` has access to the `tinywasm` instance where needed:
- Verify `goflare` structs have reference to `tinywasm.TinyWasm`
- Update any constructors/initializers accordingly

---

## 7. Documentation Updates

### 7.1 Files to Update

1. **README.md**: 
   - Add section about embedded wasm_exec files
   - Explain test requirements before first build

2. **ISSUE_CHANGE_COMPILER.md**: 
   - Reference new `GetWasmExecContent()` API
   - Update examples using system paths

3. **ISSUE_BUG_MODES_COMPILATIONS.md**: 
   - Note that embedded files ensure consistent behavior across modes

4. **New Section in Contributing Guide**:
   - Explain how to update embedded wasm_exec files
   - Document procedure for new Go/TinyGo versions

### 7.2 API Documentation

Add to package-level documentation:

```go
// Embedded Assets
//
// TinyWasm embeds wasm_exec.js files for both Go and TinyGo compilers in the
// assets/ directory. These files are embedded at build time using go:embed.
//
// To update embedded files:
//   1. Run: go test -v -run TestEnsureWasmExecFilesExists
//   2. This copies latest versions from your Go/TinyGo installations
//   3. Commit updated assets/ files to version control
//
// The GetWasmExecContent() method provides access to these embedded files
// without requiring runtime system path lookups.
```

---

## 8. Implementation Checklist

### Phase 1: Setup and Testing (tinywasm)
- [ ] Create `tinywasm/assets/` directory
- [ ] Move `wasm_exec_test.go` from goflare to tinywasm
- [ ] Update package declaration to `package tinywasm`
- [ ] Run test to populate `assets/wasm_exec_go.js` and `assets/wasm_exec_tinygo.js`
- [ ] Verify files exist and are valid JavaScript

### Phase 2: Embed Implementation (tinywasm)
- [ ] Add `//go:embed` directives in `javascripts.go`
- [ ] Add package-level variables for embedded content
- [ ] Implement `GetWasmExecContent()` method
- [ ] Add test case `TestGetWasmExecContent()`

### Phase 3: Refactor (tinywasm)
- [ ] Refactor `JavascriptForInitializing()`:
  - [ ] Remove lines 42-44 (WASM type check - moved to `GetWasmExecContent()`)
  - [ ] Keep lines 47-65 (cache checking logic - mode-specific caching)
  - [ ] Remove lines 67-78 (system path determination and file reading)
  - [ ] Add call to `GetWasmExecContent()` after cache checks
  - [ ] Keep lines 83-128 (header injection, normalization, caching)
- [ ] Verify all existing tests still pass

### Phase 4: Goflare Migration
- [ ] Locate all usages of `getWasmExecContent()` in goflare
- [ ] Replace with calls to `tinywasm.GetWasmExecContent()`
- [ ] Remove old `getWasmExecContent()` method from goflare
- [ ] Update goflare tests if needed
- [ ] Verify goflare builds and tests pass

### Phase 5: Documentation
- [ ] Update README.md with embedded assets section
- [ ] Update ISSUE_CHANGE_COMPILER.md with new API
- [ ] Update ISSUE_BUG_MODES_COMPILATIONS.md with stability notes
- [ ] Add API documentation comments
- [ ] Update CONTRIBUTING.md with asset update procedure

### Phase 6: Validation
- [ ] Run full test suite in tinywasm: `go test -v ./...`
- [ ] Run full test suite in goflare: `go test -v ./...`
- [ ] Test mode switching (c → d → p)
- [ ] Verify offline operation (without Go/TinyGo in PATH)
- [ ] Verify build works on fresh clone (embedded assets included)

---

## 9. Risk Assessment

### Low Risk
- ✅ Embed is stable Go feature (since 1.16)
- ✅ Files are static, don't change during execution
- ✅ Comprehensive test coverage ensures files exist

### Medium Risk
- ⚠️ **Version Staleness**: Embedded files may become outdated
  - **Mitigation**: Document update procedure, add CI check
- ⚠️ **File Size**: Two JS files increase binary size (~50-100KB each)
  - **Mitigation**: Acceptable tradeoff for reliability

### Considerations
- Test suite must run before first build on new systems
- Document how to update embedded files when Go/TinyGo versions change
- Consider adding version comment in embedded files for tracking

---

## 10. Success Criteria

✅ **Functional Requirements**:
- `GetWasmExecContent()` returns correct content based on compiler
- `JavascriptForInitializing()` works identically to current behavior
- All three modes (c, d, p) compile successfully
- Goflare successfully uses tinywasm's `GetWasmExecContent()`

✅ **Quality Requirements**:
- All existing tests pass
- New tests added for `GetWasmExecContent()`
- Code coverage maintained or improved
- No breaking changes to public API (only additions)

✅ **Documentation Requirements**:
- All docs updated with embed approach
- API documentation complete
- Update procedure documented
- Examples updated

---

## 11. Timeline Estimate

1. **Phase 1-2** (Setup + Embed): 30 minutes
2. **Phase 3** (Refactor tinywasm): 20 minutes  
3. **Phase 4** (Goflare migration): 30 minutes
4. **Phase 5** (Documentation): 30 minutes
5. **Phase 6** (Testing & validation): 30 minutes

**Total**: ~2.5 hours (excluding review time)

---

## 12. Rollback Plan

If issues arise:

1. **Revert commit** with embed changes
2. **Restore original** `JavascriptForInitializing()` logic
3. **Keep test file** in tinywasm for future attempts
4. **Document blockers** in new issue

The migration is designed to be atomic - either fully complete or fully reverted.

---

## Appendix A: File Locations Reference

```
Source Files:
  goflare/wasm_exec_test.go → tinywasm/wasm_exec_test.go

New/Modified Files:
  tinywasm/assets/wasm_exec_go.js         [NEW - populated by test]
  tinywasm/assets/wasm_exec_tinygo.js     [NEW - populated by test]
  tinywasm/javascripts.go                 [MODIFIED - add embed + method]
  tinywasm/wasm_exec_test.go              [NEW - migrated + adapted]
  goflare/[files with getWasmExecContent] [MODIFIED - use tinywasm]
  
Documentation:
  tinywasm/docs/WASM_EXEC_EMBED_MIGRATION.md [NEW - this document]
  tinywasm/docs/ISSUE_CHANGE_COMPILER.md     [MODIFIED]
  tinywasm/docs/ISSUE_BUG_MODES_COMPILATIONS.md [MODIFIED]
  tinywasm/README.md                         [MODIFIED]
```

---

## Appendix B: Code Examples

### Example: Using GetWasmExecContent() in External Packages

```go
package mypackage

import "github.com/cdvelop/tinywasm"

func generateWorkerFile() error {
    tw := tinywasm.New(&tinywasm.Config{
        AppRootDir: "/my/project",
    })
    
    // GetWasmExecContent() uses TinyWasm's internal state to determine
    // which compiler (Go vs TinyGo) is being used and returns the
    // appropriate embedded wasm_exec.js content
    content, err := tw.GetWasmExecContent()
    if err != nil {
        return err
    }
    
    // Use content (e.g., write to file, embed in worker, etc.)
    return os.WriteFile("output/wasm_exec.js", content, 0644)
}
```

**Note**: `GetWasmExecContent()` determines the correct file based on the TinyWasm instance's configuration and current compiler state, accessed via `WasmProjectTinyGoJsUse()` method.

---

**Document Version**: 1.0  
**Created**: 2025-10-09  
**Author**: TinyWasm Development Team  
**Status**: 🔴 AWAITING REVIEW
