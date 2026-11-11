package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/engine"
)

// warningExamples is how many subjects a summarized warning names.
const warningExamples = 3

// minGroupedWarnings is the smallest number of subjects sharing a detail
// that is folded into one line.
const minGroupedWarnings = 2

// warningSubjectSep separates a per-resource warning's subject from its detail,
// as in "incomplete: ns/pod: gap in cpu_usage".
const warningSubjectSep = ": "

// clusterWarnings sorts a usage source's warnings for display. Incomplete
// warnings that name a subject and share a detail with another are grouped,
// so a gap in every pod reads as one line instead of one per pod. Everything
// else, including store-wide warnings such as a short retention, is kept as
// written. Only the table and stderr use it; JSON keeps every warning.
type clusterWarnings struct {
	single []string
	groups []warningGroup
}

// warningGroup is the incomplete warnings that share one detail.
type warningGroup struct {
	detail   string
	subjects []string
}

func (g warningGroup) summary() string {
	return fmt.Sprintf("%s for %d resources", g.detail, len(g.subjects))
}

func (g warningGroup) examples() string {
	shown := g.subjects
	if len(shown) > warningExamples {
		shown = shown[:warningExamples]
	}
	list := strings.Join(shown, ", ")
	if rest := len(g.subjects) - len(shown); rest > 0 {
		list += fmt.Sprintf(", and %d more", rest)
	}
	return list
}

func (w clusterWarnings) summarized() int {
	n := 0
	for _, g := range w.groups {
		n += len(g.subjects)
	}
	return n
}

func sortClusterWarnings(warnings []string) clusterWarnings {
	var order []string
	byDetail := map[string][]string{}
	for _, w := range warnings {
		subject, detail, ok := splitIncompleteWarning(w)
		if !ok {
			continue
		}
		if _, seen := byDetail[detail]; !seen {
			order = append(order, detail)
		}
		byDetail[detail] = append(byDetail[detail], subject)
	}

	var out clusterWarnings
	for _, w := range warnings {
		if _, detail, ok := splitIncompleteWarning(w); !ok || len(byDetail[detail]) < minGroupedWarnings {
			out.single = append(out.single, w)
		}
	}
	for _, detail := range order {
		subjects := byDetail[detail]
		if len(subjects) < minGroupedWarnings {
			continue
		}
		slices.Sort(subjects)
		out.groups = append(out.groups, warningGroup{detail: detail, subjects: subjects})
	}
	return out
}

// splitIncompleteWarning splits "incomplete: <subject>: <detail>". A warning
// without the prefix or without a subject is not split.
func splitIncompleteWarning(w string) (string, string, bool) {
	rest, ok := strings.CutPrefix(w, engine.IncompleteWarningPrefix)
	if !ok {
		return "", "", false
	}
	subject, detail, ok := strings.Cut(strings.TrimSpace(rest), warningSubjectSep)
	if !ok || subject == "" || detail == "" {
		return "", "", false
	}
	return subject, detail, true
}

// printClusterWarnings writes the usage source's warnings to stderr with
// per-resource repeats summarized, then says where the full list is.
func printClusterWarnings(cmd *cobra.Command, format string, warnings []string) {
	sorted := sortClusterWarnings(warnings)
	for _, w := range sorted.single {
		cmd.PrintErrln("warning: " + w)
	}
	for _, g := range sorted.groups {
		cmd.PrintErrf("warning: %s %s (%s)\n", engine.IncompleteWarningPrefix, g.summary(), g.examples())
	}
	n := sorted.summarized()
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
func renderIncompleteFooter(cmd *cobra.Command, out clusterOutput) {
	causes := usageCauses(out.Warnings)
	if len(causes) > 0 {
		cmd.Println("Incomplete: the usage source reported partial data")
		for _, c := range causes {
			cmd.Printf("  - %s\n", c)
		}
	}
	var missing []string
	for _, p := range out.Priced {
		if p.Priced {
			continue
		}
		if p.Note == "" {
			missing = append(missing, p.ID)
		} else {
			missing = append(missing, fmt.Sprintf("%s: %s", p.ID, p.Note))
		}
	}
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

func usageCauses(warnings []string) []string {
	sorted := sortClusterWarnings(warnings)
	var causes []string
	for _, w := range sorted.single {
		if rest, ok := strings.CutPrefix(w, engine.IncompleteWarningPrefix); ok {
			causes = append(causes, strings.TrimSpace(rest))
		}
	}
	for _, g := range sorted.groups {
		causes = append(causes, g.summary())
	}
	return causes
}
