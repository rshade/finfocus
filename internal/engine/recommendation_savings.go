package engine

// savingsGroupKey identifies the recommendations of one resource and Type.
// position is -1 when resource is a real identifier and the caller's position
// when no identifier exists, so a positional fallback never equals a real
// resource ID.
type savingsGroupKey struct {
	resource string
	position int
	recType  string
}

// savingsGroup is a set of recommendations that change overlapping properties
// and are therefore alternatives; max is the largest EstimatedSavings among them.
type savingsGroup struct {
	properties map[string]bool
	max        float64
}

// savingsBucket holds the alternative groups of one resource and Type.
// Recommendations without property detail share the single undetailed group.
type savingsBucket struct {
	groups     []*savingsGroup
	undetailed float64
}

// SavingsGrouper totals the savings a user can actually realize.
//
// Recommendations for one resource are often alternatives for the same
// property (for example a generation upgrade and a Graviton change for the
// same instance), so their savings are not additive. They are grouped by
// resource and Type. Within a group, recommendations whose ActionDetail.Modify
// changes overlapping config keys (keys whose value differs between
// CurrentConfig and RecommendedConfig, or all RecommendedConfig keys when
// CurrentConfig is empty) are alternatives and only the largest
// EstimatedSavings counts. Recommendations that change disjoint keys are
// independent and sum. Recommendations without modify detail or changed keys
// share one alternative group per resource and Type. The group maxima are
// summed by Total.
//
// The zero value is not usable; create one with NewSavingsGrouper.
type SavingsGrouper struct {
	buckets map[savingsGroupKey]*savingsBucket
}

// NewSavingsGrouper returns an empty SavingsGrouper.
func NewSavingsGrouper() *SavingsGrouper {
	return &SavingsGrouper{buckets: make(map[savingsGroupKey]*savingsBucket)}
}

// Add records rec for resource. position keys a recommendation that has no
// resource identifier (resource is empty): recommendations with the same
// position and Type are grouped, and a positional key never equals a real
// resource ID.
func (g *SavingsGrouper) Add(resource string, position int, rec Recommendation) {
	key := savingsGroupKey{resource: resource, position: -1, recType: rec.Type}
	if resource == "" {
		key.position = position
	}
	bucket := g.buckets[key]
	if bucket == nil {
		bucket = &savingsBucket{}
		g.buckets[key] = bucket
	}
	bucket.add(changedProperties(rec), rec.EstimatedSavings)
}

// Total returns the sum of the largest alternative saving in every group.
func (g *SavingsGrouper) Total() float64 {
	var total float64
	for _, bucket := range g.buckets {
		total += bucket.total()
	}
	return total
}

// AchievableSavings returns the savings of recs that can all be realized at
// once, using the rule documented on SavingsGrouper. A recommendation with no
// ResourceID is its own position, so it is never treated as an alternative of
// another. It does not look at Status or Currency: callers drop dismissed or
// snoozed items first and withhold a total when currencies are mixed.
func AchievableSavings(recs []Recommendation) float64 {
	grouper := NewSavingsGrouper()
	for i, rec := range recs {
		grouper.Add(rec.ResourceID, i, rec)
	}
	return grouper.Total()
}

// changedProperties returns the config keys a modify recommendation changes: keys
// whose value differs between the current and recommended config, including keys
// present only in the recommended config. When the current config is empty, all
// recommended keys count. It returns nil when no keys can be determined.
func changedProperties(rec Recommendation) map[string]bool {
	if rec.ActionDetail == nil || rec.ActionDetail.Modify == nil {
		return nil
	}
	modify := rec.ActionDetail.Modify
	changed := make(map[string]bool)
	for key, recommended := range modify.RecommendedConfig {
		current, present := modify.CurrentConfig[key]
		if len(modify.CurrentConfig) == 0 || !present || current != recommended {
			changed[key] = true
		}
	}
	if len(changed) == 0 {
		return nil
	}
	return changed
}

func intersects(a, b map[string]bool) bool {
	for key := range a {
		if b[key] {
			return true
		}
	}
	return false
}

// add places savings into the group chain, merging every group whose properties
// overlap the given ones.
func (b *savingsBucket) add(properties map[string]bool, savings float64) {
	if properties == nil {
		b.undetailed = max(b.undetailed, savings)
		return
	}
	merged := &savingsGroup{properties: properties, max: savings}
	kept := make([]*savingsGroup, 0, len(b.groups)+1)
	for _, group := range b.groups {
		if !intersects(group.properties, properties) {
			kept = append(kept, group)
			continue
		}
		for key := range group.properties {
			merged.properties[key] = true
		}
		merged.max = max(merged.max, group.max)
	}
	kept = append(kept, merged)
	b.groups = kept
}

func (b *savingsBucket) total() float64 {
	total := b.undetailed
	for _, group := range b.groups {
		total += group.max
	}
	return total
}
