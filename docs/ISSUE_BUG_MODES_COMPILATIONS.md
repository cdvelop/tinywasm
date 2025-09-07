# ISSUE: Compilation Mode Switching Bug

## Problem Description

The TinyWasm package has a critical bug in the compilation mode switching mechanism. While the system is designed to support three compilation modes (coding, debugging, production), the mode switching functionality is not working correctly.

## Expected Behavior

The TinyWasm package should support three distinct compilation modes:

1. **Coding Mode** (`c`): Fast compilation using Go standard compiler with `GOOS=js GOARCH=wasm`
2. **Debugging Mode** (`d`): TinyGo compilation with debug symbols (`-opt=1`)
3. **Production Mode** (`p`): TinyGo optimized compilation (`-opt=z -no-debug -panic=trap`)

When users call `TinyWasm.Change(mode)`, the system should:
- Switch the active builder to the appropriate mode
- Update internal state to reflect the new mode
- Recompile the WASM file using the new compiler settings

## Actual Behavior

Based on test results from `TestCompileAllModes`, the following issues were identified:

1. **Mode Detection Failure**: `Value()` method returns empty string instead of current mode
2. **Builder Not Switching**: All modes use the same `*gobuild.GoBuild` instance (coding mode)
3. **Incorrect Progress Messages**: All modes report "Switching to coding mode" regardless of requested mode
4. **Same Output Size**: All modes produce identical file sizes (1,598,360 bytes), indicating same compilation

## Test Case

The issue is verified by the test `TestCompileAllModes` in `compile_modes_test.go` which:

1. Creates a minimal WASM project structure
2. Calls `NewFileEvent` with "create" event (simulates `InitialRegistration`)
3. Calls `Change()` for each mode ("c", "d", "p")
4. Calls `NewFileEvent` with "write" event (simulates file modification)
5. Verifies compilation output exists on disk

### Test Output Analysis

```
Before Change - Current mode: , Active builder: *gobuild.GoBuild
After Change to debugging mode - Current mode: , Active builder: *gobuild.GoBuild, Progress: Switching to coding mode
```

This shows:
- `Value()` returns empty string (should return "c", "d", or "p")
- `activeBuilder` doesn't change type (should be different instances)
- Progress message is wrong (should reflect actual mode)

## Root Cause Analysis

### 1. Value() Method Issue

The `Value()` method compares `w.activeBuilder` with `w.builderCoding`, `w.builderDebug`, and `w.builderProduction` using reference equality (`==`). However, all builders are instances of `*gobuild.GoBuild`, so the comparison may be failing.

```go
func (w *TinyWasm) Value() string {
    if w.activeBuilder == w.builderCoding {
        return w.Config.CodingShortcut
    }
    // ... other comparisons
    return w.Config.CodingShortcut // fallback
}
```

### 2. Builder Initialization Issue

In `builderWasmInit()`, all three builders are created as separate `*gobuild.GoBuild` instances with different configurations. However, the pointer comparison in `Value()` may not work as expected.

### 3. Change() Method Success Messages

The `getSuccessMessage()` method appears to work correctly, but the progress callback might be receiving the wrong mode value due to the `Value()` method bug.

## Possible Solutions

### Solution 1: Add Mode Tracking Field

Add an explicit `currentMode` field to track the active mode:

```go
type TinyWasm struct {
    // ... existing fields
    currentMode string // Track current mode explicitly
}

func (w *TinyWasm) Value() string {
    if w.currentMode == "" {
        return w.Config.CodingShortcut // default
    }
    return w.currentMode
}

func (w *TinyWasm) updateCurrentBuilder(mode string) {
    // ... existing logic
    w.currentMode = mode // Update tracking field
}
```

### Solution 2: Builder Type Identification

Add a method to identify builder types or store builder metadata:

```go
type builderInfo struct {
    builder *gobuild.GoBuild
    mode    string
}

// Store builder info instead of just builders
```

### Solution 3: Pointer Comparison Debug

Investigate why pointer comparison fails by adding logging to see if builders are being replaced unexpectedly.

## Impact

- **Functional Impact**: Users cannot switch between compilation modes
- **Development Impact**: No access to TinyGo optimizations or debug features
- **Performance Impact**: Always uses Go standard compiler (slower runtime, larger files)
- **Debugging Impact**: Cannot use TinyGo debug mode for better debugging experience

## Test Coverage

The `TestCompileAllModes` test successfully:
- ✅ Verifies compilation works for all modes
- ✅ Checks file output exists on disk  
- ✅ Simulates real integration flow (`NewFileEvent` + `Change`)
- ❌ Reveals mode switching doesn't work

## Priority

**HIGH** - This breaks a core feature of the TinyWasm package and prevents users from accessing different compilation modes.

## Next Steps

1. Implement Solution 1 (explicit mode tracking) as it's the most straightforward
2. Add unit tests specifically for mode switching logic
3. Verify that different modes actually use different compiler commands
4. Test file size differences between modes to ensure they're truly different
