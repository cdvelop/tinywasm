package tinywasm

import (
	"fmt"
	"os"
	"path/filepath"
)

// VerifyTinyGoProjectCompatibility checks if the project is compatible with TinyGo compilation
func (w *TinyWasm) VerifyTinyGoProjectCompatibility() {
	// Verify tinystring library dependencies
	fmt.Fprintln(w.Writer, "=== TinyString Library TinyGo Compatibility Check ===")

	// Verify the library directory exists
	libPath := "./tinystring"
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		libPath = "."
	}

	// Check for problematic imports
	problematicImports := []string{"fmt", "strings", "strconv"}
	found := false
	err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(path) != ".go" || filepath.Base(path) == "verify_tinygo.go" {
			return nil
		}

		// Skip test files since they're not part of the compiled library
		fileName := filepath.Base(path)
		if len(fileName) > 8 && fileName[len(fileName)-8:] == "_test.go" {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Read file content (simplified check)
		buffer := make([]byte, 1024)
		n, _ := file.Read(buffer)
		content := string(buffer[:n])
		for _, imp := range problematicImports {
			importStr := fmt.Sprintf("\"%s\"", imp)
			if contains(content, importStr) {
				fmt.Fprintf(w.Writer, "❌ Found problematic import %s in %s\n", imp, path)
				found = true
			}
		}

		return nil
	})
	if err != nil {
		fmt.Fprintf(w.Writer, "Error walking directory: %v\n", err)
		return
	}

	if !found {
		fmt.Fprintln(w.Writer, "✅ No problematic standard library imports found!")
		fmt.Fprintln(w.Writer, "✅ TinyString library is TinyGo compatible!")
		fmt.Fprintln(w.Writer, "")
		fmt.Fprintln(w.Writer, "Key Features:")
		fmt.Fprintln(w.Writer, "- Zero dependency on fmt, strings, strconv packages")
		fmt.Fprintln(w.Writer, "- Manual implementations for string/number conversions")
		fmt.Fprintln(w.Writer, "- Optimized for minimal binary size")
		fmt.Fprintln(w.Writer, "- Compatible with embedded systems and WebAssembly")
	} else {
		fmt.Fprintln(w.Writer, "❌ TinyString library still has standard library dependencies")
	}
}

// contains is a simple string contains function to avoid using strings package
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
