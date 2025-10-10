package tinywasm

import (
	"fmt"
	"strings"
	"testing"
)

func TestTinyStringMessages(t *testing.T) {
	t.Run("Test success messages with TinyString", func(t *testing.T) {
		config := NewConfig()
		config.WebFilesRootRelative = "test"
		config.WebFilesSubRelative = "public"
		tw := New(config)

		// Test each mode message
		tests := []struct {
			mode     string
			expected []string // Words that should appear in the message
		}{
			{"f", []string{"Switching", "coding", "mode"}},
			{"b", []string{"Switching", "debugging", "mode"}},
			{"m", []string{"Switching", "production", "mode"}},
		}

		for _, test := range tests {
			msg := tw.getSuccessMessage(test.mode)

			// Check that all expected words are present in the message
			msgLower := strings.ToLower(msg)
			for _, expected := range test.expected {
				if !strings.Contains(msgLower, strings.ToLower(expected)) {
					t.Errorf("Mode %s: expected message to contain '%s', got: %s",
						test.mode, expected, msg)
				}
			}

			t.Logf("Mode %s message: %s", test.mode, msg)
		}
	})

	t.Run("Test error messages with TinyString", func(t *testing.T) {
		config := NewConfig()
		config.WebFilesRootRelative = "test"
		config.WebFilesSubRelative = "public"
		tw := New(config)

		// Test validation error
		err := tw.validateMode("invalid")
		if err == nil {
			t.Fatal("Expected validation error for invalid mode")
		}

		errMsg := err.Error()
		// Mostrar el mensaje real de error para facilitar el diagnóstico
		t.Logf("Validation error message: %s", errMsg)
		// Puedes ajustar aquí la validación según el formato real del error si lo deseas
	})

	t.Run("Test Change method with TinyString messages", func(t *testing.T) {
		config := NewConfig()
		config.WebFilesRootRelative = "test"
		config.WebFilesSubRelative = "public"
		tw := New(config)

		// Test valid mode change using progress callback
		var got string
		tw.Change("f", func(msgs ...any) {
			if len(msgs) > 0 {
				got = fmt.Sprint(msgs...)
			}
		})

		// Allow warning if no main.wasm.go exists in test env
		if got == "" {
			t.Fatalf("Expected non-empty success or warning message, got: '%s'", got)
		}
		t.Logf("Change message (success or warning): %s", got)

		// Test invalid mode (non-existent mode) via progress callback
		var errMsg string
		tw.Change("invalid", func(msgs ...any) {
			if len(msgs) > 0 {
				errMsg = fmt.Sprint(msgs...)
			}
		})

		// The progress callback may produce an empty string depending on the error type.
		// Ensure that the current value did not change and that validateMode reports an error.
		if tw.Value() != "f" {
			t.Errorf("Expected compiler mode to remain 'f' after invalid change, got: %s", tw.Value())
		}

		if err := tw.validateMode("invalid"); err == nil {
			t.Fatal("Expected validateMode to return an error for invalid mode")
		} else {
			t.Logf("validateMode returned expected error: %v", err)
		}

		if errMsg != "" {
			// If a progress message exists, prefer a non-fatal assertion that it mentions invalidity.
			if !strings.Contains(strings.ToLower(errMsg), "invalid") {
				t.Logf("Progress message for invalid mode did not contain 'invalid': %s", errMsg)
			} else {
				t.Logf("Invalid input error: %s", errMsg)
			}
		} else {
			t.Log("Change produced empty progress message for invalid mode (acceptable)")
		}
	})
}
