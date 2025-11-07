# TinyWasm: Refactor Change Method to Use Channel-Based Progress

## Objective
Refactor the `Change` method signature in TinyWasm library from callback-based `func(msgs ...any)` to channel-based `chan<- string` to match DevTUI's new interface and enable better streaming support for MCP tools.

## Current Signature
```go
// Change.go
func (w *TinyWasm) Change(newValue string, progress func(msgs ...any))
```

## Target Signature
```go
// Change.go
func (w *TinyWasm) Change(newValue string, progress chan<- string)
```

## Rationale
1. **Consistency**: DevTUI updated to use channels instead of callbacks
2. **MCP Integration**: TinyWasm exposes tools via MCP that stream progress messages
3. **Simplicity**: Single string messages instead of variadic any
4. **Idiomatic Go**: Channels for communication between goroutines

## Files to Modify

### 1. `/Change.go`
Update main Change method implementation:

```go
// Change updates the compiler mode for TinyWasm and reports progress via the provided channel.
// Implements the HandlerEdit interface: Change(newValue string, progress chan<- string)
func (w *TinyWasm) Change(newValue string, progress chan<- string) {
    // Normalize input: trim spaces and convert to uppercase
    newValue = Convert(newValue).ToUpper().String()

    // Validate mode
    if err := w.validateMode(newValue); err != nil {
        progress <- err.Error() // Changed from progress(err)
        return
    }

    // Check TinyGo installation for debug/production modes
    if w.requiresTinyGo(newValue) && !w.tinyGoInstalled {
        progress <- w.handleTinyGoMissing().Error() // Changed from progress(...)
        return
    }

    // Update active builder
    w.updateCurrentBuilder(newValue)

    // Check if main WASM file exists
    sourceDir := path.Join(w.AppRootDir, w.Config.SourceDir)
    mainWasmPath := path.Join(sourceDir, w.Config.MainInputFile)
    if _, err := os.Stat(mainWasmPath); err != nil {
        progress <- w.getSuccessMessage(newValue) // Changed from progress(...)
        return
    }

    // Auto-recompile
    if err := w.RecompileMainWasm(); err != nil {
        warningMsg := Translate("Warning:", "auto", "compilation", "failed:", err).String()
        if warningMsg == "" {
            warningMsg = "Warning: auto compilation failed: " + err.Error()
        }
        progress <- warningMsg // Changed from progress(warningMsg)
        return
    }

    // Ensure wasm_exec.js is available
    w.wasmProjectWriteOrReplaceWasmExecJsOutput()

    // Report success
    progress <- w.getSuccessMessage(newValue) // Changed from progress(...)
}
```

### 2. `/mcp-tool.go`
Update MCP tool Execute function that calls Change:

```go
// Current
Execute: func(args map[string]any, progress func(msgs ...any)) error {
    // ...
    w.Change(mode, progress)
    return nil
},

// Target
Execute: func(args map[string]any, progress chan<- string) {
    // ...
    w.Change(mode, progress)
},
```

Note: The Execute signature also changes to remove error return (see separate MCP refactor).

### 3. Test Files

Update all test files that call `Change` method with progress callback:

#### `/compile_modes_test.go`
```go
// Before
progressCb := func(msgs ...any) {
    if len(msgs) > 0 {
        progressMsg = fmt.Sprint(msgs...)
    }
}
w.Change(tc.mode, progressCb)

// After
progressChan := make(chan string, 5)
go func() {
    for msg := range progressChan {
        progressMsg = msg
    }
}()
w.Change(tc.mode, progressChan)
close(progressChan)
```

#### `/file_event_test.go`
```go
// Before
var changeMsg string
tinyWasm.Change("M", func(msgs ...any) {
    if len(msgs) > 0 {
        changeMsg = fmt.Sprint(msgs...)
    }
})

// After
progressChan := make(chan string, 1)
var changeMsg string
go func() {
    for msg := range progressChan {
        changeMsg = msg
    }
}()
tinyWasm.Change("M", progressChan)
close(progressChan)
```

#### `/tinystring_test.go`
```go
// Before
var got string
tw.Change("L", func(msgs ...any) {
    if len(msgs) > 0 {
        got = fmt.Sprint(msgs...)
    }
})

// After
progressChan := make(chan string, 1)
var got string
done := make(chan bool)
go func() {
    for msg := range progressChan {
        got = msg
    }
    done <- true
}()
tw.Change("L", progressChan)
close(progressChan)
<-done
```

#### `/compiler_test.go`
```go
// Before
var msg string
tinyWasm.Change("b", func(msgs ...any) {
    if len(msgs) > 0 {
        msg = fmt.Sprint(msgs...)
    }
})

// After
progressChan := make(chan string, 1)
var msg string
done := make(chan bool)
go func() {
    for m := range progressChan {
        msg = m
    }
    done <- true
}()
tinyWasm.Change("b", progressChan)
close(progressChan)
<-done
```

## Pattern for Test Updates

Use this helper pattern for all tests:

```go
// Helper function for collecting messages from channel
func collectProgress(ch <-chan string) string {
    var result string
    for msg := range ch {
        result = msg // Keep last message
    }
    return result
}

// Usage in test
progressChan := make(chan string, 5)
go func() {
    message = collectProgress(progressChan)
}()
w.Change(mode, progressChan)
close(progressChan)
// Wait briefly for goroutine
time.Sleep(10 * time.Millisecond)
```

## Implementation Steps

1. **Update Change.go** - Change method signature and all progress calls
2. **Update mcp-tool.go** - Update Execute function calls to Change
3. **Update compile_modes_test.go** - Refactor all Change calls with channels
4. **Update file_event_test.go** - Refactor Change calls
5. **Update tinystring_test.go** - Refactor Change calls  
6. **Update compiler_test.go** - Refactor Change calls
7. **Run tests** - Verify all tests pass: `go test ./...`

## Key Implementation Details

### Message Collection Pattern
```go
// Create buffered channel (size depends on expected messages)
progressChan := make(chan string, 5)

// Collect messages in goroutine
var messages []string
done := make(chan bool)
go func() {
    for msg := range progressChan {
        messages = append(messages, msg)
    }
    done <- true
}()

// Execute
w.Change(mode, progressChan)

// IMPORTANT: Close channel after sending
close(progressChan)

// Wait for collection
<-done

// Use collected messages
lastMessage := ""
if len(messages) > 0 {
    lastMessage = messages[len(messages)-1]
}
```

### Avoid Deadlocks
- Always close channel after sending messages
- Use buffered channels to avoid blocking
- Start collector goroutine BEFORE calling Change
- Wait for goroutine completion with done channel

## Breaking Changes
⚠️ **This is a BREAKING CHANGE** - All code calling `TinyWasm.Change()` must update to use channels.

**External Consumers:**
- GoLite (will be updated separately)
- Any other tools using TinyWasm directly

## Success Criteria
- [ ] Change.go updated with new signature
- [ ] All progress calls changed from `progress(msg)` to `progress <- msg`
- [ ] mcp-tool.go Execute function updated
- [ ] All test files updated (5 files)
- [ ] All tests pass: `go test ./...`
- [ ] No compilation errors
- [ ] No deadlocks in tests
- [ ] Messages collected correctly

## Test Verification Commands

```bash
# Run all tests
cd /home/cesar/Dev/Pkg/Mine/tinywasm
go test ./... -v

# Run specific test files
go test -v -run TestCompileAllModes
go test -v -run TestTinyWasmNewFileEvent
go test -v -run TestTinyStringMessages
go test -v -run TestCompilerComparison
```

## Notes
- Use buffered channels with reasonable size (5-10)
- Always close channel in caller
- Collector goroutine runs until channel closed
- Single message per operation (keep last)
- Format error messages before sending to channel
