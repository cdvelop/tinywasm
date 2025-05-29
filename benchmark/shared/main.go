package main

import (
	"syscall/js"
)

// Shared WASM example that works with both Go standard and TinyGo
func main() {
	println("TinyWasm Benchmark Example")

	// Register functions for JavaScript interaction
	js.Global().Set("processText", js.FuncOf(processText))
	js.Global().Set("calculateNumbers", js.FuncOf(calculateNumbers))
	js.Global().Set("testStrings", js.FuncOf(testStrings))

	// Keep program running
	select {}
}

// processText handles text processing operations
func processText(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return "Error: No text provided"
	}

	input := args[0].String()

	// Basic text processing that works in both compilers
	words := splitWords(input)
	processed := make([]string, len(words))

	for i, word := range words {
		processed[i] = toUppercase(word)
	}

	return "Processed " + toString(len(processed)) + " words: " + joinWords(processed, " | ")
}

// calculateNumbers performs basic calculations
func calculateNumbers(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return "Error: Need 2 numbers"
	}

	num1 := parseNumber(args[0].String())
	num2 := parseNumber(args[1].String())

	if num1 == -1 || num2 == -1 {
		return "Error: Invalid numbers"
	}

	result := "Sum: " + toString(int(num1+num2)) +
		" | Product: " + toString(int(num1*num2)) +
		" | Division: " + toString(int(num1/num2))

	return result
}

// testStrings performs string operations
func testStrings(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return "Error: No string provided"
	}

	input := args[0].String()

	result := "Original: " + input +
		" | Length: " + toString(len(input)) +
		" | Uppercase: " + toUppercase(input) +
		" | Reversed: " + reverseString(input)

	return result
}

// Helper functions that work in both compilers

func splitWords(s string) []string {
	var words []string
	var current string

	for _, char := range s {
		if char == ' ' || char == '\t' || char == '\n' {
			if len(current) > 0 {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if len(current) > 0 {
		words = append(words, current)
	}

	return words
}

func toUppercase(s string) string {
	result := ""
	for _, char := range s {
		if char >= 'a' && char <= 'z' {
			result += string(char - 32)
		} else {
			result += string(char)
		}
	}
	return result
}

func toString(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	digits := ""
	for n > 0 {
		digits = string('0'+rune(n%10)) + digits
		n /= 10
	}

	if negative {
		digits = "-" + digits
	}

	return digits
}

func joinWords(words []string, separator string) string {
	if len(words) == 0 {
		return ""
	}

	result := words[0]
	for i := 1; i < len(words); i++ {
		result += separator + words[i]
	}

	return result
}

func parseNumber(s string) float64 {
	result := 0.0
	decimal := false
	divisor := 1.0

	for _, char := range s {
		if char == '.' {
			decimal = true
			continue
		}

		if char >= '0' && char <= '9' {
			digit := float64(char - '0')
			if decimal {
				divisor *= 10
				result += digit / divisor
			} else {
				result = result*10 + digit
			}
		} else {
			return -1 // Error
		}
	}

	return result
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
