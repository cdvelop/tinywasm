# tinywasm

`tinywasm` is a Go package designed to be integrated into development kits, providing specific functionality for compiling Go files to WebAssembly (WASM) using the TinyGo compiler in an optimized way with intelligent file detection.

## Features

- **Development kit integration**: Designed to be embedded in broader development tools
- **Single WASM output**: All files compile to a single `main.wasm` file for optimized loading
- **Intelligent file detection**: Automatically distinguishes between frontend and backend files
- **Frontend prefix support**: Configurable prefixes to identify frontend-specific files
- **Module support**: Handles both main files (`main.wasm.go`) and module files with smart detection
- **TinyGo by default**: Uses TinyGo as the default compiler to generate smaller WASM files
- **JavaScript file management**: Automatically provides necessary `wasm_exec.js` files
- **Flexible configuration**: Allows customization of output directories and frontend prefixes
- **Simple API**: Clear interface for integration with other projects

## Integration

This package is designed to be used as a dependency in other projects:

```bash
go get github.com/your-user/tinywasm
```

## Usage in Development Kit

### Basic Integration

```go
package main

import (
    "os"
    "github.com/your-user/tinywasm"
)

func main() {
    // Configure TinyWasm with frontend prefix detection
    config := &tinywasm.WasmConfig{
        WebFilesFolder: func() (string, string) { 
            return "web", "public" 
        },
        Log: os.Stdout, // Log output for integration with external systems (TUI, console)
        FrontendPrefix: []string{"f.", "frontend.", "ui."}, // Configure frontend file prefixes
    }

    // Create TinyWasm instance
    tw := tinywasm.New(config)

    // Integrate into development kit event system
    // All files compile to single main.wasm output
    err := tw.NewFileEvent("main.wasm.go", ".go", "web/main.wasm.go", "write")
    if err != nil {
        // Handle error in development kit context
        panic(err)
    }
}
```

### Usage in File Watcher

This package is ideal for integration into file monitoring systems with intelligent file detection:

```go
// Example integration in a watcher with frontend/backend detection
func handleFileChange(filePath, event string) {
    fileName := filepath.Base(filePath)
    
    // The package automatically determines if the file should be compiled
    // based on: .wasm.go extension, frontend prefixes, or location in modules
    err := tinyWasm.NewFileEvent(fileName, ".go", filePath, event)
    if err != nil {
        log.Printf("Error compiling WASM: %v", err)
    }
}

// Example: File detection logic
// ✅ Compiles: main.wasm.go, users.wasm.go, f.login.go, api.go
// ❌ Ignores: b.service.go, backend.logic.go, server.auth.go
```


### Integration with Logging Systems

The `Log` field allows integration with different systems:

```go
// Integration with TUI (Terminal User Interface)
var tuiOutput io.Writer = getTUILogWriter()

// Integration with custom logging  
var customLogger io.Writer = getCustomLogger()

// Integration with buffer for testing
var testBuffer bytes.Buffer

config := &tinywasm.WasmConfig{
    WebFilesFolder: func() (string, string) { return "web", "public" },
    Log: tuiOutput, // or customLogger, or &testBuffer
    FrontendPrefix: []string{"f.", "client.", "ui."}, // Configure as needed
}
```

### Frontend/Backend File Detection

The package intelligently detects which files should be compiled to WASM:

```go
// Files that WILL be compiled to WASM:
// ✅ main.wasm.go (always)
// ✅ users.wasm.go (explicit WASM module) 
// ✅ f.login.go (frontend prefix)
// ✅ client.auth.go (frontend prefix)
// ✅ api.go (regular Go file in modules)

// Files that will NOT be compiled:
// ❌ b.service.go (unknown prefix with dot - assumed backend)
// ❌ backend.logic.go (unknown prefix with dot - assumed backend)
// ❌ server.auth.go (unknown prefix with dot - assumed backend)
// ❌ script.js (non-Go file)
// ❌ main.go (root level file)

config := &tinywasm.WasmConfig{
    FrontendPrefix: []string{"f.", "frontend.", "client.", "ui."},
    // Only files with these prefixes are considered frontend
    // All other prefixed files (with dots) are assumed to be backend
}
```

## Use Cases

### Integration in Development Tools
- **Hot reload**: Automatic recompilation during development with intelligent file detection
- **Build systems**: Integration into build pipelines with single WASM output
- **IDEs and editors**: Support for development extensions with frontend/backend awareness
- **Development servers**: On-demand compilation with smart file filtering

### Compatible Development Kits
This package can be easily integrated into:
- Automated build systems with file type detection
- Web development tools with frontend/backend separation
- Go development frameworks with WASM support
- Development servers with hot reload and intelligent compilation

## File Structure and Compilation Rules

The package expects a specific file structure and follows intelligent compilation rules:

```
project/
├── web/
│   ├── main.wasm.go            # ✅ Main WASM file (always compiles) - Input in web root
│   └── public/
│       ├── main.wasm           # Generated output (single file for all) - Output in public root
│       └── wasm_exec.js        # JS runtime (generated) - In public root
└── modules/
    ├── users/
    │   ├── api.go              # ✅ Regular Go file (compiles)
    │   ├── f.login.go          # ✅ Frontend prefix (compiles) 
    │   ├── b.service.go        # ❌ Backend prefix (ignored)
    │   └── users.wasm.go       # ✅ Explicit WASM (compiles)
    └── auth/
        ├── handler.go          # ✅ Regular Go file (compiles)
        ├── client.auth.go      # ✅ Frontend prefix (compiles)
        └── server.logic.go     # ❌ Unknown prefix (ignored)
```

### Compilation Rules:
1. **Always compile**: `main.wasm.go` and `*.wasm.go` files
2. **Frontend files**: Files with configured frontend prefixes
3. **Module Go files**: Regular `.go` files in modules directory (without unknown prefixes)
4. **Ignored files**: Files with unknown prefixes containing dots (assumed backend)
5. **Single output**: All compiled files result in one `main.wasm` file

## Complete API Reference

### Types

#### `TinyWasm`
Main structure that handles WASM compilation with the following fields:
- `ModulesFolder string`: Directory for modules (default: "modules")
- `mainInputFile string`: Main input file name (default: "main.wasm.go")
- `mainOutputFile string`: Main output file name (default: "main.wasm")

#### `WasmConfig`
Configuration for the WASM compiler:
- `WebFilesFolder func() (string, string)`: Function that returns web folders (e.g: "web", "public")
- `Log io.Writer`: Output for compilation logs, ideal for TUI or custom system integration
- `FrontendPrefix []string`: Prefixes used to identify frontend files (e.g: ["f.", "frontend.", "ui."])

### Core Methods

#### `New(config *WasmConfig) *TinyWasm`
Creates a new TinyWasm instance with the provided configuration.

#### `NewFileEvent(fileName, extension, filePath, event string) error`
Processes file events for WASM compilation with intelligent file detection.
- `fileName`: Name of the file (e.g., "main.wasm.go", "f.login.go")
- `extension`: File extension (e.g., ".go")
- `filePath`: Full path to the file
- `event`: Type of file event ("create", "remove", "write", "rename")

**Note**: Only "write" events trigger compilation. The method automatically determines if a file should be compiled based on the intelligent detection rules.

#### `ShouldCompileToWasm(fileName, filePath string) bool`
Determines if a file should trigger WASM compilation based on:
- Main WASM file (`main.wasm.go`)
- Explicit WASM files (`*.wasm.go`) 
- Frontend prefix configuration
- Unknown prefixes with dots (assumed backend, returns false)
- Regular Go files in modules (returns true)

#### `OutputPathMainFileWasm() string`
Returns the output path for the main WASM file (e.g: "web/public/main.wasm").

#### `UnobservedFiles() []string`
Returns files that should not be watched for changes (e.g: ["main.wasm"]). 

**Note**: `main.wasm.go` IS watched for changes as developers can modify it. Only the generated `main.wasm` file is excluded from watching.

### Compiler Methods

#### `TinyGoCompiler() bool`
Indicates if TinyGo compiler should be used (always true for this package).

#### `WasmProjectTinyGoJsUse() (bool, bool)`
Returns whether TinyGo JS should be used for the project.

### JavaScript Integration

#### `JavascriptForInitializing() (string, error)`
Returns the JavaScript code needed to initialize WASM. Provides the appropriate `wasm_exec.js` content based on the compiler being used.

### Utility Methods

#### `GetModuleName(filePath string) (string, error)`
Extracts module name from file path (e.g: extracts "users" from "modules/users/users.wasm.go").

### Verification Methods

#### `VerifyTinyGoInstallation() error`
Checks if TinyGo is properly installed and available in PATH.

#### `GetTinyGoVersion() (string, error)`
Returns the installed TinyGo version.

#### `VerifyTinyGoProjectCompatibility()`
Checks if the project is compatible with TinyGo compilation by analyzing imports and dependencies.

## Supported Events

- **`write`**: Compiles the file when saved
- **`create`**: Ignored (does not compile)
- **`remove`**: Ignored (does not compile)
- **`rename`**: Ignored (does not compile)

## Compilers

### TinyGo (Default)
- Generates smaller WASM files
- Optimized for web applications
- Command: `tinygo build -o output.wasm -target wasm --no-debug input.go`

### Standard Go (Alternative)
- Full Go compatibility
- Larger WASM files
- Command: `GOOS=js GOARCH=wasm go build -o output.wasm input.go`

## WASM Application Example

```go
// main.wasm.go - Single entry point for all WASM functionality
package main

import "syscall/js"

func hello(this js.Value, args []js.Value) any {
    return "Hello from TinyWasm!"
}

func main() {
    js.Global().Set("hello", js.FuncOf(hello))
    
    // All module functionality can be imported and used here
    // since everything compiles to a single main.wasm file
    
    select {} // Keep the program alive
}
```

### Frontend File Example

```go
// modules/auth/f.login.go - Frontend file (will be compiled)
package auth

import "syscall/js"

func LoginHandler() js.Func {
    return js.FuncOf(func(this js.Value, args []js.Value) any {
        // Frontend login logic
        return "Login successful"
    })
}
```

### Backend File Example

```go
// modules/auth/b.service.go - Backend file (will be ignored)
package auth

import "database/sql"

func AuthenticateUser(username, password string) bool {
    // This backend logic won't be compiled to WASM
    // as it has the "b." prefix which is not in FrontendPrefix
    return true
}
```

## Development and Testing

### Run Tests

```bash
go test ./...
```

### Verify TinyGo Installation

The package includes utilities to verify that TinyGo is correctly installed:

```go
// Verify if TinyGo is available
if err := tw.VerifyTinyGoInstallation(); err != nil {
    log.Fatal("TinyGo is not installed or not in PATH:", err)
}

// Get TinyGo version
version, err := tw.GetTinyGoVersion()
if err != nil {
    log.Fatal("Failed to get TinyGo version:", err)
}
fmt.Println("TinyGo version:", version)

// Check project compatibility
tw.VerifyTinyGoProjectCompatibility()
```

## Dynamic Compiler Selection

TinyWasm now supports dynamic compiler selection between standard Go and TinyGo compilers, with automatic project detection and intelligent switching capabilities.

### Compiler Configuration

```go
// Create TinyWasm instance with default TinyGo compiler
tw := tinywasm.New(config)

// Check if using TinyGo compiler
isTinyGo := tw.TinyGoCompiler() // Returns true if TinyGo is configured

// Check if current project is detected as WASM project
isWasm := tw.WasmProjectTinyGoJsUse() // Returns true if WASM project detected

// Dynamically switch to standard Go compiler
err := tw.SetTinyGoCompiler(false)
if err != nil {
    log.Printf("Error switching to Go compiler: %v", err)
}

// Switch back to TinyGo compiler (with validation)
err = tw.SetTinyGoCompiler(true) 
if err != nil {
    log.Printf("Error switching to TinyGo: %v", err)
    // Error might occur if TinyGo is not installed
}
```

### Automatic Project Detection

The library automatically detects if the current project should use WASM compilation:

```go
// Project structure that triggers WASM detection:
project/
├── *.wasm.go files present
├── web/ or public/ directories
├── wasm_exec.js file
└── go.mod with WASM-related dependencies

// The detection is dynamic and updates automatically
tw.WasmProjectTinyGoJsUse() // Returns current detection status
```

### Compiler Switching Examples

```go
// Example: Dynamic switching based on build mode
func setupCompiler(tw *tinywasm.TinyWasm, production bool) error {
    if production {
        // Use TinyGo for smaller WASM files in production
        return tw.SetTinyGoCompiler(true)
    } else {
        // Use standard Go for faster compilation in development
        return tw.SetTinyGoCompiler(false)
    }
}

// Example: Validation before switching
func safeCompilerSwitch(tw *tinywasm.TinyWasm, useTinyGo bool) error {
    if useTinyGo {
        // Verify TinyGo is available before switching
        err := tw.SetTinyGoCompiler(true)
        if err != nil {
            log.Printf("TinyGo not available, falling back to standard Go")
            return tw.SetTinyGoCompiler(false)
        }
        return nil
    }
    return tw.SetTinyGoCompiler(false)
}
```

### Compilation Differences

| Compiler | Build Speed | File Size | Use Case |
|----------|------------|-----------|----------|
| Go Standard | Faster | Larger (~1.6MB) | Development, debugging |
| TinyGo | Slower | Smaller (~170KB) | Production, web deployment |

### Benchmark System

TinyWasm includes an integrated benchmark system to compare compiler performance:

```bash
# Run unified benchmark (avoids code duplication)
cd benchmark/scripts
./unified-benchmark.sh

# Results show:
# - Build time comparison
# - File size comparison
# - Performance metrics
```

## Requirements

- Go 1.19 or higher
- TinyGo installed and in PATH (optional, for TinyGo compiler mode)
- Operating System: Windows, Linux, macOS

## License

This project is under the license specified in the LICENSE file.
