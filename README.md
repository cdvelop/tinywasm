# tinywasm

Go package for intelligent WebAssembly compilation with automatic file detection.

## Features

- Smart file detection via prefixes (frontend/backend separation)
- Dual compiler support: TinyGo (production, 47KB) / Go standard (dev, 1.6MB)
- VS Code auto-configuration for WASM development
- Single output: All files → `main.wasm`

## Quick Start

```go
config := &tinywasm.Config{
    WebFilesFolder: func() (string, string) { return "web", "public" },
    FrontendPrefix: []string{"f.", "ui."},
}
tw := tinywasm.New(config)
tw.NewFileEvent("main.wasm.go", ".go", "web/main.wasm.go", "write")
```

## File Detection

**Compiles:** `*.wasm.go`, frontend prefixes (`f.*.go`), modules/*/
**Ignores:** Backend prefixes (`b.*.go`), root `main.go`, non-Go files

## VS Code Integration

Auto-creates `.vscode/settings.json` with WASM environment:
```json
{"gopls": {"env": {"GOOS": "js", "GOARCH": "wasm"}}}
```

## API

**Core:**
- `New(config *Config) *TinyWasm`
- `NewFileEvent(fileName, ext, path, event string) error`
- `ShouldCompileToWasm(fileName, path string) bool`

**Compiler:**
- `TinyGoCompiler() bool` / `SetTinyGoCompiler(bool) error`
- `VerifyTinyGoInstallation() error`

**Utils:**
- `OutputPathMainFileWasm() string`
- `UnobservedFiles() []string`
- `JavascriptForInitializing() (string, error)`

## Config

```go
type Config struct {
    WebFilesFolder func() (string, string) // web dir, public dir
    FrontendPrefix []string                // frontend prefixes
    Log io.Writer                          // compilation output
    Callback func(string, error)          // optional callback
    CompilingArguments func() []string     // compiler args
}
```

## Requirements

Go 1.20+, TinyGo (optional)
