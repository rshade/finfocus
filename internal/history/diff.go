package history

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	costChangeTolerance = 0.01
	hoursPerDay         = 24
	urnSeparator        = "::"
)

// CostDiffPoint is one end of a cost history diff.
type CostDiffPoint struct {
	Version      int
	Timestamp    time.Time
	TotalMonthly float64
}

// CostDiffItem is one added, removed, or changed resource.
// Delta is the signed monthly impact.
type CostDiffItem struct {
	URN         string
	Type        string
	SKU         string
	Region      string
	MonthlyCost float64
	Delta       float64
}

// CostDiff is the URN set comparison of two stored snapshots.
type CostDiff struct {
	From           CostDiffPoint
	To             CostDiffPoint
	Delta          float64
	Currency       string
	Added          []CostDiffItem
	Removed        []CostDiffItem
	Changed        []CostDiffItem
	UnchangedCount int
	UnchangedTotal float64
}

// DiffSnapshots compares resources by URN.
// A cost move of costChangeTolerance or less is unchanged.
// threshold 0 shows every categorized change. A positive threshold hides
// impacts that are not above that amount. A negative threshold is rejected.
func DiffSnapshots(from, to CostSnapshot, threshold float64) (CostDiff, error) {
	if threshold < 0 {
		return CostDiff{}, errors.New("threshold must be >= 0")
	}
	currency, err := diffCurrency(from, to)
	if err != nil {
		return CostDiff{}, err
	}
	added, removed, changed, unchangedTotal := classifyResources(from, to)
	added = shownItems(added, threshold)
	removed = shownItems(removed, threshold)
	changed = shownItems(changed, threshold)
	return CostDiff{
		From:           diffPoint(from),
		To:             diffPoint(to),
		Delta:          to.TotalMonthly - from.TotalMonthly,
		Currency:       currency,
		Added:          added,
		Removed:        removed,
		Changed:        changed,
		UnchangedCount: unchangedCount(from, to),
		UnchangedTotal: unchangedTotal,
	}, nil
}

// ResolveCostSnapshot selects a snapshot by version (35 or v35), by the
// nearest timestamp to a YYYY-MM-DD date, or the newest snapshot when spec
// is empty.
func ResolveCostSnapshot(snaps []CostSnapshot, spec string) (CostSnapshot, error) {
	if len(snaps) == 0 {
		return CostSnapshot{}, errors.New("no cost history snapshots")
	}
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return latestSnapshot(snaps), nil
	}
	if version, ok := parseVersionSpec(spec); ok {
		snap, found := snapshotByVersion(snaps, version)
		if !found {
			return CostSnapshot{}, fmt.Errorf("snapshot version %d not found", version)
		}
		return snap, nil
	}
	when, err := time.Parse("2006-01-02", spec)
	if err != nil {
		return CostSnapshot{}, fmt.Errorf("invalid snapshot selector %q (use v35 or YYYY-MM-DD)", spec)
	}
	return nearestSnapshot(snaps, when.UTC()), nil
}

// RenderCostDiff formats a table for cost history diff.
func RenderCostDiff(stack string, diff CostDiff) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Stack: %s\n", stack)
	fmt.Fprintf(&b, "Cost Delta: %s/mo (%s → %s)\n",
		signedMoney(diff.Delta), formatMoney(diff.From.TotalMonthly), formatMoney(diff.To.TotalMonthly))
	fmt.Fprintf(&b, "%s\n", formatDiffPeriod(diff))
	writeDiffSection(&b, "Added Resources", sumDelta(diff.Added), diff.Added)
	writeDiffSection(&b, "Removed Resources", sumDelta(diff.Removed), diff.Removed)
	writeDiffSection(&b, "Changed Resources", sumDelta(diff.Changed), diff.Changed)
	fmt.Fprintf(&b, "\nUnchanged Resources (%d):\n", diff.UnchangedCount)
	fmt.Fprintf(&b, "  %s/mo across %d resources\n", formatMoney(diff.UnchangedTotal), diff.UnchangedCount)
	return b.String()
}

func diffCurrency(from, to CostSnapshot) (string, error) {
	left := strings.ToUpper(strings.TrimSpace(from.Currency))
	right := strings.ToUpper(strings.TrimSpace(to.Currency))
	if left != "" && right != "" && left != right {
		return "", ErrMixedCurrencies
	}
	if right != "" {
		return right, nil
	}
	return left, nil
}

func classifyResources(from, to CostSnapshot) ([]CostDiffItem, []CostDiffItem, []CostDiffItem, float64) {
	var added, removed, changed []CostDiffItem
	var unchangedTotal float64
	fromMap := resourceMap(from.Resources)
	toMap := resourceMap(to.Resources)
	for urn, resource := range toMap {
		previous, ok := fromMap[urn]
		if !ok {
			added = append(added, itemFrom(resource, resource.MonthlyCost))
			continue
		}
		delta := resource.MonthlyCost - previous.MonthlyCost
		if math.Abs(delta) > costChangeTolerance {
			changed = append(changed, itemFrom(resource, delta))
			continue
		}
		unchangedTotal += resource.MonthlyCost
	}
	for urn, resource := range fromMap {
		if _, ok := toMap[urn]; ok {
			continue
		}
		removed = append(removed, itemFrom(resource, -resource.MonthlyCost))
	}
	return added, removed, changed, unchangedTotal
}

func unchangedCount(from, to CostSnapshot) int {
	fromMap := resourceMap(from.Resources)
	count := 0
	for urn, resource := range resourceMap(to.Resources) {
		previous, ok := fromMap[urn]
		if !ok {
			continue
		}
		if math.Abs(resource.MonthlyCost-previous.MonthlyCost) <= costChangeTolerance {
			count++
		}
	}
	return count
}

func resourceMap(resources []CostResource) map[string]CostResource {
	out := make(map[string]CostResource, len(resources))
	for _, resource := range resources {
		out[resource.URN] = resource
	}
	return out
}

func itemFrom(resource CostResource, delta float64) CostDiffItem {
	return CostDiffItem{
		URN:         resource.URN,
		Type:        resource.Type,
		SKU:         resource.SKU,
		Region:      resource.Region,
		MonthlyCost: resource.MonthlyCost,
		Delta:       delta,
	}
}

func shownItems(items []CostDiffItem, threshold float64) []CostDiffItem {
	kept := make([]CostDiffItem, 0, len(items))
	for _, item := range items {
		if threshold > 0 && math.Abs(item.Delta) <= threshold {
			continue
		}
		kept = append(kept, item)
	}
	sort.Slice(kept, func(i, j int) bool {
		left := math.Abs(kept[i].Delta)
		right := math.Abs(kept[j].Delta)
		if left != right {
			return left > right
		}
		return kept[i].URN < kept[j].URN
	})
	return kept
}

func diffPoint(snapshot CostSnapshot) CostDiffPoint {
	return CostDiffPoint{
		Version:      snapshot.Version,
		Timestamp:    snapshot.Timestamp,
		TotalMonthly: snapshot.TotalMonthly,
	}
}

func latestSnapshot(snaps []CostSnapshot) CostSnapshot {
	best := snaps[0]
	for _, snap := range snaps[1:] {
		if newerSnapshot(snap, best) {
			best = snap
		}
	}
	return best
}

func parseVersionSpec(spec string) (int, bool) {
	body := spec
	if strings.HasPrefix(strings.ToLower(body), "v") {
		body = body[1:]
	}
	if body == "" || strings.HasPrefix(body, "-") {
		return 0, false
	}
	version, err := strconv.Atoi(body)
	if err != nil || version < 0 {
		return 0, false
	}
	return version, true
}

func nearestSnapshot(snaps []CostSnapshot, target time.Time) CostSnapshot {
	best := snaps[0]
	for _, snap := range snaps[1:] {
		if nearerSnapshot(snap, best, target) {
			best = snap
		}
	}
	return best
}

func nearerSnapshot(candidate, best CostSnapshot, target time.Time) bool {
	cand := candidate.Timestamp.Sub(target).Abs()
	have := best.Timestamp.Sub(target).Abs()
	if cand != have {
		return cand < have
	}
	if !candidate.Timestamp.Equal(best.Timestamp) {
		return candidate.Timestamp.Before(best.Timestamp)
	}
	return candidate.Version < best.Version
}

func formatDiffPeriod(diff CostDiff) string {
	days := int(diff.To.Timestamp.Sub(diff.From.Timestamp).Abs().Hours() / hoursPerDay)
	versions := diff.To.Version - diff.From.Version
	if versions < 0 {
		versions = -versions
	}
	return fmt.Sprintf("Period: %s → %s (%d days, %d versions)",
		diff.From.Timestamp.UTC().Format("Jan 02, 2006"),
		diff.To.Timestamp.UTC().Format("Jan 02, 2006"),
		days, versions)
}

func writeDiffSection(b *strings.Builder, title string, total float64, items []CostDiffItem) {
	fmt.Fprintf(b, "\n%s (%s/mo):\n", title, signedMoney(total))
	if len(items) == 0 {
		fmt.Fprintln(b, "  (none detected)")
		return
	}
	for _, item := range items {
		fmt.Fprintf(b, "  %s  %s  %s/mo  (%s, %s)\n",
			item.Type, resourceLabel(item.URN), signedMoney(item.Delta), item.SKU, item.Region)
	}
}

func sumDelta(items []CostDiffItem) float64 {
	total := 0.0
	for _, item := range items {
		total += item.Delta
	}
	return total
}

func resourceLabel(urn string) string {
	if urn == "" {
		return "(unknown)"
	}
	if _, label, ok := strings.CutLast(urn, urnSeparator); ok {
		return label
	}
	return urn
}
