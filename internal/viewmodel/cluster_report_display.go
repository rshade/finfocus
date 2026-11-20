package viewmodel

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
)

const (
	// clusterDigestDisplayLen is how many characters of a policy digest are shown.
	clusterDigestDisplayLen = 12
	// clusterPercent converts a ratio to a percentage.
	clusterPercent = 100
	// clusterWarningExamples is how many subjects a summarized warning names.
	clusterWarningExamples = 3
	// clusterMinGroupedWarnings is the smallest number of distinct subjects
	// sharing a detail that is folded into one line.
	clusterMinGroupedWarnings = 2
	// clusterWarningSubjectSep separates a per-resource warning's subject from
	// its detail, as in "incomplete: ns/pod: gap in cpu_usage".
	clusterWarningSubjectSep = ": "
)

// ClusterReportDisplay is the formatted text of a cluster report. The CLI table
// and the web dashboard show the same mode, share, warning, and incomplete
// text; amounts here carry the currency symbol and code.
type ClusterReportDisplay struct {
	Total string `json:"total"`
	// Idle and IdleShare are empty when the report is scoped.
	Idle      string `json:"idle,omitempty"`
	IdleShare string `json:"idleShare,omitempty"`
	// Scope names what scoped the report ("namespace", "selector", or both);
	// empty when unscoped.
	Scope    string                   `json:"scope,omitempty"`
	Mode     string                   `json:"mode"`
	Policy   string                   `json:"policy"`
	Groups   []ClusterGroupDisplay    `json:"groups"`
	Capacity []ClusterCapacityDisplay `json:"capacity"`
	// Warnings are the usage source's warnings with per-resource repeats
	// summarized; the report's warnings field keeps every one.
	Warnings []string `json:"warnings"`
	// IncompleteCauses are what the usage source reported missing.
	IncompleteCauses []string `json:"incompleteCauses"`
	// Unpriced names each resource that could not be priced, with its note.
	Unpriced []string `json:"unpriced"`
}

// ClusterGroupDisplay is one group's formatted amounts, in report order.
type ClusterGroupDisplay struct {
	Key    string `json:"key"    datakey:"true"`
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Total  string `json:"total"`
	Share  string `json:"share"`
}

// ClusterCapacityDisplay is one capacity type's formatted amounts. Idle is
// empty when idle capacity was not measured.
type ClusterCapacityDisplay struct {
	CapacityType string `json:"capacityType"`
	Total        string `json:"total"`
	Share        string `json:"share"`
	Idle         string `json:"idle,omitempty"`
}

// BuildClusterReportDisplay formats a cluster report for display.
func BuildClusterReportDisplay(report engine.ClusterReport) ClusterReportDisplay {
	warnings := SummarizeClusterWarnings(report.Warnings)
	display := ClusterReportDisplay{
		Total:            FormatMoney(report.Total, report.Currency),
		Scope:            clusterScope(report),
		Mode:             ClusterModeLine(report.Mode, report.Period),
		Policy:           ClusterPolicyLine(report.Policy.Source, report.Policy.Digest),
		Groups:           make([]ClusterGroupDisplay, 0, len(report.Groups)),
		Capacity:         make([]ClusterCapacityDisplay, 0, len(report.Capacity)),
		Warnings:         nonNil(warnings.Lines()),
		IncompleteCauses: nonNil(warnings.UsageCauses()),
		Unpriced:         nonNil(ClusterUnpriced(report.Priced)),
	}
	if report.Idle != nil {
		display.Idle = FormatMoney(*report.Idle, report.Currency)
		display.IdleShare = ClusterShare(*report.Idle, report.Total)
	}
	for _, g := range report.Groups {
		display.Groups = append(display.Groups, ClusterGroupDisplay{
			Key:    g.Key,
			CPU:    FormatMoney(g.CPUCost, report.Currency),
			Memory: FormatMoney(g.MemCost, report.Currency),
			Total:  FormatMoney(g.TotalCost, report.Currency),
			Share:  ClusterShare(g.TotalCost, report.Total),
		})
	}
	for _, c := range report.Capacity {
		row := ClusterCapacityDisplay{
			CapacityType: c.CapacityType,
			Total:        FormatMoney(c.TotalCost, report.Currency),
			Share:        ClusterShare(c.TotalCost, report.Total),
		}
		if c.IdleCost != nil {
			row.Idle = FormatMoney(*c.IdleCost, report.Currency)
		}
		display.Capacity = append(display.Capacity, row)
	}
	return display
}

// ClusterModeLine describes the report's basis: the monthly run rate or the
// historical window.
func ClusterModeLine(mode, period string) string {
	if mode == engine.ModeHistorical {
		return fmt.Sprintf("%s (%s)", mode, period)
	}
	return fmt.Sprintf("%s (monthly, %d h)", mode, engine.HoursPerMonth)
}

// ClusterShare is part as a percentage of total, to one decimal. A total of
// zero or less has no share and reads 0.0%.
func ClusterShare(part, total float64) string {
	share := 0.0
	if total > 0 {
		share = part / total * clusterPercent
	}
	return fmt.Sprintf("%.1f%%", share)
}

// ClusterPolicyLine names a policy by its source and short digest.
func ClusterPolicyLine(source, digest string) string {
	return source + " · " + ShortDigest(digest)
}

// ShortDigest is the leading part of a policy digest shown beside its source.
func ShortDigest(digest string) string {
	if len(digest) > clusterDigestDisplayLen {
		return digest[:clusterDigestDisplayLen]
	}
	return digest
}

// ClusterUnpriced names each resource that could not be priced, followed by
// its note when it has one.
func ClusterUnpriced(priced []engine.PricedSummary) []string {
	var missing []string
	for _, p := range priced {
		if p.Priced {
			continue
		}
		if p.Note == "" {
			missing = append(missing, p.ID)
		} else {
			missing = append(missing, fmt.Sprintf("%s: %s", p.ID, p.Note))
		}
	}
	return missing
}

func clusterScope(report engine.ClusterReport) string {
	var scopes []string
	if report.NamespaceScoped {
		scopes = append(scopes, "namespace")
	}
	if report.SelectorScoped {
		scopes = append(scopes, "selector")
	}
	return strings.Join(scopes, " and ")
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// ClusterWarningSummary is a usage source's warnings sorted for display.
// Incomplete warnings that name a subject and share a detail with another are
// grouped, so a gap in every pod reads as one line instead of one per pod.
// Everything else, including store-wide warnings such as a short retention,
// is kept as written.
type ClusterWarningSummary struct {
	single []string
	groups []clusterWarningGroup
}

// clusterWarningGroup is the incomplete warnings that share one detail. A
// usage source can repeat a subject (one pod name on two nodes), so subjects
// are distinct and entries counts the warnings folded into the group.
type clusterWarningGroup struct {
	detail   string
	subjects []string
	entries  int
}

func (g clusterWarningGroup) summary() string {
	return fmt.Sprintf("%s for %d resources", g.detail, len(g.subjects))
}

func (g clusterWarningGroup) examples() string {
	shown := g.subjects
	if len(shown) > clusterWarningExamples {
		shown = shown[:clusterWarningExamples]
	}
	list := strings.Join(shown, ", ")
	if rest := len(g.subjects) - len(shown); rest > 0 {
		list += fmt.Sprintf(", and %d more", rest)
	}
	return list
}

// SummarizeClusterWarnings sorts warnings for display.
func SummarizeClusterWarnings(warnings []string) ClusterWarningSummary {
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

	grouped := map[string]bool{}
	var out ClusterWarningSummary
	for _, detail := range order {
		entries := len(byDetail[detail])
		subjects := slices.Compact(slices.Sorted(slices.Values(byDetail[detail])))
		if len(subjects) < clusterMinGroupedWarnings {
			continue
		}
		grouped[detail] = true
		out.groups = append(out.groups, clusterWarningGroup{detail: detail, subjects: subjects, entries: entries})
	}
	for _, w := range warnings {
		if _, detail, ok := splitIncompleteWarning(w); !ok || !grouped[detail] {
			out.single = append(out.single, w)
		}
	}
	return out
}

// Lines are the warnings to show: those kept as written, then one line per
// group naming a few of its subjects.
func (s ClusterWarningSummary) Lines() []string {
	lines := slices.Clone(s.single)
	for _, g := range s.groups {
		lines = append(lines, fmt.Sprintf("%s %s (%s)", engine.IncompleteWarningPrefix, g.summary(), g.examples()))
	}
	return lines
}

// Summarized counts the warnings folded into groups.
func (s ClusterWarningSummary) Summarized() int {
	n := 0
	for _, g := range s.groups {
		n += g.entries
	}
	return n
}

// UsageCauses are what the usage source reported missing: each incomplete
// warning kept as written, without its prefix, then each group's summary.
func (s ClusterWarningSummary) UsageCauses() []string {
	var causes []string
	for _, w := range s.single {
		if rest, ok := strings.CutPrefix(w, engine.IncompleteWarningPrefix); ok {
			causes = append(causes, strings.TrimSpace(rest))
		}
	}
	for _, g := range s.groups {
		causes = append(causes, g.summary())
	}
	return causes
}

// splitIncompleteWarning splits "incomplete: <subject>: <detail>". A warning
// without the prefix or without a subject is not split.
func splitIncompleteWarning(w string) (string, string, bool) {
	rest, ok := strings.CutPrefix(w, engine.IncompleteWarningPrefix)
	if !ok {
		return "", "", false
	}
	subject, detail, ok := strings.Cut(strings.TrimSpace(rest), clusterWarningSubjectSep)
	if !ok || subject == "" || detail == "" {
		return "", "", false
	}
	return subject, detail, true
}
