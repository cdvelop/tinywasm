Here is a refactor plan for `tinywasm` to make its input/output architecture fully compatible with the project structure described in your README.md. This plan is written in English and will be saved as `tinywasm/docs/REFACTOR_IN_OUT_FILES.md`. No code changes will be made until you review and approve.

---

# Refactor Plan: Input/Output Architecture for `tinywasm`

## Goal

Align `tinywasm` input and output file/directory handling with the Go project structure described in the main README, specifically:

- **Source (input):** `src/cmd/webclient/main.go`
- **WASM Output:** `src/web/public/main.wasm`
- **Watchable JS Output:** `src/web/ui/js/wasm_exec.js` (for file watcher integration)

## Current Issues

- `tinywasm` currently assumes output directories are relative to the input source directory (e.g., `src/cmd/webclient/public/`), which does not match the recommended architecture.
- The field `WebFilesSubRelativeJsOutput` creates ambiguity because it serves a different purpose than the main WASM output directory.
- This causes confusion and breaks convention with the rest of the project, where all frontend assets are in `src/web/public/`.

## Understanding TinyWasm's Dual Output Strategy

TinyWasm produces **two types of outputs** that serve different purposes:

### 1. **WASM Binary Output** (`OutputDir`)
- **Location:** `src/web/public/main.wasm`
- **Purpose:** Final compiled WebAssembly binary
- **Consumed by:** Browser at runtime
- **Modes:** All three compilation modes produce output here
  - `f` (fast) - Go standard compiler
  - `b` (bugs) - TinyGo with debug symbols
  - `m` (minimal) - TinyGo optimized for size

### 2. **Watchable JavaScript Output** (`WasmExecJsOutputDir`)
- **Location:** `src/web/ui/js/wasm_exec.js`
- **Purpose:** Mode-specific JavaScript runtime that:
  - Informs external tools about the current compilation mode (Go vs TinyGo)
  - Triggers file watcher to reload the browser when compilation mode changes
  - Gets compiled together with other JavaScript files by external asset bundlers
- **Consumed by:** 
  - File watchers (e.g., `devwatch`) for change detection
  - Asset bundlers (e.g., `assetmin`) for final compilation into `src/web/public/main.js`
- **Important:** TinyWasm's **only responsibility** is to write the correct `wasm_exec.js` file according to the active compilation mode. External tools handle bundling and final output.

### Why Two Separate Directories?

1. **Separation of Concerns:**
   - `OutputDir` → Final runtime assets (WASM binary)
   - `WasmExecJsOutputDir` → Development/build-time assets (JavaScript runtime)

2. **Build Pipeline Integration:**
   - Changes to `wasm_exec.js` in `WasmExecJsOutputDir` trigger file watchers
   - Asset bundlers collect JavaScript from `src/web/ui/` and compile into `src/web/public/main.js`
   - WASM binary stays in `OutputDir` for direct browser loading

3. **No Development vs Production States:**
   - TinyWasm operates in **compilation modes only** (fast/debug/minimal)
   - All modes write to the same directories
   - External tools handle environment-specific optimizations

## Refactor Steps

1. **Config API Redesign**
   - Change the config struct to accept:
     - `SourceDir`: Directory containing the Go source for the webclient (e.g., `src/cmd/webclient`)
     - `OutputDir`: Directory for WASM binary output (e.g., `src/web/public`)
     - `WasmExecJsOutputDir`: Directory for watchable JavaScript runtime (e.g., `src/web/ui/js`)
   - **Rename:** `WebFilesSubRelativeJsOutput` → `WasmExecJsOutputDir`
   - Remove any logic that assumes output is a subdirectory of the source.

2. **Compilation Logic**
   - When compiling, always use `SourceDir/main.go` as the entry point.
   - Output the WASM binary directly into `OutputDir` as `main.wasm`.
   - Output `wasm_exec.js` into `WasmExecJsOutputDir` (mode-aware: Go vs TinyGo).

3. **Asset Handling**
   - `wasm_exec.js` should **always** be placed in `WasmExecJsOutputDir`, regardless of compilation mode.
   - Do not place JavaScript files in `OutputDir` (that's handled by external bundlers).
   - Do not place assets in the source directory.

4. **API Usage Example**
   ```go
   tinywasm.New(&tinywasm.Config{
       AppRootDir:       "/path/to/project",
       SourceDir:        "src/cmd/webclient",
       OutputDir:        "src/web/public",
       WasmExecJsOutputDir: "src/web/ui/js",
       MainInputFile:    "main.wasm.go",
       OutputName:       "main",
       Logger:           logger,
   })
   ```

5. **Migration Guide**
   - Document the breaking change in the README and migration notes.
   - **Old field:** `WebFilesSubRelativeJsOutput` → **New field:** `WasmExecJsOutputDir`
   - Provide a simple upgrade path for existing users.
   - Clarify that `WasmExecJsOutputDir` is not for final output, but for build-time integration.

6. **Tests & Validation**
   - Update all tests to use the new directory structure.
   - Validate that WASM output files are always placed in `OutputDir`.
   - Validate that `wasm_exec.js` is always placed in `WasmExecJsOutputDir`.
   - Ensure all three compilation modes work correctly with the new structure.

7. **Documentation**
   - Update all code comments, README, and usage examples to reflect the new architecture.
   - Document the dual-output strategy and why both directories are needed.
   - Clarify that TinyWasm does not handle final JavaScript bundling.

## Benefits

- **Clear Separation:** WASM binaries vs. JavaScript runtime files have distinct purposes and locations.
- **Better Integration:** File watchers and asset bundlers can properly track changes.
- **Full Compatibility:** Aligns with Go best practices and your project's architecture.
- **Elimination of Ambiguity:** `WasmExecJsOutputDir` clearly indicates its purpose.
- **Easier Onboarding:** New contributors understand the build pipeline flow.
- **Mode Transparency:** External tools can detect compilation mode changes via `wasm_exec.js`.
