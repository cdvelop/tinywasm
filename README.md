# tinywasm

`tinywasm` is a Go package designed to be integrated into development kits, providing specific functionality for compiling Go files to WebAssembly (WASM) using the TinyGo compiler in an optimized way.

## Features

- **Development kit integration**: Designed to be embedded in broader development tools
- **Automatic compilation**: Automatically detects changes in `.wasm.go` files and compiles them to WebAssembly
- **Module support**: Handles both main files (`main.wasm.go`) and independent modules
- **TinyGo by default**: Uses TinyGo as the default compiler to generate smaller WASM files
- **JavaScript file management**: Automatically provides necessary `wasm_exec.js` files
- **Flexible configuration**: Allows customization of output directories and compilation settings
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
    // Configure TinyWasm for integration
    config := &tinywasm.WasmConfig{
        WebFilesFolder: func() (string, string) { 
            return "web", "public" 
        },
        Log: os.Stdout, // Log output for integration with external systems
    }

    // Create TinyWasm instance
    tw := tinywasm.New(config)

    // Integrate into development kit event system
    err := tw.NewFileEvent("main.wasm.go", ".go", "web/public/wasm/main.wasm.go", "write")
    if err != nil {
        // Handle error in development kit context
        panic(err)
    }
}
```

### Usage in File Watcher

This package is ideal for integration into file monitoring systems:

```go
// Example integration in a watcher
func handleFileChange(filePath, event string) {
    if strings.HasSuffix(filePath, ".wasm.go") {
        err := tinyWasm.NewFileEvent(
            filepath.Base(filePath),
            ".go",
            filePath,
            event,
        )
        if err != nil {
            log.Printf("Error compiling WASM: %v", err)
        }
    }
}
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
}
```

## Use Cases

### Integration in Development Tools
- **Hot reload**: Automatic recompilation during development
- **Build systems**: Integration into build pipelines
- **IDEs and editors**: Support for development extensions
- **Development servers**: On-demand compilation

### Compatible Development Kits
This package can be easily integrated into:
- Automated build systems
- Web development tools
- Go development frameworks
- Development servers with hot reload

## File Structure

The package expects a specific file structure:

```
project/
├── web/
│   └── public/
│       └── wasm/
│           ├── main.wasm.go     # Main WASM file
│           ├── main.wasm        # Compiled file (generated)
│           └── wasm_exec.js     # JS runtime (generated)
└── modules/
    └── users/
        └── wasm/
            ├── users.wasm.go    # WASM module
            └── users.wasm       # Compiled module (generated)
```

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

### Core Methods

#### `New(config *WasmConfig) *TinyWasm`
Creates a new TinyWasm instance with the provided configuration.

#### `NewFileEvent(fileName, extension, filePath, event string) error`
Processes file events for WASM compilation.
- `fileName`: Name of the file (e.g., "main.wasm.go")
- `extension`: File extension (e.g., ".go")
- `filePath`: Full path to the file
- `event`: Type of file event ("create", "remove", "write", "rename")

#### `OutputPathMainFileWasm() string`
Returns the output path for the main WASM file (e.g: "web/public/wasm/main.wasm").

#### `UnobservedFiles() []string`
Returns files that should not be watched for changes (e.g: "main.wasm").

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
Extracts module name from file path (e.g: extracts "users" from "modules/users/wasm/users.wasm.go").

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
// main.wasm.go
package main

import "syscall/js"

func hello(this js.Value, args []js.Value) any {
    return "Hello from TinyWasm!"
}

func main() {
    js.Global().Set("hello", js.FuncOf(hello))
    select {} // Keep the program alive
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

## Requirements

- Go 1.19 or higher
- TinyGo installed and in PATH
- Operating System: Windows, Linux, macOS

## License

This project is under the license specified in the LICENSE file.
