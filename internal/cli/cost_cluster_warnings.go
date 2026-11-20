package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// printClusterWarnings writes the usage source's warnings to stderr with
// per-resource repeats summarized, then says where the full list is. JSON
// output keeps every warning.
func printClusterWarnings(cmd *cobra.Command, format string, warnings []string) {
	summary := viewmodel.SummarizeClusterWarnings(warnings)
	for _, line := range summary.Lines() {
		cmd.PrintErrln("warning: " + line)
	}
	n := summary.Summarized()
	if n == 0 {
		return
	}
	where := `--output json lists each one under "warnings"`
	if format == outputFormatJSON || format == outputFormatNDJSON {
		where = `the "warnings" field lists each one`
	}
	cmd.PrintErrf("warning: %d warnings summarized above; %s\n", n, where)
}

// renderIncompleteFooter states why the report is incomplete: what the usage
// source reported missing, and which resources could not be priced.
func renderIncompleteFooter(cmd *cobra.Command, out engine.ClusterReport) {
	causes := viewmodel.SummarizeClusterWarnings(out.Warnings).UsageCauses()
	if len(causes) > 0 {
		cmd.Println("Incomplete: the usage source reported partial data")
		for _, c := range causes {
			cmd.Printf("  - %s\n", c)
		}
	}
	missing := viewmodel.ClusterUnpriced(out.Priced)
	if len(missing) > 0 {
		noun := "resources"
		if len(missing) == 1 {
			noun = "resource"
		}
		cmd.Printf("Incomplete: %d %s could not be priced (%s)\n", len(missing), noun, strings.Join(missing, "; "))
	}
	if len(causes) == 0 && len(missing) == 0 {
		cmd.Println("Incomplete: see the warnings on stderr")
	}
}
