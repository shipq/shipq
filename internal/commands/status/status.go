package status

import (
	"fmt"
	"strings"

	shipqdag "github.com/shipq/shipq/internal/dag"
	"github.com/shipq/shipq/project"
)

// StatusCmd implements the "shipq status" command.
// It prints the current state of the project DAG, showing which commands
// have been completed (their postconditions are met) and which are available
// to run next.
func StatusCmd() {
	roots, err := project.FindProjectRoots()
	if err != nil {
		fmt.Println("Not in a shipq project.")
		fmt.Println("Run 'shipq init' to get started.")
		return
	}

	graph := shipqdag.Graph()
	satisfied := shipqdag.SatisfiedFunc(roots.ShipqRoot)

	fmt.Println("shipq project status:")
	fmt.Println("")

	for _, node := range graph.Nodes() {
		isSatisfied := satisfied(node.ID)
		icon := "✗"
		if isSatisfied {
			icon = "✓"
		}

		suffix := ""
		if !isSatisfied {
			unsatisfied := graph.CheckHardDeps(node.ID, satisfied)
			if len(unsatisfied) > 0 {
				names := make([]string, len(unsatisfied))
				for i, id := range unsatisfied {
					names[i] = shipqdag.CommandName(id)
				}
				suffix = fmt.Sprintf(" (requires: %s)", strings.Join(names, ", "))
			}
		}

		fmt.Printf("  %s %-18s %s%s\n",
			icon, shipqdag.CommandName(node.ID), node.Description, suffix)
	}

	available := graph.Available(satisfied)
	if len(available) > 0 {
		fmt.Println("")
		fmt.Println("Available next steps:")
		for _, id := range available {
			node := graph.Find(id)
			fmt.Printf("  shipq %-20s %s\n", shipqdag.CommandName(id), node.Description)
		}
	}
}
