// Package sourcepos turns the source positions Pulumi records for a resource
// into project-relative declaration locations.
//
// It is a leaf package: it imports only the standard library. A location is
// returned only for a regular file under the project root, so a recorded path
// that points outside the project (an SDK, a dependency, the standard
// library, a symlink out of the root) never reaches output.
package sourcepos

import (
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	positionScheme = "project:///"
	goRuntime      = "go"
	parentSegment  = ".."
	maxGoModBytes  = 1 << 20
)

// Location is a declaration site: a project-relative path with forward
// slashes, a 1-based line, and a 1-based column (0 when the runtime records
// none).
type Location struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column,omitempty"`
}

// Resolver resolves recorded positions against one project.
type Resolver struct {
	root       string
	runtime    string
	modulePath string
}

// NewResolver returns a resolver for the project at root. The runtime is the
// Pulumi runtime name; for "go" the module path is read once from go.mod,
// because Go programs are built with -trimpath and record module paths.
// A root that cannot be resolved yields a resolver that finds nothing.
func NewResolver(root, runtime string) *Resolver {
	r := &Resolver{runtime: strings.ToLower(runtime)}
	abs, err := filepath.Abs(root)
	if err != nil || root == "" {
		return r
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return r
	}
	r.root = resolved
	if r.runtime == goRuntime {
		r.modulePath = readModulePath(filepath.Join(resolved, "go.mod"))
	}
	return r
}

// Resolve returns the declaration location for a resource, or nil when none
// can be established. frames are the recorded stack-trace positions from the
// innermost outward; the first one inside the project wins, and position (the
// resource's own recorded position) is the fallback. Empty frames are skipped.
func (r *Resolver) Resolve(position string, frames []string) *Location {
	if r == nil || r.root == "" {
		return nil
	}
	for _, frame := range frames {
		if loc := r.locate(frame); loc != nil {
			return loc
		}
	}
	return r.locate(position)
}

func (r *Resolver) locate(raw string) *Location {
	spec, ok := parsePosition(raw)
	if !ok {
		return nil
	}
	rel, ok := r.projectRelative(spec.path)
	if !ok || !r.isRegularFileInRoot(rel) {
		return nil
	}
	return &Location{File: rel, Line: spec.line, Column: spec.column}
}

type position struct {
	path         string
	line, column int
}

func parsePosition(raw string) (position, bool) {
	rest, ok := strings.CutPrefix(raw, positionScheme)
	if !ok {
		return position{}, false
	}
	filePart, spec, ok := strings.Cut(rest, "#")
	if !ok || filePart == "" {
		return position{}, false
	}
	lineText, columnText, hasColumn := strings.Cut(spec, ",")
	line, ok := parseCount(lineText)
	if !ok || line < 1 {
		return position{}, false
	}
	column := 0
	if hasColumn {
		if column, ok = parseCount(columnText); !ok {
			return position{}, false
		}
	}
	return position{path: strings.ReplaceAll(filePart, `\`, "/"), line: line, column: column}, true
}

func parseCount(text string) (int, bool) {
	if text == "" {
		return 0, false
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(text)
	return n, err == nil
}

func (r *Resolver) projectRelative(recorded string) (string, bool) {
	cleaned := path.Clean(recorded)
	rel := cleaned
	if cleaned == parentSegment || strings.HasPrefix(cleaned, parentSegment+"/") {
		var inModule bool
		if rel, inModule = r.stripModulePath(cleaned); !inModule {
			return "", false
		}
	}
	if rel == "." || strings.HasPrefix(rel, "/") || isDependencyPath(rel) {
		return "", false
	}
	if filepath.VolumeName(filepath.FromSlash(rel)) != "" {
		return "", false
	}
	return rel, true
}

// stripModulePath maps a trimpath module path such as
// "../../shop/main.go" to the project file "main.go". Go records the module
// path under a run of ".." segments whose length is the project's depth.
func (r *Resolver) stripModulePath(cleaned string) (string, bool) {
	if r.modulePath == "" {
		return "", false
	}
	trimmed := cleaned
	for trimmed == parentSegment || strings.HasPrefix(trimmed, parentSegment+"/") {
		trimmed = strings.TrimPrefix(strings.TrimPrefix(trimmed, parentSegment), "/")
	}
	rel, ok := strings.CutPrefix(trimmed, r.modulePath+"/")
	if !ok || rel == "" {
		return "", false
	}
	return rel, true
}

func isDependencyPath(rel string) bool {
	for segment := range strings.SplitSeq(rel, "/") {
		if segment == "node_modules" || segment == "site-packages" {
			return true
		}
	}
	return false
}

func (r *Resolver) isRegularFileInRoot(rel string) bool {
	target, err := filepath.EvalSymlinks(filepath.Join(r.root, filepath.FromSlash(rel)))
	if err != nil {
		return false
	}
	within, err := filepath.Rel(r.root, target)
	if err != nil || within == parentSegment || strings.HasPrefix(within, parentSegment+string(filepath.Separator)) {
		return false
	}
	info, err := os.Stat(target)
	return err == nil && info.Mode().IsRegular()
}

func readModulePath(goMod string) string {
	info, err := os.Lstat(goMod)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxGoModBytes {
		return ""
	}
	data, err := os.ReadFile(goMod)
	if err != nil {
		return ""
	}
	for text := range strings.Lines(string(data)) {
		line, _, _ := strings.Cut(text, "//")
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "module" {
			continue
		}
		if unquoted, unquoteErr := strconv.Unquote(fields[1]); unquoteErr == nil {
			return unquoted
		}
		return fields[1]
	}
	return ""
}
