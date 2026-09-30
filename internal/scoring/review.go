package scoring

import (
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

const floatTolerance = 1e-9

// ReviewPolicy decides which scored recommendations are flagged "needs review".
// Scores are ranking signals whose values move slightly between identical calls, so each
// threshold is widened downward by half the dead band: a recommendation that scores
// inside the band around a threshold is still routed to a human. The flag never dismisses
// or applies anything.
type ReviewPolicy struct {
	Risk                 float64
	FalsePositive        float64
	InsufficientEvidence float64
	DeadBand             float64
}

// NewReviewPolicy builds a policy from resolved configuration.
func NewReviewPolicy(cfg config.ResolvedScoring) ReviewPolicy {
	return ReviewPolicy{
		Risk:                 cfg.RiskThreshold,
		FalsePositive:        cfg.FalsePositiveLimit,
		InsufficientEvidence: cfg.InsufficientEvidence,
		DeadBand:             cfg.DeadBand,
	}
}

// NeedsReview reports whether any signal reaches its threshold minus half the dead band.
func (p ReviewPolicy) NeedsReview(s *engine.RecommendationScores) bool {
	if s == nil {
		return false
	}
	half := p.DeadBand / 2 //nolint:mnd // half the band on each side of a threshold
	reaches := func(v *float64, threshold float64) bool {
		return v != nil && *v >= threshold-half-floatTolerance
	}
	return reaches(s.Risk, p.Risk) ||
		reaches(s.FalsePositive, p.FalsePositive) ||
		reaches(s.InsufficientEvidence, p.InsufficientEvidence)
}

// Mark sets NeedsReview on every recommendation that has scores.
func (p ReviewPolicy) Mark(recs []engine.Recommendation) {
	for i := range recs {
		if s := recs[i].Scores; s != nil {
			s.NeedsReview = p.NeedsReview(s)
		}
	}
}
