package history

import (
	"cmp"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	exportFormatJSON   = "json"
	exportFormatCSV    = "csv"
	exportFormatNDJSON = "ndjson"
	exportMoneyDigits  = 2
	// exportCSVFixedColumns is timestamp, version, total, resource count, and message.
	exportCSVFixedColumns = 5
)

// Export is the JSON document written by `cost history export`.
type Export struct {
	Stack       string             `json:"stack"`
	Currency    string             `json:"currency"`
	ExportedAt  time.Time          `json:"exported_at"`
	Snapshots   []ExportSnapshot   `json:"snapshots"`
	Annotations []ExportAnnotation `json:"annotations"`
}

// ExportSnapshot is one checkpoint in an export, without per-resource rows.
type ExportSnapshot struct {
	Timestamp     time.Time          `json:"timestamp"`
	Version       int                `json:"version"`
	TotalMonthly  float64            `json:"total_monthly"`
	ByProvider    map[string]float64 `json:"by_provider"`
	ResourceCount int                `json:"resource_count"`
}

// ExportAnnotation is a deployment note joined to its snapshot time.
type ExportAnnotation struct {
	Timestamp time.Time `json:"timestamp"`
	Version   int       `json:"version"`
	Message   string    `json:"message"`
}

// BuildExport copies snapshots into the export document.
// A non-empty provider keeps every snapshot and sets total_monthly to that
// provider's amount (0 when the snapshot has none). Snapshots are ordered by
// time, then version. exportedAt is stored in UTC.
func BuildExport(
	stack, currency, provider string,
	snapshots []CostSnapshot,
	annotations []CostAnnotation,
	exportedAt time.Time,
) Export {
	filtered := filterExportProvider(snapshots, provider)
	ordered := orderedSnapshots(filtered)
	return Export{
		Stack:       stack,
		Currency:    currency,
		ExportedAt:  exportedAt.UTC(),
		Snapshots:   exportSnapshots(ordered),
		Annotations: exportAnnotations(ordered, annotations),
	}
}

// WriteExport writes doc in json, csv, or ndjson.
// JSON is one indented document. CSV has a header row. NDJSON is one snapshot per line.
func WriteExport(w io.Writer, format string, doc Export) error {
	switch format {
	case exportFormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(doc)
	case exportFormatCSV:
		return writeExportCSV(w, doc)
	case exportFormatNDJSON:
		return writeExportNDJSON(w, doc)
	default:
		return fmt.Errorf("unsupported export format: %s (supported: json, csv, ndjson)", format)
	}
}

func filterExportProvider(snapshots []CostSnapshot, provider string) []CostSnapshot {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return snapshots
	}
	out := make([]CostSnapshot, len(snapshots))
	for i, snapshot := range snapshots {
		amount := providerAmount(snapshot.ByProvider, provider)
		snapshot.TotalMonthly = amount
		snapshot.ByProvider = map[string]float64{provider: amount}
		out[i] = snapshot
	}
	return out
}

func providerAmount(by map[string]float64, provider string) float64 {
	for key, amount := range by {
		if strings.EqualFold(key, provider) {
			return amount
		}
	}
	return 0
}

func exportSnapshots(snapshots []CostSnapshot) []ExportSnapshot {
	out := make([]ExportSnapshot, len(snapshots))
	for i, snapshot := range snapshots {
		out[i] = ExportSnapshot{
			Timestamp:     snapshot.Timestamp.UTC(),
			Version:       snapshot.Version,
			TotalMonthly:  snapshot.TotalMonthly,
			ByProvider:    copyProviders(snapshot.ByProvider),
			ResourceCount: snapshot.ResourceCount,
		}
	}
	return out
}

func copyProviders(by map[string]float64) map[string]float64 {
	if len(by) == 0 {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(by))
	maps.Copy(out, by)
	return out
}

func exportAnnotations(snapshots []CostSnapshot, annotations []CostAnnotation) []ExportAnnotation {
	when := make(map[int]time.Time, len(snapshots))
	for _, snapshot := range snapshots {
		if _, ok := when[snapshot.Version]; ok {
			continue
		}
		when[snapshot.Version] = snapshot.Timestamp.UTC()
	}
	out := make([]ExportAnnotation, 0, len(annotations))
	for _, annotation := range annotations {
		stamp, ok := when[annotation.Version]
		if !ok {
			continue
		}
		out = append(out, ExportAnnotation{
			Timestamp: stamp,
			Version:   annotation.Version,
			Message:   annotation.Message,
		})
	}
	slices.SortFunc(out, func(a, b ExportAnnotation) int {
		if version := cmp.Compare(a.Version, b.Version); version != 0 {
			return version
		}
		return strings.Compare(a.Message, b.Message)
	})
	return out
}

func writeExportNDJSON(w io.Writer, doc Export) error {
	encoder := json.NewEncoder(w)
	for _, snapshot := range doc.Snapshots {
		row := struct {
			Timestamp    time.Time `json:"timestamp"`
			Version      int       `json:"version"`
			TotalMonthly float64   `json:"total_monthly"`
		}{
			Timestamp:    snapshot.Timestamp,
			Version:      snapshot.Version,
			TotalMonthly: snapshot.TotalMonthly,
		}
		if err := encoder.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

func writeExportCSV(w io.Writer, doc Export) error {
	providers := exportProviders(doc.Snapshots)
	header := make([]string, 0, len(providers)+exportCSVFixedColumns)
	header = append(header, "timestamp", "version", "total_monthly")
	header = append(header, providers...)
	header = append(header, "resource_count", "message")

	writer := csv.NewWriter(w)
	if err := writer.Write(header); err != nil {
		return err
	}
	messages := messagesByVersion(doc.Annotations)
	for _, snapshot := range doc.Snapshots {
		row := make([]string, 0, len(header))
		row = append(row,
			snapshot.Timestamp.UTC().Format(time.RFC3339),
			strconv.Itoa(snapshot.Version),
			strconv.FormatFloat(snapshot.TotalMonthly, 'f', exportMoneyDigits, 64),
		)
		for _, provider := range providers {
			row = append(row, strconv.FormatFloat(snapshot.ByProvider[provider], 'f', exportMoneyDigits, 64))
		}
		row = append(row, strconv.Itoa(snapshot.ResourceCount), messages[snapshot.Version])
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func exportProviders(snapshots []ExportSnapshot) []string {
	seen := make(map[string]struct{})
	for _, snapshot := range snapshots {
		for provider := range snapshot.ByProvider {
			seen[provider] = struct{}{}
		}
	}
	providers := make([]string, 0, len(seen))
	for provider := range seen {
		providers = append(providers, provider)
	}
	slices.Sort(providers)
	return providers
}

func messagesByVersion(annotations []ExportAnnotation) map[int]string {
	grouped := make(map[int][]string)
	for _, annotation := range annotations {
		if annotation.Message == "" {
			continue
		}
		grouped[annotation.Version] = append(grouped[annotation.Version], annotation.Message)
	}
	out := make(map[int]string, len(grouped))
	for version, messages := range grouped {
		out[version] = strings.Join(messages, "; ")
	}
	return out
}

func sortSnapshots(snapshots []CostSnapshot) {
	slices.SortFunc(snapshots, func(a, b CostSnapshot) int {
		if a.Timestamp.Before(b.Timestamp) {
			return -1
		}
		if a.Timestamp.After(b.Timestamp) {
			return 1
		}
		return cmp.Compare(a.Version, b.Version)
	})
}
