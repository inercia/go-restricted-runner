// Package common provides utility functions and shared components.
package common

import (
	"os/exec"
	"runtime"
)

// CheckExecutableExists checks if a command is available in the system PATH.
//
// Parameters:
//   - executableName: The name of the executable to check
//
// Returns:
//   - true if the executable exists and is accessible, false otherwise
func CheckExecutableExists(executableName string) bool {
	_, err := exec.LookPath(executableName)
	return err == nil
}

// CheckOSMatches checks if the current operating system matches the required OS.
//
// Parameters:
//   - requiredOS: The required operating system (e.g., "darwin", "linux", "windows")
//     Can be empty to skip OS check.
//
// Returns:
//   - true if the current OS matches the required OS or if requiredOS is empty,
//     false otherwise
func CheckOSMatches(requiredOS string) bool {
	// If no OS is specified, consider it a match
	if requiredOS == "" {
		return true
	}

	// Check if the current OS matches the required OS
	return runtime.GOOS == requiredOS
}
