package scoring

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

const (
	tokenHexLen    = 16
	minScrubLength = 3
	redacted       = "[redacted]"
	hashIDToken    = "[id]"
	hashNameToken  = "[name]"
	keyBytes       = 32
)

// identifiers applies the identifier mode to resource ids, names and recommendation ids.
type identifiers struct {
	mode    string
	key     []byte
	hashing bool
}

// newIdentifiers returns a transformer with a fresh random per-request key.
func newIdentifiers(mode string) (*identifiers, error) {
	key := make([]byte, keyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating pseudonymization key: %w", err)
	}
	return &identifiers{mode: mode, key: key}, nil
}

// forHashing returns a transformer that replaces identifiers with fixed placeholders, so
// content hashes do not depend on the per-request key.
func forHashing(mode string) *identifiers {
	return &identifiers{mode: mode, hashing: true}
}

// normalizeID makes ids that differ only in case or padding pseudonymize identically.
func normalizeID(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func (i *identifiers) token(prefix, raw string) string {
	mac := hmac.New(sha256.New, i.key)
	mac.Write([]byte(prefix))
	mac.Write([]byte{0})
	mac.Write([]byte(normalizeID(raw)))
	return prefix + "-" + hex.EncodeToString(mac.Sum(nil))[:tokenHexLen]
}

func (i *identifiers) resourceID(raw string) string {
	switch {
	case raw == "" || i.mode == config.ScoringIdentifierRaw:
		return raw
	case i.mode == config.ScoringIdentifierOmitted:
		return ""
	case i.hashing:
		return hashIDToken
	default:
		return i.token("res", raw)
	}
}

func (i *identifiers) resourceName(raw string) string {
	switch {
	case raw == "" || i.mode == config.ScoringIdentifierRaw:
		return raw
	case i.mode == config.ScoringIdentifierOmitted:
		return ""
	case i.hashing:
		return hashNameToken
	default:
		return i.token("name", raw)
	}
}

// scrubber replaces raw resource ids and names inside free text and map values.
type scrubber struct {
	pairs []string
}

func newScrubber(ids *identifiers, rawID, rawName string) scrubber {
	if ids.mode == config.ScoringIdentifierRaw {
		return scrubber{}
	}
	var pairs []string
	add := func(raw, replacement string) {
		if len(raw) >= minScrubLength {
			pairs = append(pairs, raw, replacement)
		}
	}
	idRepl, nameRepl := ids.resourceID(rawID), ids.resourceName(rawName)
	if idRepl == "" {
		idRepl = redacted
	}
	if nameRepl == "" {
		nameRepl = redacted
	}
	add(rawID, idRepl)
	add(rawName, nameRepl)
	return scrubber{pairs: pairs}
}

func (s scrubber) text(v string) string {
	if len(s.pairs) == 0 {
		return v
	}
	return strings.NewReplacer(s.pairs...).Replace(v)
}

func (s scrubber) texts(vs []string) []string {
	if len(vs) == 0 {
		return vs
	}
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = s.text(v)
	}
	return out
}

func (s scrubber) mapValues(m map[string]string) map[string]string {
	if len(m) == 0 || len(s.pairs) == 0 {
		return m
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = s.text(v)
	}
	return out
}

// toProto converts an engine recommendation to the wire message with the identifier mode
// applied. The recommendation id is left as the plugin assigned it; callers replace it.
// It returns false when the recommendation has no usable id.
func toProto(rec engine.Recommendation, ids *identifiers) (*pbc.Recommendation, bool) {
	if rec.ID == "" {
		return nil, false
	}

	rawID := rec.ResourceID
	rawName := ""
	if rec.ResourceInfo != nil {
		rawName = rec.ResourceInfo.Name
	}
	scrub := newScrubber(ids, rawID, rawName)

	out := &pbc.Recommendation{
		Id:          rec.ID,
		Description: scrub.text(rec.Description),
		Reasoning:   scrub.texts(rec.Reasoning),
		Source:      rec.Source,
		Metadata:    scrub.mapValues(rec.Metadata),
		Category:    enumValue(pbc.RecommendationCategory_value, rec.Category, pbc.RecommendationCategory(0)),
		ActionType:  enumValue(pbc.RecommendationActionType_value, rec.Type, pbc.RecommendationActionType(0)),
		Priority:    enumValue(pbc.RecommendationPriority_value, rec.Priority, pbc.RecommendationPriority(0)),
	}
	out.ConfidenceScore = rec.ConfidenceScore
	if rec.CreatedAt != nil {
		out.CreatedAt = timestamppb.New(*rec.CreatedAt)
	}

	out.Impact = impactToProto(rec)
	out.Resource = resourceToProto(rec, ids, scrub)
	return out, true
}

func enumValue[E ~int32](values map[string]int32, name string, zero E) E {
	if v, ok := values[name]; ok {
		return E(v)
	}
	return zero
}

func impactToProto(rec engine.Recommendation) *pbc.RecommendationImpact {
	if rec.ImpactDetail == nil && rec.EstimatedSavings == 0 && rec.Currency == "" {
		return nil
	}
	impact := &pbc.RecommendationImpact{
		EstimatedSavings: rec.EstimatedSavings,
		Currency:         rec.Currency,
	}
	if d := rec.ImpactDetail; d != nil {
		impact.ProjectionPeriod = d.ProjectionPeriod
		impact.CurrentCost = d.CurrentCost
		impact.ProjectedCost = d.ProjectedCost
		impact.SavingsPercentage = d.SavingsPercentage
		impact.ImplementationCost = d.ImplementationCost
		impact.MigrationEffortHours = d.MigrationEffortHours
	}
	return impact
}

func resourceToProto(rec engine.Recommendation, ids *identifiers, scrub scrubber) *pbc.ResourceRecommendationInfo {
	if rec.ResourceInfo == nil && rec.ResourceID == "" {
		return nil
	}
	res := &pbc.ResourceRecommendationInfo{Id: ids.resourceID(rec.ResourceID)}
	info := rec.ResourceInfo
	if info == nil {
		return res
	}
	res.Name = ids.resourceName(info.Name)
	res.Provider = info.Provider
	res.ResourceType = info.ResourceType
	res.Region = info.Region
	res.Sku = info.SKU
	res.Tags = scrub.mapValues(info.Tags)
	if u := info.Utilization; u != nil {
		res.Utilization = &pbc.ResourceUtilization{
			CpuPercent:     u.CPUPercent,
			MemoryPercent:  u.MemoryPercent,
			StoragePercent: u.StoragePercent,
			NetworkInMbps:  u.NetworkInMbps,
			NetworkOutMbps: u.NetworkOutMbps,
			CustomMetrics:  u.CustomMetrics,
		}
	}
	return res
}

// applyAllowlist clears every optional field not named in the allowlist. An empty
// allowlist keeps everything. The recommendation id is never cleared.
func applyAllowlist(rec *pbc.Recommendation, allowlist []string) {
	if len(allowlist) == 0 {
		return
	}
	keep := func(name string) bool { return slices.Contains(allowlist, name) }
	if !keep("category") {
		rec.Category = 0
	}
	if !keep("action_type") {
		rec.ActionType = 0
	}
	if !keep("resource") {
		rec.Resource = nil
	}
	if !keep("impact") {
		rec.Impact = nil
	}
	if !keep("priority") {
		rec.Priority = 0
	}
	if !keep("confidence_score") {
		rec.ConfidenceScore = nil
	}
	if !keep("description") {
		rec.Description = ""
	}
	if !keep("reasoning") {
		rec.Reasoning = nil
	}
	if !keep("source") {
		rec.Source = ""
	}
	if !keep("created_at") {
		rec.CreatedAt = nil
	}
	if !keep("metadata") {
		rec.Metadata = nil
	}
}

func identifierModeProto(mode string) pbc.IdentifierMode {
	switch mode {
	case config.ScoringIdentifierRaw:
		return pbc.IdentifierMode_IDENTIFIER_MODE_RAW
	case config.ScoringIdentifierOmitted:
		return pbc.IdentifierMode_IDENTIFIER_MODE_OMITTED
	default:
		return pbc.IdentifierMode_IDENTIFIER_MODE_PSEUDONYMIZED
	}
}
