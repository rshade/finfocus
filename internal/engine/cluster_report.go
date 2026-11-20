package engine

import "encoding/json"

// ClusterPolicy is the policy section of the `cost cluster` JSON contract, and
// the whole `--show-policy` document.
type ClusterPolicy struct {
	// Source is the policy file path, "built-in defaults", or the label of
	// policy text supplied by a request.
	Source    string          `json:"source"`
	Digest    string          `json:"digest"`
	Effective json.RawMessage `json:"effective"`
}

// ClusterReport is the stable JSON contract of `cost cluster --output json`.
// The web dashboard returns the same document.
type ClusterReport struct {
	Mode            string          `json:"mode"`
	Period          string          `json:"period"`
	Currency        string          `json:"currency"`
	GroupBy         string          `json:"group_by"`
	Total           float64         `json:"total"`
	Idle            *float64        `json:"idle,omitempty"` // nil when namespace- or selector-scoped
	NamespaceScoped bool            `json:"namespace_scoped"`
	SelectorScoped  bool            `json:"selector_scoped"`
	Incomplete      bool            `json:"incomplete"`
	Groups          []ClusterGroup  `json:"groups"`
	Capacity        []CapacityGroup `json:"capacity"`
	Priced          []PricedSummary `json:"priced"`
	Policy          ClusterPolicy   `json:"policy"`
	Warnings        []string        `json:"warnings,omitempty"`
}

// NewClusterReport builds the report document from an allocation result and
// its grouped rows. A scoped result has no idle amount, and capacity is an
// empty list rather than null.
func NewClusterReport(res *ClusterResult, groups []ClusterGroup, groupBy string, policy ClusterPolicy) ClusterReport {
	report := ClusterReport{
		Mode: res.Mode, Period: res.Period, Currency: res.Currency, GroupBy: groupBy, Total: res.Total,
		NamespaceScoped: res.NamespaceScoped, SelectorScoped: res.SelectorScoped, Incomplete: res.Incomplete,
		Groups: groups, Capacity: res.Capacity, Priced: res.Priced, Policy: policy, Warnings: res.Warnings,
	}
	if report.Capacity == nil {
		report.Capacity = []CapacityGroup{}
	}
	if !res.Scoped() {
		report.Idle = new(res.Idle)
	}
	return report
}
