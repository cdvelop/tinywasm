package tinywasm

import "fmt"

// ToolExecutor defines how a tool should be executed
type ToolExecutor func(args map[string]any, progress func(msgs ...any)) error

// ToolMetadata provides MCP tool configuration metadata
type ToolMetadata struct {
	Name        string
	Description string
	Parameters  []ParameterMetadata
	Execute     ToolExecutor // Execution function
}

// ParameterMetadata describes a tool parameter
type ParameterMetadata struct {
	Name        string
	Description string
	Required    bool
	Type        string
	EnumValues  []string
	Default     any
}

// GetMCPToolsMetadata returns metadata for all TinyWasm MCP tools
func (w *TinyWasm) GetMCPToolsMetadata() []ToolMetadata {
	return []ToolMetadata{
		{
			Name: "wasm_set_mode",
			Description: "Change WebAssembly compilation mode for the Go frontend. " +
				"L=LARGE (Go std, ~2MB, full features), " +
				"M=MEDIUM (TinyGo debug, ~500KB, most features), " +
				"S=SMALL (TinyGo compact, ~200KB, minimal). " +
				"Use single letter shortcuts: L, M, or S.",
			Parameters: []ParameterMetadata{
				{
					Name:        "mode",
					Description: "Compilation mode: L (large), M (medium), or S (small)",
					Required:    true,
					Type:        "string",
					EnumValues:  []string{"L", "M", "S"},
				},
			},
			Execute: func(args map[string]any, progress func(msgs ...any)) error {
				modeValue, ok := args["mode"]
				if !ok {
					return fmt.Errorf("missing required parameter 'mode'. Use L, M, or S")
				}

				mode, ok := modeValue.(string)
				if !ok {
					return fmt.Errorf("parameter 'mode' must be a string (L, M, or S)")
				}

				// Domain-specific logic: Change WASM compilation mode
				w.Change(mode, progress)
				return nil
			},
		},
		{
			Name:        "wasm_recompile",
			Description: "Force immediate WASM recompilation of the Go frontend code with current mode (useful after code changes or mode switch to see results immediately).",
			Parameters:  []ParameterMetadata{},
			Execute: func(args map[string]any, progress func(msgs ...any)) error {
				// Domain-specific logic: Force recompilation
				if err := w.RecompileMainWasm(); err != nil {
					return fmt.Errorf("recompilation failed: %w", err)
				}
				progress("WASM recompiled successfully")
				return nil
			},
		},
		{
			Name:        "wasm_get_size",
			Description: "Get current WASM file size and comparison across all three modes (LARGE/MEDIUM/SMALL) to help decide optimal size/feature tradeoff for production.",
			Parameters:  []ParameterMetadata{},
			Execute: func(args map[string]any, progress func(msgs ...any)) error {
				// TODO: Implement size retrieval from TinyWasm
				progress("Current WASM size: [not implemented yet]")
				return nil
			},
		},
	}
}
