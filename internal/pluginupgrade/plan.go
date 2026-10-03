package pluginupgrade

import (
	"fmt"
	goversion "go/version"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	modsemver "golang.org/x/mod/semver"
)

// Plan is the upgrade from a project's finfocus-spec version to a target.
type Plan struct {
	Current    string   `json:"current"`
	Target     string   `json:"target"`
	UpToDate   bool     `json:"up_to_date"`
	GoVersion  string   `json:"go_version,omitempty"`
	RequiredGo string   `json:"required_go,omitempty"`
	Hops       []Hop    `json:"hops"`
	Warnings   []string `json:"warnings,omitempty"`
}

// NewPlan plans the upgrade of p to target. latest is the newest version the
// running finfocus binary knows (its own pluginsdk.SpecVersion); target may
// not exceed it, and may not be older than the project's version.
func NewPlan(p *Project, target, latest string) (*Plan, error) {
	cur, err := semver.NewVersion(p.Version)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read finfocus-spec version %q from go.mod", ErrNotPlugin, p.Version)
	}
	newest, err := semver.NewVersion(latest)
	if err != nil {
		return nil, fmt.Errorf("%w: core spec version %q: %s", ErrInvalidTarget, latest, err.Error())
	}
	if !modsemver.IsValid(target) || modsemver.Canonical(target) != target {
		return nil, fmt.Errorf("%w: %q is not a vMAJOR.MINOR.PATCH version", ErrInvalidTarget, target)
	}
	tgt := semver.MustParse(target)
	if err = checkRange(cur, tgt, newest); err != nil {
		return nil, err
	}
	if err = checkKnownRelease(target, newest); err != nil {
		return nil, err
	}

	plan := &Plan{
		Current:   p.Version,
		Target:    "v" + tgt.String(),
		GoVersion: p.GoVersion,
		Hops:      []Hop{},
	}
	plan.addHops(cur, tgt)
	staleDecl := plan.addDeclWarnings(p.SpecVersionDecls)
	plan.addReplaceWarning(p)

	plan.UpToDate = tgt.Equal(cur) && !staleDecl && plan.RequiredGo == ""
	return plan, nil
}

func checkRange(cur, tgt, newest *semver.Version) error {
	switch {
	case cur.LessThan(semver.MustParse(minimumVersion)):
		return fmt.Errorf("%w: it requires finfocus-spec %s and the oldest supported is %s; "+
			"scaffold a new project with 'finfocus plugin init' and port the pricing logic",
			ErrTooOld, cur.Original(), minimumVersion)
	case cur.GreaterThan(newest):
		return fmt.Errorf("%w: it requires finfocus-spec %s and this build supports up to %s; upgrade finfocus",
			ErrNewerThanCore, cur.Original(), newest.Original())
	case tgt.GreaterThan(newest):
		return fmt.Errorf("%w: %s is newer than this finfocus build supports (%s)",
			ErrInvalidTarget, tgt.Original(), newest.Original())
	case tgt.LessThan(cur):
		return fmt.Errorf("%w: %s is older than the plugin's %s; downgrades are not supported",
			ErrInvalidTarget, tgt.Original(), cur.Original())
	}
	return nil
}

// checkKnownRelease accepts only versions known to be released: hop targets
// and core's own version. The command has no network access, so a patch
// release between them cannot be confirmed, and writing an unreleased
// version would only fail later in go mod tidy.
func checkKnownRelease(target string, newest *semver.Version) error {
	var known []string
	for _, h := range Hops() {
		if !semver.MustParse(h.To).GreaterThan(newest) {
			known = append(known, h.To)
		}
	}
	if len(known) == 0 || known[len(known)-1] != "v"+newest.String() {
		known = append(known, "v"+newest.String())
	}
	if slices.Contains(known, target) {
		return nil
	}
	return fmt.Errorf("%w: %s is not a release this build knows; use one of %s",
		ErrInvalidTarget, target, strings.Join(known, ", "))
}

func (plan *Plan) addReplaceWarning(p *Project) {
	switch {
	case p.ReplacePath == "":
	case p.ReplaceVersion != "" && p.ReplaceVersion != plan.Target:
		plan.Warnings = append(plan.Warnings, fmt.Sprintf(
			"go.mod replaces %s %s with %s; it stops applying once the requirement is %s, so update or remove it",
			Module, p.ReplaceVersion, p.ReplacePath, plan.Target))
	default:
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("go.mod replaces %s with %s; the replace is left unchanged", Module, p.ReplacePath))
	}
}

// addHops selects the hops in (cur, tgt] and the go directive they require.
func (plan *Plan) addHops(cur, tgt *semver.Version) {
	minGo := ""
	for _, h := range Hops() {
		to := semver.MustParse(h.To)
		if !to.GreaterThan(cur) || to.GreaterThan(tgt) {
			continue
		}
		plan.Hops = append(plan.Hops, h)
		if h.MinGo != "" && (minGo == "" || goLess(minGo, h.MinGo)) {
			minGo = h.MinGo
		}
	}
	if minGo != "" && (plan.GoVersion == "" || goLess(plan.GoVersion, minGo)) {
		plan.RequiredGo = minGo
	}
}

// addDeclWarnings reports SpecVersion declarations that disagree with go.mod
// or lack the v prefix, and whether any still needs rewriting to the target.
func (plan *Plan) addDeclWarnings(decls []SpecVersionDecl) bool {
	stale := false
	for _, d := range decls {
		if strings.TrimPrefix(d.Value, "v") != strings.TrimPrefix(plan.Current, "v") {
			plan.Warnings = append(plan.Warnings,
				fmt.Sprintf("%s declares SpecVersion %q but go.mod requires %s", d.File, d.Value, plan.Current))
		}
		if !strings.HasPrefix(d.Value, "v") {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf(
				"%s declares SpecVersion %q without the v prefix, which GetPluginInfo rejects; it is rewritten as %q",
				d.File, d.Value, plan.Target))
		}
		if d.Value != plan.Target {
			stale = true
		}
	}
	return stale
}

// goLess reports whether Go version a is older than b ("1.25.5" style).
func goLess(a, b string) bool {
	return goversion.Compare("go"+a, "go"+b) < 0
}
