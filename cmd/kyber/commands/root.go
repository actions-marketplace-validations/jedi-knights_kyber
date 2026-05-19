// Package commands wires the kyber subcommands onto a cobra.Command tree.
// Each subcommand lives in its own file; this file only builds the root.
package commands

import "github.com/spf13/cobra"

// NewRoot constructs the kyber root command with version injected.
func NewRoot(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "kyber",
		Short:         "Function-level Go code-quality metrics.",
		Long:          "kyber analyzes a Go codebase function by function and emits per-function scores across size, complexity, and maintainability metrics — including cyclomatic and cognitive complexity, NPath, Halstead difficulty and effort, maintainability index, nesting depth, return count, function length, readability, and testability. Run `kyber list-metrics` for the full registered set.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(newAnalyzeCmd())
	root.AddCommand(newListMetricsCmd())
	root.AddCommand(newVersionCmd(version))
	return root
}
