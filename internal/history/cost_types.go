package history

import "time"

// CostSnapshot is the projected monthly cost of one Pulumi checkpoint.
type CostSnapshot struct {
	Timestamp     time.Time          `json:"timestamp"`
	Version       int                `json:"version"`
	TotalMonthly  float64            `json:"total_monthly"`
	Currency      string             `json:"currency"`
	ResourceCount int                `json:"resource_count"`
	ByProvider    map[string]float64 `json:"by_provider"`
	ByType        map[string]float64 `json:"by_type"`
	Resources     []CostResource     `json:"resources"`
}

// CostResource is one priced custom resource inside a CostSnapshot.
type CostResource struct {
	URN         string  `json:"urn"`
	Type        string  `json:"type"`
	Provider    string  `json:"provider"`
	MonthlyCost float64 `json:"monthly_cost"`
	SKU         string  `json:"sku"`
	Region      string  `json:"region"`
}

// CostAnnotation is the deployment note stored beside a snapshot.
type CostAnnotation struct {
	Version         int            `json:"version"`
	Message         string         `json:"message"`
	Kind            string         `json:"kind"`
	ResourceChanges map[string]int `json:"resource_changes"`
}

// StackUpdate is one row from `pulumi stack history --json`.
type StackUpdate struct {
	Version         int
	Kind            string
	Start           time.Time
	Message         string
	Result          string
	ResourceChanges map[string]int
}

// CostDBStats describes one per-stack cost history database.
type CostDBStats struct {
	Stack       string
	Path        string
	Snapshots   int
	First       time.Time
	Last        time.Time
	CollectedAt time.Time
	Size        int64
	LastVersion uint64
	Schema      uint64
}
