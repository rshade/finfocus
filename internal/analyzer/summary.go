package analyzer

import (
	"encoding/json"
	"errors"
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
	MixedCurrencies  bool           `json:"mixed_currencies"`
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
// BuildCostSummary aggregates a slice of engine.CostResult into a CostSummary for the given stack and project.
// 
// BuildCostSummary skips any cost entries that have a non-nil Error or notes that indicate an error, and includes
// only successful entries in the totals and resource list. The summary's Timestamp is set to the provided `now`
// or to the current UTC time if `now` is zero. The summary's Currency is taken from the first valid resource that
// provides one; MixedCurrencies is set to true when multiple currencies are present.
// 
// Parameters:
//   - costs: slice of engine.CostResult values to aggregate.
//   - stack: identifier for the stack being summarized.
//   - project: project name associated with the summary.
//   - now: time to use for the summary Timestamp; if zero, current UTC time is used.
// 
// Returns:
//   Pointer to a CostSummary containing schema version, timestamp, stack, project, total monthly cost,
//   currency, resource count, mixed-currencies flag, and per-resource cost entries.
func BuildCostSummary(costs []engine.CostResult, stack, project string, now time.Time) *CostSummary {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	summary := &CostSummary{
		SchemaVersion: costSummarySchemaVersion,
		Timestamp:     now.UTC().Format(time.RFC3339),
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
			// Set currency only from the first valid resource
			if len(currencies) == 1 {
				summary.Currency = c.Currency
			}
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
// WriteCostSummary writes the provided CostSummary as indented JSON to the given directory
// using an atomic write (temporary file and rename).
//
// The function creates the target directory with permissions 0750 if it does not exist,
// writes the summary to a temporary file with permissions 0600, ensures the file ends
// with a newline, and renames the temporary file to "last-cost-summary.json".
//
// If summary is nil, WriteCostSummary returns an error. It also returns wrapped errors
// for failures creating the directory, marshaling the summary, writing the temporary file,
// or renaming the temporary file (the temporary file is removed on rename failure).
func WriteCostSummary(summary *CostSummary, dir string) error {
	if summary == nil {
		return errors.New("nil summary")
	}

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