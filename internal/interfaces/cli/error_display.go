package cli

import (
	"fmt"
	"os"
)

// DisplayError prints a standardized error message to stderr.
// Format: Error: {message}
func DisplayError(message string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
}

// DisplayErrorf prints a formatted error message to stderr.
func DisplayErrorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// DisplayErrorWithSuggestion prints an error with a suggestion to stderr.
// Format: Error: {message}
//
//	Suggestion: {suggestion}
func DisplayErrorWithSuggestion(message, suggestion string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
	if suggestion != "" {
		fmt.Fprintf(os.Stderr, "Suggestion: %s\n", suggestion)
	}
}

// DisplayErrorFull prints a full error with details and suggestion to stderr.
// Format: Error: {message}
//
//	Details: {details}
//	Suggestion: {suggestion}
func DisplayErrorFull(message, details, suggestion string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
	if details != "" {
		fmt.Fprintf(os.Stderr, "Details: %s\n", details)
	}
	if suggestion != "" {
		fmt.Fprintf(os.Stderr, "Suggestion: %s\n", suggestion)
	}
}

// DisplayInfo prints an informational message to stderr.
func DisplayInfo(message string) {
	fmt.Fprintf(os.Stderr, "[INFO] %s\n", message)
}

// ExitWithError exits with code 1 (general error) after printing the error.
func ExitWithError(message string) {
	DisplayError(message)
	os.Exit(1)
}

// ExitWithErrorf exits with code 1 after printing a formatted error.
func ExitWithErrorf(format string, args ...interface{}) {
	DisplayErrorf(format, args...)
	os.Exit(1)
}

// ExitWithValidationError exits with code 2 (validation failure).
func ExitWithValidationError(message string) {
	DisplayError(message)
	os.Exit(2)
}

// ExitWithExecutionError exits with code 3 (execution failure).
func ExitWithExecutionError(message string) {
	DisplayError(message)
	os.Exit(3)
}
