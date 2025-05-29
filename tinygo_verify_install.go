package tinywasm

import (
	"fmt"
	"os/exec"
	"strings"
)

// VerifyTinyGoInstallation checks if TinyGo is properly installed
func (t *TinyWasm) VerifyTinyGoInstallation() error {
	_, err := exec.LookPath("tinygo")
	if err != nil {

		// install TinyGo from https://tinygo.org/getting-started/installation/
		return fmt.Errorf("TinyGo not found in PATH: %v", err)
	}
	return nil
}

// GetTinyGoVersion returns the installed TinyGo version
func (t *TinyWasm) GetTinyGoVersion() (string, error) {
	cmd := exec.Command("tinygo", "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get TinyGo version: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}
