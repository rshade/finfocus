package esc

import (
	"fmt"
	"regexp"
	"strings"
)

// segmentPattern matches one ESC reference segment or version.
var segmentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

const (
	orgMinLen = 3
	orgMaxLen = 40
)

// Ref is one Pulumi ESC environment reference.
type Ref struct {
	Org     string
	Project string
	Name    string
	Version string
}

// ParseRef parses [[org/]project/]name[@version].
// One segment is default/<name>. The organization must be 3 to 40 characters.
func ParseRef(raw string) (Ref, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Ref{}, fmt.Errorf("invalid ESC environment reference %q: empty reference", raw)
	}
	namePart, version, hasVersion := strings.Cut(trimmed, "@")
	if hasVersion && version == "" {
		return Ref{}, fmt.Errorf("invalid ESC environment reference %q: missing version", raw)
	}
	if hasVersion && !segmentPattern.MatchString(version) {
		return Ref{}, fmt.Errorf("invalid ESC environment reference %q: invalid version", raw)
	}
	parts := strings.Split(namePart, "/")
	for _, part := range parts {
		if part == "" {
			return Ref{}, fmt.Errorf("invalid ESC environment reference %q: empty segment", raw)
		}
		if !segmentPattern.MatchString(part) {
			return Ref{}, fmt.Errorf("invalid ESC environment reference %q: invalid segment", raw)
		}
	}
	ref, err := refFromParts(raw, parts)
	if err != nil {
		return Ref{}, err
	}
	ref.Version = version
	return ref, nil
}

const (
	refSegmentsProject = 2
	refSegmentsFull    = 3
)

func refFromParts(raw string, parts []string) (Ref, error) {
	switch len(parts) {
	case 1:
		return Ref{Project: "default", Name: parts[0]}, nil
	case refSegmentsProject:
		return Ref{Project: parts[0], Name: parts[1]}, nil
	case refSegmentsFull:
		if len(parts[0]) < orgMinLen || len(parts[0]) > orgMaxLen {
			return Ref{}, fmt.Errorf(
				"invalid ESC environment reference %q: organization must be 3 to 40 characters",
				raw,
			)
		}
		return Ref{Org: parts[0], Project: parts[1], Name: parts[2]}, nil
	default:
		return Ref{}, fmt.Errorf("invalid ESC environment reference %q: too many segments", raw)
	}
}

// String returns the canonical reference, without an empty organization or version.
func (r Ref) String() string {
	if r.Name == "" && r.Project == "" && r.Org == "" {
		return ""
	}
	var b strings.Builder
	if r.Org != "" {
		b.WriteString(r.Org)
		b.WriteByte('/')
	}
	if r.Project != "" {
		b.WriteString(r.Project)
		if r.Name != "" {
			b.WriteByte('/')
		}
	}
	b.WriteString(r.Name)
	if r.Version != "" {
		b.WriteByte('@')
		b.WriteString(r.Version)
	}
	return b.String()
}
