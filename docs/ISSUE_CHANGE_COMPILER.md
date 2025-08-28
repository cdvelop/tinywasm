# TinyWasm Compiler Mode Implementation

## Executive Summary

**Objective**: Implement a 3-mode compiler system for TinyWasm with DevTUI integration, replacing the current boolean `TinyGoCompiler` with a comprehensive build mode system.

**Modes**: 
- `"c"` (Coding/Development) - Go standard compiler for fast iteration
- `"d"` (Debug) ---

## Final Implementation Questions

### � CONFIRMED FINAL DECISIONS

1. **Value() Method**: Return shortcut ("c", "d", "p") - represents actual field value, not description ✅
2. **Config Constructor**: Create `NewConfig()` with default shortcuts for explicit initialization ✅  
3. **Recompile Error Handling**: Return messages with appropriate keywords ("Warning:", "Error:") for automatic MessageType detection via `messagetype.DetectMessageType()` ✅

### Implementation Specifications

#### FieldHandler Implementation
```go
func (w *TinyWasm) Label() string {
    return "Build Mode: c, d, p"
}

func (w *TinyWasm) Value() string {
    return w.getCurrentMode() // Returns "c", "d", or "p" (actual value)
}

func (w *TinyWasm) Editable() bool {
    return true
}

func (w *TinyWasm) Change(newValue any) (string, error) {
    modeStr, ok := newValue.(string)
    if !ok {
        return "", Err(D.Invalid, D.Input, D.Type)
    }
    
    // Validate mode
    if err := w.validateMode(modeStr); err != nil {
        return "", err
    }
    
    // Check TinyGo installation for debug/production modes
    if w.requiresTinyGo(modeStr) && !w.tinyGoInstalled {
        if err := w.handleTinyGoMissing(); err != nil {
            return "", err
        }
    }
    
    // Update active builder
    w.updateCurrentBuilder(modeStr)
    
    // Auto-recompile with appropriate message format for MessageType detection
    if err := w.recompileMainWasm(); err != nil {
        // Return warning message - MessageType will detect "Warning:" keyword
        warningMsg := Translate("Warning:", D.Auto, D.Compilation, D.Failed, err.Error())
        return warningMsg, nil // Don't fail the mode change
    }
    
    return w.getSuccessMessage(modeStr), nil
}

func (w *TinyWasm) Timeout() time.Duration {
    return 60 * time.Second
}
```

#### Config Constructor Pattern
```go
// NewConfig creates a TinyWasm Config with sensible defaults
func NewConfig() *Config {
    return &Config{
        CodingShortcut:     "c",
        DebuggingShortcut:  "d",
        ProductionShortcut: "p",
        TinyGoCompiler:     false, // Default to fast Go compilation
        FrontendPrefix:     []string{"f.", "front."},
    }
}
```

#### Message Format for MessageType Detection
```go
func (w *TinyWasm) getSuccessMessage(mode string) string {
    switch mode {
    case w.Config.CodingShortcut:
        return Translate(D.Switching, D.Mode, D.Coding)      // "Switching Mode Coding"
    case w.Config.DebuggingShortcut:
        return Translate(D.Switching, D.Mode, D.Debugging)   // "Switching Mode Debugging"
    case w.Config.ProductionShortcut:
        return Translate(D.Switching, D.Mode, D.Production)  // "Switching Mode Production"
    default:
        return Translate(D.Invalid, D.Mode)
    }
}

// Error messages formatted for MessageType detection
func (w *TinyWasm) handleTinyGoMissing() error {
    if err := w.installTinyGo(); err != nil {
        // Format: "Error: Cannot Install TinyGo: details"
        return Err("Error:", D.Cannot, D.Install, "TinyGo", err.Error())
    }
    return nil
}
```

1. **TinyString Import**: `"github.com/cdvelop/tinystring"` ✅
2. **Required Dictionary Terms**: Coding, Debugging, Production, Switching, Mode, Valid, Modes, Install, Installation, Implemented, Auto, Compilation, Failed ✅  
3. **Method Signatures**:
   - `Label() string` → `"Build Mode: c, d, p"`
   - `Value() string` → Current shortcut ("c", "d", or "p")
   - `Editable() bool` → `true`
   - `Change(newValue any) (string, error)` → Validates, switches, recompiles
   - `Timeout() time.Duration` → `60 * time.Second`

4. **Builder Architecture**: 3 builders + 1 activeBuilder pointer for state management ✅
5. **No Backward Compatibility**: Complete refactor of `SetTinyGoCompiler` ✅yGo with debug-friendly optimizations  
- `"p"` (Production) - TinyGo with maximum size optimization

**Integration**: Full DevTUI `FieldHandler` interface implementation with multilingual support via TinyString dictionary.

---

## Technical Requirements

### 1. FieldHandler Interface Implementation

```go
// TinyWasm will implement DevTUI FieldHandler interface
func (w *TinyWasm) Label() string                       // "Build Mode: c, d, p"
func (w *TinyWasm) Value() string                       // Current mode: "c", "d", or "p"  
func (w *TinyWasm) Editable() bool                      // true - user can edit
func (w *TinyWasm) Change(newValue any) (string, error) // Validates and switches mode
func (w *TinyWasm) Timeout() time.Duration              // 1 minute for all modes
```

**Display Format**:
- Label: `"Build Mode: c, d, p"`
- Value: Current mode (`"c"`, `"d"`, or `"p"`)
- Editable: `true` (allows user input)

### 2. Struct Modifications

#### TinyWasm Struct Changes
```go
type TinyWasm struct {
    *Config
    ModulesFolder string
    mainInputFile string

    // RENAME & ADD: 4 builders for complete mode coverage
    builderCoding     *gobuild.GoBuild // Go standard - fast compilation
    builderDebug      *gobuild.GoBuild // TinyGo debug - easier debugging  
    builderProduction *gobuild.GoBuild // TinyGo production - smallest size
    activeBuilder     *gobuild.GoBuild // Current active builder

    // EXISTING: Keep for installation detection (no compilerMode needed - activeBuilder handles state)
    tinyGoCompiler  bool // Enable TinyGo compiler
    wasmProject     bool
    tinyGoInstalled bool

    // ... rest unchanged
}
```

#### Config Struct Additions
```go
type Config struct {
    // ... existing fields ...

    // NEW: Shortcut configuration (default: "c", "d", "p")
    CodingShortcut    string // default "c" 
    DebuggingShortcut string // default "d"
    ProductionShortcut string // default "p"
}
```

### 3. Compilation Mode Configurations

#### Mode Specifications
Based on [optimizing-binaries.md](benchmark/optimizing-binaries.md):

**"c" (Coding/Development)**:
- Compiler: Go standard
- Environment: `GOOS=js GOARCH=wasm`
- Args: `["-tags", "dev"]`
- Purpose: Fast compilation for development iteration
- **Status**: ✅ Already implemented

**"d" (Debug)**:
- Compiler: TinyGo
- Args: `["-target", "wasm", "-opt=1"]`
- Features: Easier debugging, keep debug symbols
- Purpose: Debug-friendly TinyGo compilation

**"p" (Production)**:
- Compiler: TinyGo  
- Args: `["-target", "wasm", "-opt=z", "-no-debug", "-panic=trap"]`
- Purpose: Smallest binary size for production deployment

### 4. Method Renaming and Signatures

#### Primary Method: Change (DevTUI FieldHandler)
```go
// RENAME: SetTinyGoCompiler -> Change (implements FieldHandler interface)
func (w *TinyWasm) Change(newValue any) (string, error) {
    // 1. Validate input string (only "c", "d", "p" allowed, using Config shortcuts)
    // 2. Check TinyGo installation for "d"/"p" modes
    // 3. Call updateCurrentBuilder(mode)
    // 4. Auto-recompile main.wasm.go if exists
    // 5. Return multilingual success message using TinyString Translate() function
}
```

#### Supporting Methods
```go
// RENAME: updateBuilderConfig -> updateCurrentBuilder
func (w *TinyWasm) updateCurrentBuilder(mode string) {
    // 1. Cancel any ongoing compilation
    if w.activeBuilder != nil {
        w.activeBuilder.Cancel()
    }

    // 2. Set activeBuilder based on mode
    switch mode {
    case w.Config.CodingShortcut:     // "c"
        w.activeBuilder = w.builderCoding
    case w.Config.DebuggingShortcut:  // "d" 
        w.activeBuilder = w.builderDebug
    case w.Config.ProductionShortcut: // "p"
        w.activeBuilder = w.builderProduction
    default:
        w.activeBuilder = w.builderCoding // fallback to coding mode
    }
}

// NEW: Installation handler (placeholder for future)
func (w *TinyWasm) installTinyGo() error {
    // Placeholder method - will show error for now
    // Future: implement automatic TinyGo installation
    return Err("TinyGo", D.Installation, D.Not, D.Implemented)
}
```

### 5. Builder Initialization Refactoring

#### Updated initializeBuilder Method
```go
func (w *TinyWasm) initializeBuilder() {
    rootFolder := w.Config.WebFilesRootRelative
    subFolder := w.Config.WebFilesSubRelative
    mainFilePath := path.Join(rootFolder, w.mainInputFile)
    outFolder := path.Join(rootFolder, subFolder)

    // Base configuration shared by all builders
    baseConfig := gobuild.Config{
        MainInputFileRelativePath: mainFilePath,
        OutName:      "main",
        Extension:    ".wasm", 
        OutFolderRelativePath:    outFolder,
        Writer:       w.Writer,
        Timeout:      60 * time.Second, // 1 minute for all modes
        Callback:     w.Callback,
    }

    // Configure Coding builder (Go standard)
    codingConfig := baseConfig
    codingConfig.Command = "go"
    codingConfig.Env = []string{"GOOS=js", "GOARCH=wasm"}
    codingConfig.CompilingArguments = func() []string {
        args := []string{"-tags", "dev"}
        if w.CompilingArguments != nil {
            args = append(args, w.CompilingArguments()...)
        }
        return args
    }
    w.builderCoding = gobuild.New(&codingConfig)

    // Configure Debug builder (TinyGo debug-friendly)
    debugConfig := baseConfig
    debugConfig.Command = "tinygo"
    debugConfig.CompilingArguments = func() []string {
        args := []string{"-target", "wasm", "-opt=1"} // Keep debug symbols
        if w.CompilingArguments != nil {
            args = append(args, w.CompilingArguments()...)
        }
        return args
    }
    w.builderDebug = gobuild.New(&debugConfig)

    // Configure Production builder (TinyGo optimized)
    prodConfig := baseConfig
    prodConfig.Command = "tinygo"
    prodConfig.CompilingArguments = func() []string {
        args := []string{"-target", "wasm", "-opt=z", "-no-debug", "-panic=trap"}
        if w.CompilingArguments != nil {
            args = append(args, w.CompilingArguments()...)
        }
        return args
    }
    w.builderProduction = gobuild.New(&prodConfig)

    // Set initial mode and active builder (default to coding mode)
    w.activeBuilder = w.builderCoding // Default: fast development
}
```

### 6. State Management

#### Default Mode Detection
```go
func (w *TinyWasm) getCurrentMode() string {
    // Determine current mode based on activeBuilder
    switch w.activeBuilder {
    case w.builderCoding:
        return w.Config.CodingShortcut     // "c"
    case w.builderDebug:
        return w.Config.DebuggingShortcut  // "d"
    case w.builderProduction:
        return w.Config.ProductionShortcut // "p"
    default:
        return w.Config.CodingShortcut     // fallback
    }
}

func (w *TinyWasm) validateMode(mode string) error {
    validModes := []string{
        w.Config.CodingShortcut,    // "c"
        w.Config.DebuggingShortcut, // "d" 
        w.Config.ProductionShortcut, // "p"
    }
    
    for _, valid := range validModes {
        if mode == valid {
            return nil
        }
    }
    
    return Err(D.Invalid, D.Mode, mode, D.Valid, D.Modes, validModes)
}
```

### 7. Multilingual Message System (TinyString Integration)

#### Required Dictionary Additions
Add these terms to TinyString dictionary if not present:
- `Coding` - for development mode
- `Debugging` - for debug mode  
- `Production` - for production mode
- `Switching` - for mode change messages
- `Mode` - for mode references
- `Valid` - for validation messages
- `Modes` - plural of mode
- `Install` - for installation messages
- `Installation` - for installation process
- `Implemented` - for not implemented features
- `Input` - for input validation
- `Type` - for type validation
- `Auto` - for automatic processes
- `Compilation` - for compilation processes
- `Failed` - for failed operations

#### Message Implementation
```go
func (w *TinyWasm) getSuccessMessage(mode string) string {
    // Import: "github.com/cdvelop/tinystring"
    switch mode {
    case w.Config.CodingShortcut:
        return Translate(D.Switching, D.Mode, D.Coding)      // "Switching Mode Coding"
    case w.Config.DebuggingShortcut:
        return Translate(D.Switching, D.Mode, D.Debugging)   // "Switching Mode Debugging"  
    case w.Config.ProductionShortcut:
        return Translate(D.Switching, D.Mode, D.Production)  // "Switching Mode Production"
    default:
        return Translate(D.Invalid, D.Mode)                  // "Invalid Mode"
    }
}
```

### 8. Error Handling and Fallbacks

#### TinyGo Installation Check
```go
func (w *TinyWasm) requiresTinyGo(mode string) bool {
    return mode == w.Config.DebuggingShortcut || mode == w.Config.ProductionShortcut
}

func (w *TinyWasm) handleTinyGoMissing(mode string) error {
    // Try installation (placeholder for now)
    if err := w.installTinyGo(); err != nil {
        return Err(D.Cannot, D.Install, "TinyGo", err.Error())
    }
    
    // Re-verify installation
    if err := w.VerifyTinyGoInstallation(); err != nil {
        return err
    }
    
    w.tinyGoInstalled = true
    return nil
}
```

#### Timeout Configuration
- **All modes**: 60 seconds (1 minute)
- **Reasoning**: TinyGo can be slow, but user needs reasonable feedback
- **Consistent**: Same timeout regardless of mode complexity

---

## Implementation Plan

### Phase 1: Core Structure Changes
1. **Rename builder fields** in TinyWasm struct
2. **Add compilerMode field** and Config shortcuts
3. **Update initializeBuilder()** with 3-mode configuration
4. **Rename SetTinyGoCompiler() to Change()**

### Phase 2: DevTUI Integration  
1. **Implement FieldHandler interface** methods
2. **Add multilingual messages** via TinyString
3. **Create validation logic** for mode shortcuts
4. **Test DevTUI field integration**

### Phase 3: Error Handling & Polish
1. **Implement TinyGo installation placeholder**
2. **Add comprehensive error messages**
3. **Update existing tests** for new mode system
4. **Performance testing** for each mode

### Phase 4: Documentation
1. **Update README.md** with new public methods
2. **Add mode comparison table** (speed vs size vs debuggability)  
3. **Usage examples** for each mode
4. **Migration guide** from boolean TinyGoCompiler

---

## Test Strategy

### Unit Tests to Update/Add
1. **Modify existing compilation tests** to test all 3 modes
2. **Add FieldHandler interface tests** (Label, Value, Editable, Change, Timeout)
3. **Add validation tests** for mode shortcuts ("c", "d", "p")
4. **Add fallback tests** when TinyGo not installed
5. **Add performance comparison tests** (optional)

### Integration Tests
1. **DevTUI integration test** - field display and interaction
2. **Auto-recompilation test** - mode changes trigger recompile
3. **Error handling test** - invalid modes, missing TinyGo

---

## Open Questions & Recommendations

###  ALTERNATIVE: Config Shortcut Defaults

**Implementation Priority**: High - Core functionality enhancement  
**Estimated Effort**: 2-3 days development + testing  
**Dependencies**: TinyString dictionary updates, DevTUI FieldHandler interface

---

**Next Steps**: 
1. ✅ **All decisions confirmed** - ready for implementation
2. **Add TinyString dictionary terms** for multilingual support
3. **Implement NewConfig() constructor** with default shortcuts
4. **Begin TinyWasm refactoring** with 3-mode system
5. **Update tests** for new FieldHandler interface
6. **Message format validation** for automatic MessageType detection

**Message Integration with DevTUI**: 
- Success messages: Auto-detected as `messagetype.Info` or `messagetype.Success`
- Warning messages: Prefixed with "Warning:" → `messagetype.Warning`
- Error messages: Prefixed with "Error:" → `messagetype.Error`
- DevTUI will automatically apply appropriate colors and styling based on MessageType detection
