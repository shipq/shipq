package cli

import (
	"fmt"
	"os"
)

// Fatal prints a message to stderr and exits with code 1.
func Fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

// FatalErr prints an error message with details to stderr and exits with code 1.
func FatalErr(msg string, err error) {
	fmt.Fprintf(os.Stderr, "error: %s: %v\n", msg, err)
	os.Exit(1)
}

// Info prints an informational message to stdout.
func Info(msg string) {
	fmt.Println(msg)
}

// Infof prints a formatted informational message to stdout.
func Infof(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

// Success prints a success message to stdout.
func Success(msg string) {
	fmt.Println("✓", msg)
}

// Successf prints a formatted success message to stdout.
func Successf(format string, args ...any) {
	fmt.Printf("✓ "+format+"\n", args...)
}

// Warn prints a warning message to stderr.
func Warn(msg string) {
	fmt.Fprintln(os.Stderr, "warning:", msg)
}

// Warnf prints a formatted warning message to stderr.
func Warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}
