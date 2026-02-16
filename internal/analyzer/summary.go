package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rshade/finfocus/internal/engine"
)

const (
	// costSummarySchemaVersion is the current schema version for cost summary files.
	costSummarySchemaVersion = "1"

	// costSummaryFilename is the default filename for cost summary output.
	costSummaryFilename = "last-cost-summary.json"
)

// CostSummary represents the structured cost summary written after each AnalyzeStack call.
// It follows the schema defined in contracts/cost-summary-schema.json.
type CostSummary struct {
	SchemaVersion    string         `json:"schema_version"`
	Timestamp        string         `json:"timestamp"`
	Stack            string         `json:"stack"`
	Project          string         `json:"project"`
	TotalMonthlyCost float64        `json:"total_monthly_cost"`
	Currency         string         `json:"currency"`
	ResourceCount    int            `json:"resource_count"`
	MixedCurrencies  bool           `json:"mixed_currencies,omitempty"`
	Resources        []ResourceCost `json:"resources"`
}

// ResourceCost represents the cost breakdown for a single resource.
type ResourceCost struct {
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	MonthlyCost float64 `json:"monthly_cost"`
	Currency    string  `json:"currency"`
	Adapter     string  `json:"adapter"`
}

// BuildCostSummary aggregates cost results into a CostSummary struct.
// Error resources (those with non-nil Error field or ERROR:/VALIDATION: prefixed notes)
// are excluded from the total and resource list. Mixed currencies are detected and flagged.
func BuildCostSummary(costs []engine.CostResult, stack, project string) *CostSummary {
	summary := &CostSummary{
		SchemaVersion: costSummarySchemaVersion,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Stack:         stack,
		Project:       project,
		Currency:      defaultCurrency,
		Resources:     make([]ResourceCost, 0),
	}

	currencies := make(map[string]bool)

	for _, c := range costs {
		// Skip error resources
		if c.Error != nil || isErrorNote(c.Notes) {
			continue
		}

		summary.TotalMonthlyCost += c.Monthly
		summary.ResourceCount++

		if c.Currency != "" {
			currencies[c.Currency] = true
			summary.Currency = c.Currency
		}

		summary.Resources = append(summary.Resources, ResourceCost{
			Type:        c.ResourceType,
			Name:        c.ResourceID,
			MonthlyCost: c.Monthly,
			Currency:    c.Currency,
			Adapter:     c.Adapter,
		})
	}

	if len(currencies) > 1 {
		summary.MixedCurrencies = true
	}

	return summary
}

// WriteCostSummary writes the cost summary to the specified directory using an
// atomic write pattern (temp file + rename). The directory is created if it
// does not exist. The output file has permissions 0o600 (owner read/write only).
func WriteCostSummary(summary *CostSummary, dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating summary directory: %w", err)
	}

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cost summary: %w", err)
	}
	// Ensure file ends with newline
	data = append(data, '\n')

	targetPath := filepath.Join(dir, costSummaryFilename)
	tmpPath := targetPath + ".tmp"

	if writeErr := os.WriteFile(tmpPath, data, 0o600); writeErr != nil {
		return fmt.Errorf("writing cost summary temp file: %w", writeErr)
	}

	if renameErr := os.Rename(tmpPath, targetPath); renameErr != nil {
		// Clean up temp file on rename failure
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming cost summary temp file: %w", renameErr)
	}

	return nil
}
