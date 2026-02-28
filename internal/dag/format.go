package shipqdag

import (
	"fmt"
	"os"

	"github.com/shipq/shipq/cli"
)

// PrintPrerequisiteError prints a formatted error message listing the
// unsatisfied hard dependencies for a command, with instructions on what
// to run.
func PrintPrerequisiteError(cmd CommandID, unsatisfied []CommandID) {
	fmt.Fprintf(os.Stderr, "error: cannot run 'shipq %s'\n", CommandName(cmd))
	fmt.Fprintln(os.Stderr, "The following prerequisites are not met:")
	for _, dep := range unsatisfied {
		fmt.Fprintf(os.Stderr, "  • run 'shipq %s' first\n", CommandName(dep))
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Run the commands above in order, then try again.")
}

// PrintSoftDepWarning prints a warning about an unsatisfied soft dependency.
func PrintSoftDepWarning(dep CommandID) {
	cli.Warnf("recommended prerequisite not met: run 'shipq %s' first", CommandName(dep))
}

// CheckPrerequisites checks hard and soft dependencies for the given command
// using the project at shipqRoot. If any hard dependencies are unmet, it
// prints a prerequisite error and returns false. If all hard deps are met
// but some soft deps are unmet, it prints warnings and returns true.
//
// Usage in a command entry point:
//
//	if !shipqdag.CheckPrerequisites(shipqdag.CmdAuth, roots.ShipqRoot) {
//	    os.Exit(1)
//	}
func CheckPrerequisites(cmd CommandID, shipqRoot string) bool {
	graph := Graph()
	satisfied := SatisfiedFunc(shipqRoot)

	// Hard dep check — fatal if unmet.
	if unsatisfied := graph.CheckHardDeps(cmd, satisfied); len(unsatisfied) > 0 {
		PrintPrerequisiteError(cmd, unsatisfied)
		return false
	}

	// Soft dep check — warnings only.
	for _, dep := range graph.CheckSoftDeps(cmd, satisfied) {
		PrintSoftDepWarning(dep)
	}

	return true
}
