# BUG: Mismatched wasm_exec.js injected by JavascriptForInitializing() -> runtime import error in browser

## Summary

When `AssetMin` builds `main.js` it calls a configured `GetRuntimeInitializerJS()` which returns the contents of a `wasm_exec.js` (either Go's or TinyGo's). In some circumstances the injected JS runtime does not match the runtime expected by the compiled `main.wasm`. That mismatch leads to a runtime import error in the browser such as:

`Import #0 "gojs" "runtime.scheduleTimeoutEvent": function import requires a callable`

This happens when the `.wasm` expects Go's import (e.g. `runtime.scheduleTimeoutEvent`) but the injected JS provides TinyGo's runtime exports (e.g. `runtime.sleepTicks`) — or vice versa.

## Symptoms

- Browser console error complaining that a WASM import is missing or not callable, e.g. `runtime.scheduleTimeoutEvent` or similar.
- `main.js` contains the wrong runtime shim (inspect for `sleepTicks` vs `scheduleTimeoutEvent`).
- `main.wasm` imports point to a different runtime API than the runtime code provided by `main.js`.

## Reproduction (typical)

1. Configure `AssetMin` with `GetRuntimeInitializerJS` pointing to `TinyWasm.JavascriptForInitializing`.
2. Build or change JS files so the pipeline regenerates `public/main.js` (this file includes the `wasm_exec.js` content returned by `GetRuntimeInitializerJS`).
3. Serve `main.wasm` compiled with one toolchain (Go or TinyGo) but `main.js` contains the other toolchain's `wasm_exec.js`.
4. Load the page in a browser and observe import/link errors as above.

## Root cause

`TinyWasm.JavascriptForInitializing()` chooses which `wasm_exec.js` content to load (Go vs TinyGo) based on configuration flags and caches the result. The file caching + selection logic can become out-of-sync with the actual compiler used to produce the `.wasm` artifact. Specifically:

- The `.wasm` binary may require Go's runtime imports (e.g. `runtime.scheduleTimeoutEvent`) but `JavascriptForInitializing()` injected TinyGo's `wasm_exec.js` (which implements different imports such as `runtime.sleepTicks`).
- Because the import names differ, WebAssembly instantiation fails in the browser with the shown import error.

## Evidence (examples from local workspace)

- `/usr/local/go/lib/wasm/wasm_exec.js` contains implementation for `runtime.scheduleTimeoutEvent` (Go runtime API).
- `/usr/local/lib/tinygo/targets/wasm_exec.js` exposes `runtime.sleepTicks` (TinyGo runtime API).
- `public/main.js` (generated) contained `runtime.sleepTicks` while `public/main.wasm` required `runtime.scheduleTimeoutEvent`.

## Immediate workarounds

1. Make sure you compile `.wasm` and include the `wasm_exec.js` from the same toolchain. For example:
   - If you used `go` to build the wasm, embed `/usr/local/go/lib/wasm/wasm_exec.js`.
   - If you used `tinygo`, embed `/usr/local/lib/tinygo/targets/wasm_exec.js`.
2. Clear JS caches (call `ClearJavaScriptCache()` if available or remove generated `main.js`) and regenerate after compilation.

## Implementation status (updated)

The codebase diverged from the original full proposal: instead of a new, separate detection module, a compact, pragmatic implementation was added directly to `tinywasm` to detect runtime type from JavaScript assets during initial registration / file events.

Summary of the actual changes made in the codebase:

- `TinyWasm` now exposes `SupportedExtensions()` returning `[]string{".js", ".go"}` and `DevWatch`/`godev` call `NewFileEvent` with JS events during initial registration.
- `NewFileEvent(...)` in `tinywasm` calls two detection handlers now:
    - `wasmDetectionFuncFromJsFile(fileName, extension, filePath, event)` — active by default
    - `wasmDetectionFuncFromGoFile(fileName, filePath)` — active by default
- A JS-focused detector was implemented in `javascripts.go` as `wasmDetectionFuncFromJsFileActive`.
    - It only processes files with extension `.js` and event `create`.
    - It only analyzes files located under `AppRootDir/WebFilesRootRelative/WebFilesSubRelative` (the web output folder used by your project).
    - It scans the file content for two small signature lists defined in `javascripts.go`:
        - `wasm_execGoSignatures()` — e.g. `runtime.scheduleTimeoutEvent`, `runtime.clearTimeoutEvent`, `runtime.wasmExit`
        - `wasm_execTinyGoSignatures()` — e.g. `runtime.sleepTicks`, `runtime.ticks`, `$runtime.alloc`, `tinygo_js`
    - Detection logic picks the runtime with the higher signature count (or accepts a single-side match) and ignores ambiguous cases.
    - On successful detection it:
        - sets `w.wasmProject = true`
        - sets `w.tinyGoCompiler` to true for TinyGo detection or false for Go detection
        - calls `ClearJavaScriptCache()` to force regeneration of cached `wasm_exec.js` contents
        - deactivates both detectors by assigning the inactive variants:
            - `w.wasmDetectionFuncFromGoFile = w.wasmDetectionFuncFromGoFileInactive`
            - `w.wasmDetectionFuncFromJsFile = w.wasmDetectionFuncFromJsFileInactive`

Files added/changed (high level)
- `tinywasm/javascripts.go` — added:
    - `wasm_execGoSignatures()` and `wasm_execTinyGoSignatures()` signature lists
    - `wasmDetectionFuncFromJsFileActive(...)` (implementation)
    - `wasmDetectionFuncFromJsFileInactive(...)` (no-op)
    - small imports: `path/filepath`, `strings`
- `tinywasm/file_event.go` — updated `NewFileEvent(...)` to call the JS detection handler and renamed/kept the Go-file detection functions.
- `tinywasm/tinywasm.go` — added wiring to initialize the detection handlers:
    - `w.wasmDetectionFuncFromJsFile = w.wasmDetectionFuncFromJsFileActive`
    - `w.wasmDetectionFuncFromGoFile = w.wasmDetectionFuncFromGoFileActive`
- Tests added:
    - `tinywasm/javascripts_test.go` — verifies `JavascriptForInitializing()` returns the expected `wasm_exec.js` content for both Go and TinyGo (searches for signatures). This test is skipped if `tinygo` isn't in PATH.
    - `tinywasm/js_file_event_test.go` — exercises JS `create` events: writes JS files into `AppRootDir/WebFilesRootRelative/WebFilesSubRelative` with Go/TinyGo signatures and asserts that `w.tinyGoCompiler`, `w.wasmProject` and detector state update as implemented.

Why this differs from the original proposal
- The original doc proposed a separate detection API and a multi-step refactor. The pragmatic implementation chosen keeps detection inside `tinywasm` (smaller surface area) and implements a best-effort JS signature scanner triggered during DevWatch initial registration (and subsequent `create` events).

Behavioral notes / guarantees
- Detection is best-effort and intentionally conservative: ambiguous or mixed-signature files do not flip configuration.
- Detection only runs for `.js` files on `create` events located under the configured web subfolder; it won't analyze arbitrary JS outside the web output directory.
- After a successful detection the detection handlers become no-ops to avoid repeated or conflicting reconfiguration at runtime. This preserves stable behavior after initial startup.

Limitations and risks
- Signature lists are small and platform-specific; they may need expansion to remain robust across toolchain versions.
- Minified or mangled JS might omit the chosen signatures; detection can fail in that case (it is non-fatal and leaves current configuration unchanged).
- The detection currently uses a simple counts comparison — rare corner cases with mixed artifacts could still produce ambiguous outcomes; we ignore ambiguous cases.

Testing
- Unit tests for the behavior were added under `tinywasm/` and were executed during development. They exercise both the `JavascriptForInitializing()` behavior and the `NewFileEvent()`+JS detection flow.

Rollout / migration notes
- The change is backward compatible: if detection fails the previous behavior remains (the configured caches and `JavascriptForInitializing()` logic are unchanged).
- To force a specific runtime (skip auto-detection) you can set configuration to prefer a mode and/or clear caches manually (call `ClearJavaScriptCache()`), but consider exposing a user-facing override flag if needed.

Next steps (recommended)
1. Expand signature lists or add a small, maintainable test fixture per toolchain to strengthen detection.
2. Optionally cache detection results per file/timestamp to avoid re-reading large JS files on repeated events.
3. Consider adding a configuration flag to explicitly force the runtime type (disable auto-detection) for CI or deterministic builds.
4. Add an optional integration test that runs in an environment with both `go` and `tinygo` installed to validate end-to-end behavior.

If you want, I can now:
- extract the detection logic into a small reusable function (for clearer unit tests),
- add a configuration flag to opt-out of detection, or
- broaden the signature database and update tests accordingly.

---

Filed by: developer request (devwatch orchestrator refactor)
Date: 2025-09-02
