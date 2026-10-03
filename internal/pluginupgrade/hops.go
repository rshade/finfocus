// Package pluginupgrade moves a FinFocus plugin project from one
// finfocus-spec version to a newer one. It detects the version a plugin
// builds against, plans the hops between that version and a target, and
// applies the edits that need no judgment. Everything else is listed as a
// manual step with a migration guide, which the finfocus-plugin-upgrade agent
// skill works through.
package pluginupgrade

const (
	// Module is the finfocus-spec Go module path.
	Module = "github.com/rshade/finfocus-spec"

	// minimumVersion is the oldest release this package upgrades from: the
	// first published under the finfocus-spec module path.
	minimumVersion = "v0.5.0"

	// GuideBaseURL is where the migration guides are published.
	GuideBaseURL = "https://github.com/rshade/finfocus/blob/main/agent-skills/finfocus-plugin-upgrade/references/"
)

// Hop is the step up to one finfocus-spec release that requires action from
// a plugin author. Releases between hops need no action beyond the go.mod
// version.
type Hop struct {
	To        string   `json:"to"`
	MinGo     string   `json:"min_go,omitempty"`
	Summary   string   `json:"summary"`
	Automatic []string `json:"automatic,omitempty"`
	Manual    []string `json:"manual,omitempty"`
	Guide     string   `json:"guide"`
	GuideURL  string   `json:"guide_url"`
}

// hops is ordered by To. Each entry needs a guide named Guide in
// agent-skills/finfocus-plugin-upgrade/references/; TestHopsMatchGuides
// enforces it, and TestHopsReachCoreSpecVersion requires a hop for the
// finfocus-spec version core builds against.
//
//nolint:gochecknoglobals // Immutable migration table; Hops returns copies.
var hops = []Hop{
	{
		To:      "v0.5.7",
		MinGo:   "1.25.7",
		Summary: "DryRun handlers take a context",
		Manual: []string{
			"Change HandleDryRun(req) to HandleDryRun(ctx context.Context, req) on DryRunHandler implementations",
		},
		Guide: "to-v0.5.7.md",
	},
	{
		To:      "v0.6.0",
		Summary: "Batch validation returns a result type; validation errors are typed",
		Manual: []string{
			"Update callers of pluginsdk.ValidateBatchCostRequest: it returns (BatchCostValidationResult, error)",
			"Replace string matching on validation errors with errors.Is against the ValidationError sentinels",
			"Expect oversized ResourceDescriptor fields to be rejected by validation",
		},
		Guide: "to-v0.6.0.md",
	},
	{
		To:        "v0.6.1",
		MinGo:     "1.27.1",
		Summary:   "Go 1.27.1 required",
		Automatic: []string{"Raise the go directive to 1.27.1"},
		Manual:    []string{"Update the Go version in CI workflows and Dockerfiles to 1.27.1"},
		Guide:     "to-v0.6.1.md",
	},
	{
		To:      "v0.6.2",
		Summary: "Supports answers reach the host; capabilities are checked",
		Manual: []string{
			"Implement SupportsProvider: the default answer now reaches hosts as Supported:false " +
				"(finfocus core fails open on it; other hosts may not)",
			"If GetPluginInfo sets Capabilities by hand, make sure it lists every implemented service",
		},
		Guide: "to-v0.6.2.md",
	},
	{
		To:      "v0.7.0",
		Summary: "Additive release; no changes required",
		Guide:   "to-v0.7.0.md",
	},
	{
		To:      "v0.7.1",
		Summary: "Additive release; one scorer field deprecated",
		Manual: []string{
			"Scorer plugins only: set ScorerInfo.ProviderRequestIds and keep ProviderRequestId as its first entry " +
				"(the single field is deprecated, so staticcheck flags it)",
		},
		Guide: "to-v0.7.1.md",
	},
}

// Hops returns a copy of the hop table, oldest first, with guide URLs set.
func Hops() []Hop {
	out := make([]Hop, len(hops))
	for i, h := range hops {
		h.Automatic = append([]string(nil), h.Automatic...)
		h.Manual = append([]string(nil), h.Manual...)
		h.GuideURL = GuideBaseURL + h.Guide
		out[i] = h
	}
	return out
}
