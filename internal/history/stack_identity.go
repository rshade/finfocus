package history

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	metaKeyProject = "project"
	stackSeparator = "@"
	urnPrefix      = "urn:pulumi:"
	qualifiedParts = 3
	orgStackParts  = 2
	urnMinParts    = 3
)

// SplitStack splits a --stack value into its project and short stack name. A
// fully qualified "org/project/stack" carries the project; "org/stack" and a
// bare "stack" do not, so the project is "" and the caller supplies it.
func SplitStack(input string) (string, string) {
	input = strings.TrimSpace(input)
	parts := strings.Split(input, "/")
	switch len(parts) {
	case qualifiedParts:
		return parts[1], parts[2]
	case orgStackParts:
		return "", parts[1]
	default:
		return "", input
	}
}

// CostFileNameFor names the timeline database for one stack of one project:
// `<project>@<stack>.history.db`. Pulumi project and stack names cannot contain
// "@", so two projects that share a stack name, or whose names only differ by a
// dash, never share a file. With no project it returns the legacy
// `<stack>.history.db` name from CostFileName.
func CostFileNameFor(project, stack string) (string, error) {
	if project == "" {
		return CostFileName(stack)
	}
	if slices.ContainsFunc([]string{project, stack}, invalidNamePart) {
		return "", fmt.Errorf("invalid project or stack name %q/%q", project, stack)
	}
	return project + stackSeparator + stack + costDBSuffix, nil
}

func invalidNamePart(part string) bool {
	return part == "" || strings.Contains(part, "..") ||
		strings.ContainsAny(part, " \t\r\n/\\"+stackSeparator)
}

// ProjectFromURN returns the project named by a Pulumi URN
// (`urn:pulumi:<stack>::<project>::<type>::<name>`), or "" for anything else.
func ProjectFromURN(urn string) string {
	if !strings.HasPrefix(urn, urnPrefix) {
		return ""
	}
	parts := strings.Split(urn, urnSeparator)
	if len(parts) < urnMinParts {
		return ""
	}
	return parts[1]
}

// ProjectFromFileName returns the project and stack encoded in a
// `<project>@<stack>.history.db` name, and false for a legacy name.
func ProjectFromFileName(name string) (string, string, bool) {
	base, ok := strings.CutSuffix(name, costDBSuffix)
	if !ok {
		return "", "", false
	}
	project, stack, found := strings.Cut(base, stackSeparator)
	if !found || project == "" || stack == "" {
		return "", "", false
	}
	return project, stack, true
}

// InferProject returns the project that owns this database: the stored one, or
// for a database written before projects were recorded, the project in the URNs
// of its snapshots. It returns "" when nothing identifies one.
func (d *CostDB) InferProject() (string, error) {
	stats, err := d.Stats()
	if err != nil {
		return "", err
	}
	if stats.Project != "" {
		return stats.Project, nil
	}
	snapshots, err := d.Snapshots(time.Time{}, time.Time{})
	if err != nil {
		return "", err
	}
	for _, snapshot := range snapshots {
		for _, resource := range snapshot.Resources {
			if project := ProjectFromURN(resource.URN); project != "" {
				return project, nil
			}
		}
	}
	return "", nil
}
