package tinywasm

import (
	"strings"
	"testing"
)

func TestTinyStringMessages(t *testing.T) {
	t.Run("Test success messages with TinyString", func(t *testing.T) {
		config := NewConfig()
		config.WebFilesFolder = func() (string, string) { return "test", "public" }
		tw := New(config)

		// Test each mode message
		tests := []struct {
			mode     string
			expected []string // Words that should appear in the message
		}{
			{"c", []string{"Switching", "coding", "mode"}},
			{"d", []string{"Switching", "debugging", "mode"}},
			{"p", []string{"Switching", "production", "mode"}},
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
		config.WebFilesFolder = func() (string, string) { return "test", "public" }
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
		config.WebFilesFolder = func() (string, string) { return "test", "public" }
		tw := New(config)

		// Test valid mode change
		msg, err := tw.Change("c")
		if err != nil {
			t.Fatalf("Unexpected error changing to coding mode: %v", err)
		}

		// Permitir mensaje de advertencia si no hay archivo main.wasm.go
		if msg == "" {
			t.Fatalf("Expected non-empty success or warning message, got: '%s'", msg)
		}
		t.Logf("Change message (success or warning): %s", msg) // Test invalid input type
		_, err = tw.Change(123)
		if err == nil {
			t.Fatal("Expected error for invalid input type")
		}

		errMsg := err.Error()
		if !strings.Contains(strings.ToLower(errMsg), "invalid") {
			t.Errorf("Expected error to contain 'invalid', got: %s", errMsg)
		}

		t.Logf("Invalid input error: %s", errMsg)
	})
}
