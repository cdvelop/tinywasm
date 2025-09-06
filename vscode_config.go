package tinywasm

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// VisualStudioCodeWasmEnvConfig automatically creates and configures VS Code settings for WASM development.
// This method resolves the "could not import syscall/js" error by setting proper environment variables
// in .vscode/settings.json file. On Windows, the .vscode directory is made hidden for a cleaner project view.
// This configuration enables VS Code's Go extension to properly recognize WASM imports and provide
// accurate IntelliSense, error detection, and code completion for syscall/js and other WASM-specific packages.
func (w *TinyWasm) VisualStudioCodeWasmEnvConfig() { // Create .vscode directory if it doesn't exist
	// Use AppRootDir from Config (falls back to "." by default)
	vscodeDir := filepath.Join(w.AppRootDir, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		if w.Logger != nil {
			w.Logger("Warning: Error creating .vscode directory:", err)
		}
		return
	}

	// Make .vscode directory hidden on Windows for cleaner project view
	if runtime.GOOS == "windows" {
		w.makeDirectoryHiddenWindows(vscodeDir)
	}

	// Configure settings.json
	settingsPath := filepath.Join(vscodeDir, "settings.json")

	var settings map[string]any

	// Load existing settings if file exists
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			settings = make(map[string]any)
		}
	} else {
		settings = make(map[string]any)
	} // Configure gopls (Go language server) for WASM development without affecting tests
	// This provides proper IntelliSense for syscall/js and WASM packages
	settings["gopls"] = map[string]any{
		"env": map[string]string{
			"GOOS":   "js",
			"GOARCH": "wasm",
		},
	}

	// Alternative: Use go.toolsEnvVars but exclude specific tools that should use native env
	// This gives better IntelliSense while allowing tests to run normally
	settings["go.alternateTools"] = map[string]string{
		"go": "go", // Use system Go for testing and building
	}
	// Write updated settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		if w.Logger != nil {
			w.Logger("Warning: marshaling VS Code settings:", err)
		}
		return
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		if w.Logger != nil {
			w.Logger("Warning: writing VS Code settings:", err)
		}
		return
	}
}

// makeDirectoryHiddenWindows makes a directory hidden on Windows using the attrib command.
// This provides a cleaner project view by hiding the .vscode configuration directory.
// Uses the most compatible Windows command that works across all Windows versions.
// If the command fails, it only logs a warning and continues normally since this is not critical.
func (w *TinyWasm) makeDirectoryHiddenWindows(dirPath string) {
	// Use attrib +h command - most compatible across Windows versions (Windows XP+)
	// This command is built into all Windows versions and doesn't require PowerShell
	cmd := exec.Command("cmd", "/c", "attrib", "+h", dirPath)
	if err := cmd.Run(); err != nil {
		if w.Logger != nil {
			w.Logger("Warning: Could not make .vscode directory hidden on Windows:", err)
		}
		// Continue normally - this is not a critical operation for WASM development
	}
}
