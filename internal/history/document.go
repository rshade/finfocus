package history

import "time"

// ViewDocument is the `cost history view --output json` document: the
// snapshots in range for one currency, their deployment annotations, and the
// mixed-currency warning.
type ViewDocument struct {
	Stack           string           `json:"stack"`
	Snapshots       []CostSnapshot   `json:"snapshots"`
	Annotations     []CostAnnotation `json:"annotations"`
	CurrencyWarning string           `json:"currency_warning,omitempty"`
}

// DiffPointDocument is one end of a DiffDocument.
type DiffPointDocument struct {
	Version      int     `json:"version"`
	Timestamp    string  `json:"timestamp"`
	TotalMonthly float64 `json:"total_monthly"`
}

// DiffItemDocument is one added, removed, or changed resource in a DiffDocument.
type DiffItemDocument struct {
	URN         string  `json:"urn"`
	Type        string  `json:"type,omitempty"`
	MonthlyCost float64 `json:"monthly_cost"`
	Delta       float64 `json:"delta"`
	SKU         string  `json:"sku,omitempty"`
	Region      string  `json:"region,omitempty"`
}

// DiffDocument is the `cost history diff --output json` document.
type DiffDocument struct {
	Stack          string             `json:"stack"`
	From           DiffPointDocument  `json:"from"`
	To             DiffPointDocument  `json:"to"`
	Delta          float64            `json:"delta"`
	Currency       string             `json:"currency"`
	Added          []DiffItemDocument `json:"added"`
	Removed        []DiffItemDocument `json:"removed"`
	Changed        []DiffItemDocument `json:"changed"`
	UnchangedCount int                `json:"unchanged_count"`
	UnchangedTotal float64            `json:"unchanged_total"`
}

// NewDiffDocument builds the diff document for stack. Timestamps are RFC 3339
// in UTC, and a zero timestamp is empty.
func NewDiffDocument(stack string, diff CostDiff) DiffDocument {
	return DiffDocument{
		Stack:          stack,
		From:           diffPointDocument(diff.From),
		To:             diffPointDocument(diff.To),
		Delta:          diff.Delta,
		Currency:       diff.Currency,
		Added:          diffItemDocuments(diff.Added),
		Removed:        diffItemDocuments(diff.Removed),
		Changed:        diffItemDocuments(diff.Changed),
		UnchangedCount: diff.UnchangedCount,
		UnchangedTotal: diff.UnchangedTotal,
	}
}

func diffPointDocument(point CostDiffPoint) DiffPointDocument {
	stamp := ""
	if !point.Timestamp.IsZero() {
		stamp = point.Timestamp.UTC().Format(time.RFC3339)
	}
	return DiffPointDocument{
		Version:      point.Version,
		Timestamp:    stamp,
		TotalMonthly: point.TotalMonthly,
	}
}

func diffItemDocuments(items []CostDiffItem) []DiffItemDocument {
	out := make([]DiffItemDocument, 0, len(items))
	for _, item := range items {
		out = append(out, DiffItemDocument{
			URN:         item.URN,
			Type:        item.Type,
			MonthlyCost: item.MonthlyCost,
			Delta:       item.Delta,
			SKU:         item.SKU,
			Region:      item.Region,
		})
	}
	return out
}
