package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"
)

var (
	// ErrNoPlugin means a checkpoint has a resource the plugin pipeline did not price.
	ErrNoPlugin = errors.New("no plugin available for resource type")
	// ErrSkipVersion means one export failed after retries and collect continues.
	ErrSkipVersion = errors.New("skip version")
	// ErrEncryptedProperty means a pricing property is a Pulumi secret.
	ErrEncryptedProperty = errors.New("required property")
	// ErrVersionResetDeclined means the user declined to clear a recreated stack's history.
	ErrVersionResetDeclined = errors.New("stack version reset detected")
	// ErrMixedCurrencies means one checkpoint priced resources in more than one currency,
	// or a strict history view spans snapshots in more than one currency.
	ErrMixedCurrencies = errors.New("mixed currencies not supported")
)

const (
	historyResultSucceeded = "succeeded"
	historyKindUpdate      = "update"
	historyKindDestroy     = "destroy"
	destroyAnnotation      = "Stack destroyed"
	defaultCurrency        = "USD"
	unpricedAdapter        = "none"
	unpricedNote           = "No pricing information available"
	secretSignature        = "4dabf18193072939515e22adb298388d" //nolint:gosec // Pulumi secret marker, not a credential
	costDBSuffix           = ".history.db"
)

// PriceResource is one custom resource from a stack export, ready to price.
type PriceResource struct {
	ID         string
	Type       string
	Provider   string
	Properties map[string]any
}

// PriceQuote is one plugin result. Failed is set when the result carries an error.
type PriceQuote struct {
	ResourceID   string
	ResourceType string
	Adapter      string
	Currency     string
	Notes        string
	Monthly      float64
	Failed       bool
}

// ParseStackHistory decodes `pulumi stack history --json`.
func ParseStackHistory(data []byte) ([]StackUpdate, error) {
	var raw []stackHistoryEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing stack history: %w", err)
	}
	updates := make([]StackUpdate, 0, len(raw))
	for _, entry := range raw {
		start, err := parseHistoryTime(entry.StartTime)
		if err != nil {
			return nil, fmt.Errorf("parsing stack history version %d time: %w", entry.Version, err)
		}
		updates = append(updates, StackUpdate{
			Version:         entry.Version,
			Kind:            entry.Kind,
			Start:           start,
			Message:         entry.Message,
			Result:          entry.Result,
			ResourceChanges: cloneChanges(entry.ResourceChanges),
		})
	}
	return updates, nil
}

type stackHistoryEntry struct {
	Version         int            `json:"version"`
	Kind            string         `json:"kind"`
	StartTime       string         `json:"startTime"`
	Message         string         `json:"message"`
	Result          string         `json:"result"`
	ResourceChanges map[string]int `json:"resourceChanges"`
}

func parseHistoryTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp %q", value)
}

// SelectOptions chooses which checkpoints collect stores.
// Have, when non-nil, is the set of versions that already have snapshots.
// LastVersion is the high-water drop used only when Have is nil.
type SelectOptions struct {
	From        time.Time
	LastVersion uint64
	Limit       int
	SkipDestroy bool
	Have        map[int]struct{}
}

// SelectUpdates keeps successful updates, and successful destroys unless skipped.
// From is inclusive at 00:00 UTC when the caller parsed a date-only flag.
// Limit keeps the newest N matches. Stored versions in Have are then dropped.
// When Have is nil, versions at or below LastVersion are dropped instead.
func SelectUpdates(updates []StackUpdate, opt SelectOptions) []StackUpdate {
	kept := make([]StackUpdate, 0, len(updates))
	for _, update := range updates {
		if !keepUpdate(update, opt.SkipDestroy) {
			continue
		}
		if !opt.From.IsZero() && update.Start.Before(opt.From) {
			continue
		}
		kept = append(kept, update)
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].Version < kept[j].Version })
	if opt.Limit > 0 && len(kept) > opt.Limit {
		kept = kept[len(kept)-opt.Limit:]
	}
	selected := make([]StackUpdate, 0, len(kept))
	for _, update := range kept {
		if alreadyCollected(update.Version, opt) {
			continue
		}
		selected = append(selected, update)
	}
	return selected
}

func alreadyCollected(version int, opt SelectOptions) bool {
	if opt.Have != nil {
		_, ok := opt.Have[version]
		return ok
	}
	return version >= 0 && uint64(version) <= opt.LastVersion
}

func keepUpdate(update StackUpdate, skipDestroy bool) bool {
	if update.Result != historyResultSucceeded {
		return false
	}
	if update.Kind == historyKindUpdate {
		return true
	}
	return update.Kind == historyKindDestroy && !skipDestroy
}

// MaxVersion returns the highest checkpoint number in updates, or 0 when empty.
func MaxVersion(updates []StackUpdate) int {
	maxVersion := 0
	for _, update := range updates {
		if update.Version > maxVersion {
			maxVersion = update.Version
		}
	}
	return maxVersion
}

// VersionReset reports that a recreated stack's history is older than the database.
// An empty history is not a reset.
func VersionReset(maxVersion int, last uint64) bool {
	if last == 0 || maxVersion <= 0 {
		return false
	}
	return uint64(maxVersion) < last
}

// CostFileName maps a Pulumi stack name to `<sanitized>.history.db`.
// Slashes become dashes. Names containing `..` are rejected.
func CostFileName(stack string) (string, error) {
	stack = strings.TrimSpace(stack)
	if stack == "" || strings.Contains(stack, "..") || strings.ContainsAny(stack, " \t\r\n") {
		return "", fmt.Errorf("invalid stack name %q", stack)
	}
	name := strings.NewReplacer("/", "-", "\\", "-").Replace(stack)
	name = strings.Trim(name, "-.")
	if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid stack name %q", stack)
	}
	return name + costDBSuffix, nil
}

// IsCostHistoryFile reports whether name is a per-stack cost timeline database.
// The resource-observation file history.db does not match.
func IsCostHistoryFile(name string) bool {
	return strings.HasSuffix(name, costDBSuffix) && name != costDBSuffix
}

// EncryptedProperty returns the first pricing property stored as a Pulumi secret.
func EncryptedProperty(resources []PriceResource) string {
	keys := []string{
		"instanceType", "instance_type", "dbInstanceClass", "machineType",
		"machine_type", "vmSize", "sku", "nodeType",
	}
	for _, resource := range resources {
		for _, key := range keys {
			if isPulumiSecret(resource.Properties[key]) {
				return key
			}
		}
	}
	return ""
}

func isPulumiSecret(value any) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	_, ok = object[secretSignature]
	return ok
}

// SnapshotFromResults aligns plugin results to resources by ResourceID.
// A real plugin result of $0 is stored. Adapter none, the unpriced note, or a
// structured error fails the checkpoint.
func SnapshotFromResults(
	when time.Time,
	version int,
	resources []PriceResource,
	results []PriceQuote,
) (CostSnapshot, error) {
	byID := make(map[string]PriceQuote, len(results))
	for _, result := range results {
		if result.ResourceID != "" {
			byID[result.ResourceID] = result
		}
	}
	snapshot := ZeroSnapshot(when, version)
	currency := ""
	for _, resource := range resources {
		result, ok := byID[resource.ID]
		if !ok || resultUnpriced(result) {
			resourceType := resource.Type
			if ok && result.ResourceType != "" {
				resourceType = result.ResourceType
			}
			return CostSnapshot{}, pluginMissing(resourceType)
		}
		next, err := mergeCurrency(currency, result.Currency)
		if err != nil {
			return CostSnapshot{}, err
		}
		currency = next
		sku, region := skuRegion(resource)
		snapshot.Resources = append(snapshot.Resources, CostResource{
			URN:         resource.ID,
			Type:        resource.Type,
			Provider:    resource.Provider,
			MonthlyCost: result.Monthly,
			SKU:         sku,
			Region:      region,
		})
		snapshot.TotalMonthly += result.Monthly
		addAmount(snapshot.ByProvider, resource.Provider, result.Monthly)
		addAmount(snapshot.ByType, typeKey(resource.Type), result.Monthly)
	}
	snapshot.ResourceCount = len(snapshot.Resources)
	if currency != "" {
		snapshot.Currency = currency
	}
	return snapshot, nil
}

func mergeCurrency(current, next string) (string, error) {
	if next == "" {
		return current, nil
	}
	if current == "" || current == next {
		return next, nil
	}
	return "", fmt.Errorf("%w: found %s and %s", ErrMixedCurrencies, current, next)
}

func resultUnpriced(result PriceQuote) bool {
	if result.Failed {
		return true
	}
	if result.Adapter == "" || result.Adapter == unpricedAdapter {
		return true
	}
	return strings.Contains(result.Notes, unpricedNote)
}

func pluginMissing(resourceType string) error {
	message := "%w '%s'. Install the required plugin first."
	return fmt.Errorf(message, ErrNoPlugin, resourceType) //nolint:staticcheck // user-facing sentence
}

// EncryptedError is the user-facing failure for a secret pricing property.
func EncryptedError(name string) error {
	message := "%w '%s' is encrypted. Please file a GitHub issue if this is unexpected."
	return fmt.Errorf(message, ErrEncryptedProperty, name) //nolint:staticcheck // user-facing sentence
}

// VersionResetWarning is the confirmation prompt for a recreated stack.
func VersionResetWarning(maxVersion int, last uint64) string {
	return fmt.Sprintf("Warning: stack version reset detected (v%d after v%d). Reset history? [y/N]", maxVersion, last)
}

// ZeroSnapshot is an empty checkpoint stored for a stack with no custom resources
// and for a destroy.
func ZeroSnapshot(when time.Time, version int) CostSnapshot {
	if when.IsZero() {
		when = time.Unix(0, 0)
	}
	return CostSnapshot{
		Timestamp:    when.UTC(),
		Version:      version,
		Currency:     defaultCurrency,
		ByProvider:   map[string]float64{},
		ByType:       map[string]float64{},
		Resources:    []CostResource{},
		TotalMonthly: 0,
	}
}

// AnnotationFrom copies a history row. Destroys use the fixed message.
func AnnotationFrom(update StackUpdate) CostAnnotation {
	message := update.Message
	if update.Kind == historyKindDestroy {
		message = destroyAnnotation
	}
	return CostAnnotation{
		Version:         update.Version,
		Message:         message,
		Kind:            update.Kind,
		ResourceChanges: cloneChanges(update.ResourceChanges),
	}
}

func cloneChanges(changes map[string]int) map[string]int {
	if len(changes) == 0 {
		return map[string]int{}
	}
	copied := make(map[string]int, len(changes))
	maps.Copy(copied, changes)
	return copied
}

func addAmount(totals map[string]float64, key string, amount float64) {
	if key == "" {
		key = "unknown"
	}
	totals[key] += amount
}

func skuRegion(resource PriceResource) (string, string) {
	sku := firstString(resource.Properties, []string{
		"instanceType", "instance_type", "dbInstanceClass", "machineType",
		"machine_type", "vmSize", "sku", "nodeType", "size",
	})
	region := firstString(resource.Properties, []string{"region", "location"})
	if region == "" {
		zone := firstString(resource.Properties, []string{"availabilityZone", "availability_zone"})
		if len(zone) > 1 {
			region = zone[:len(zone)-1]
		}
	}
	return sku, region
}

func firstString(properties map[string]any, keys []string) string {
	for _, key := range keys {
		value, ok := properties[key].(string)
		if ok && value != "" {
			return value
		}
	}
	return ""
}

func typeKey(pulumiType string) string {
	rest := pulumiType
	if i := strings.Index(rest, ":"); i >= 0 {
		rest = rest[i+1:]
	}
	module, name, ok := strings.Cut(rest, "/")
	if !ok {
		return pulumiType
	}
	if i := strings.LastIndex(name, ":"); i >= 0 {
		name = name[i+1:]
	}
	return module + ":" + name
}
