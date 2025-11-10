package tinywasm

// ToolExecutor defines how a tool should be executed
// Channel accepts string messages (no binary data in tinywasm)
type ToolExecutor func(args map[string]any, progress chan<- any)

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
			Execute: func(args map[string]any, progress chan<- any) {
				modeValue, ok := args["mode"]
				if !ok {
					progress <- "missing required parameter 'mode'. Use L, M, or S"
					return
				}

				mode, ok := modeValue.(string)
				if !ok {
					progress <- "parameter 'mode' must be a string (L, M, or S)"
					return
				}

				// Create string-only channel for Change method
				// and forward messages to the any channel
				stringChan := make(chan string, 10)
				done := make(chan bool)

				go func() {
					for msg := range stringChan {
						progress <- msg
					}
					done <- true
				}()

				// Domain-specific logic: Change WASM compilation mode
				w.Change(mode, stringChan)
				close(stringChan)
				<-done
			},
		},
		{
			Name:        "wasm_get_size",
			Description: "Get current WASM file size and comparison across all three modes (LARGE/MEDIUM/SMALL) to help decide optimal size/feature tradeoff for production.",
			Parameters:  []ParameterMetadata{},
			Execute: func(args map[string]any, progress chan<- any) {
				// TODO: Implement size retrieval from TinyWasm
				progress <- "Current WASM size: [not implemented yet]"
			},
		},
	}
}
