# tinywasm

Go package for intelligent WebAssembly compilation with automatic file detection and 3-mode compiler system.

## Features

- **3-Mode Compiler System**: Coding ("c"), Debug ("d"), Production ("p")
- **DevTUI Integration**: FieldHandler interface for interactive mode switching
- Smart file detection via prefixes (frontend/backend separation)
- Triple compiler support: Go standard (fast dev), TinyGo debug (-opt=1), TinyGo production (-opt=z)
- VS Code auto-configuration for WASM development
- Single output: All files → `main.wasm`

## Quick Start

```go
// Basic usage
config := tinywasm.NewConfig() // Pre-configured with defaults
config.WebFilesFolder = func() (string, string) { return "web", "public" }

tw := tinywasm.New(config)
tw.NewFileEvent("main.wasm.go", ".go", "web/main.wasm.go", "write")

// DevTUI Integration - 3 Mode System
fmt.Println("Current mode:", tw.Value())    // "c" (coding)
msg, err := tw.Change("d")                  // Switch to debug mode
fmt.Println("Status:", msg)                 // "Switching to debugging mode"

// Advanced configuration
config := &tinywasm.Config{
    WebFilesFolder:      func() (string, string) { return "web", "public" },
    FrontendPrefix:      []string{"f.", "ui.", "view."},
    CodingShortcut:      "c",  // Customizable shortcuts
    DebuggingShortcut:   "d",
    ProductionShortcut:  "p",
}
```

## File Detection

**Compiles:** `*.wasm.go`, frontend prefixes (`f.*.go`), modules/*/
**Ignores:** Backend prefixes (`b.*.go`), root `main.go`, non-Go files

## DevTUI FieldHandler Interface

TinyWasm implements the DevTUI FieldHandler interface for interactive development:

```go
// DevTUI Integration
label := tw.Label()           // "Compiler Mode"
current := tw.Value()         // Current mode shortcut ("c", "d", "p")
canEdit := tw.Editable()      // true
timeout := tw.Timeout()       // 0 (no timeout)

// Interactive mode change with validation
msg, err := tw.Change("d")
if err != nil {
    // Handle validation errors (invalid mode, missing TinyGo, etc.)
}
// msg contains success message or warning if auto-compilation fails
```

## VS Code Integration

Auto-creates `.vscode/settings.json` with WASM environment:
```json
{"gopls": {"env": {"GOOS": "js", "GOARCH": "wasm"}}}
```

## API

**Core:**
- `New(config *Config) *TinyWasm`
- `NewConfig() *Config` - Pre-configured with sensible defaults
- `NewFileEvent(fileName, ext, path, event string) error`
- `ShouldCompileToWasm(fileName, path string) bool`

**DevTUI FieldHandler Interface:**
- `Label() string` - Returns "Compiler Mode"
- `Value() string` - Current mode shortcut ("c", "d", "p")
- `Editable() bool` - Returns true (field is editable)
- `Change(newValue any) (string, error)` - Switch compiler mode with validation
- `Timeout() time.Duration` - Returns 0 (no timeout)

**Legacy Compiler Methods (deprecated):**
- `TinyGoCompiler() bool` - Use `Value()` instead
- `SetTinyGoCompiler(bool) error` - Use `Change()` instead
- `VerifyTinyGoInstallation() error`

**Utils:**
- `MainFileRelativePath() string`
- `UnobservedFiles() []string`
- `JavascriptForInitializing() (string, error)`

## Config

```go
type Config struct {
    // Core settings
    WebFilesFolder func() (string, string) // web dir, public dir
    FrontendPrefix []string                // frontend prefixes
    Writer io.Writer                       // compilation output (renamed from Log)
    
    // 3-Mode System (NEW)
    CodingShortcut     string             // default: "c"
    DebuggingShortcut  string             // default: "d" 
    ProductionShortcut string             // default: "p"
    
    // Legacy/Advanced
    TinyGoCompiler     bool               // deprecated, use mode system
    Callback           func(string, error) // optional callback
    CompilingArguments func() []string     // compiler args
}

// Pre-configured constructor (recommended)
func NewConfig() *Config
```

**Migration from v1:**
```go
// Old way (deprecated)
config.TinyGoCompiler = true

// New way (recommended)  
tw.Change("p")  // production mode with TinyGo -opt=z
tw.Change("d")  // debug mode with TinyGo -opt=1
tw.Change("c")  // coding mode with Go standard
```

## Requirements

- Go 1.20+
- TinyGo (optional, required for debug/production modes)
- DevTUI (optional, for interactive development)

## Migration Guide

**From TinyWasm v1.x:**

| Old API | New API | Notes |
|---------|---------|-------|
| `SetTinyGoCompiler(true)` | `Change("p")` | Production mode |
| `SetTinyGoCompiler(false)` | `Change("c")` | Coding mode |
| `TinyGoCompiler()` | `Value() == "p" \|\| Value() == "d"` | Check if using TinyGo |
| `config.Log` | `config.Writer` | Field renamed |

**Benefits of v2.x:**
- 🎯 **3 optimized modes** instead of binary choice
- 🔧 **DevTUI integration** for interactive development  
- 📦 **Smaller debug builds** with TinyGo -opt=1
- ⚡ **Auto-recompilation** on mode switch
- 🛠️ **Better error handling** with validation
