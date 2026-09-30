package config

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const keyScoring = "scoring"

// Scoring identifier modes accepted by scoring.identifier_mode.
const (
	ScoringIdentifierPseudonymized = "pseudonymized"
	ScoringIdentifierRaw           = "raw"
	ScoringIdentifierOmitted       = "omitted"
)

// Scoring defaults applied when a setting is absent from the config file.
const (
	DefaultScoringTimeoutSeconds         = 30
	DefaultScoringReviewRisk             = 0.3
	DefaultScoringReviewFalsePositive    = 0.3
	DefaultScoringReviewEvidence         = 0.5
	DefaultScoringReviewDeadBand         = 0.1
	scoringMaxThreshold                  = 1.0
	scoringNeedsReviewSubsection         = "needs_review"
	scoringKeyEnabled                    = "enabled"
	scoringKeyPlugin                     = "plugin"
	scoringKeyIdentifierMode             = "identifier_mode"
	scoringKeyFieldAllowlist             = "field_allowlist"
	scoringKeyTimeoutSeconds             = "timeout_seconds"
	scoringReviewKeyRisk                 = "risk"
	scoringReviewKeyFalsePositive        = "false_positive"
	scoringReviewKeyInsufficientEvidence = "insufficient_evidence"
	scoringReviewKeyDeadBand             = "dead_band"
)

// ScoringFieldNames lists the recommendation fields a scorer may receive and that
// scoring.field_allowlist may name. The recommendation id is always sent.
//
//nolint:gochecknoglobals // Compile-time constant lookup table.
var ScoringFieldNames = []string{
	"category", "action_type", "resource", "impact", "priority", "confidence_score",
	"description", "reasoning", "source", "created_at", "metadata",
	"action_detail", "primary_reason", "secondary_reasons",
}

// ScoringConfig configures the optional recommendation scoring step. Scoring is off by
// default and never runs unless Enabled is true and Plugin names a scorer plugin.
//
// Example:
//
//	"scoring": {
//	  "enabled": true,
//	  "plugin": "my-scorer",
//	  "identifier_mode": "pseudonymized",
//	  "field_allowlist": ["category", "impact", "description"],
//	  "needs_review": {"risk": 0.3, "dead_band": 0.1}
//	}
type ScoringConfig struct {
	// Enabled turns scoring on. Default false.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Plugin is the name of the scorer plugin. Required when Enabled is true.
	Plugin string `yaml:"plugin,omitempty" json:"plugin,omitempty"`

	// IdentifierMode controls how resource ids and names reach the scorer:
	// "pseudonymized" (default), "omitted", or "raw" (explicit opt-in).
	IdentifierMode string `yaml:"identifier_mode,omitempty" json:"identifier_mode,omitempty"`

	// FieldAllowlist limits the recommendation fields sent to the scorer. Empty sends
	// every field. Names are listed in ScoringFieldNames; the id is always sent.
	FieldAllowlist []string `yaml:"field_allowlist,omitempty" json:"field_allowlist,omitempty"`

	// TimeoutSeconds bounds each scorer call. Default 30.
	TimeoutSeconds int `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`

	// NeedsReview sets the thresholds behind the "needs review" marker.
	NeedsReview *ScoringReviewConfig `yaml:"needs_review,omitempty" json:"needs_review,omitempty"`
}

// ScoringReviewConfig holds the thresholds for the "needs review" marker. Absent values
// use defaults. Scores are ranking signals, so tune thresholds per deployment.
type ScoringReviewConfig struct {
	// Risk marks a recommendation for review at or above this risk score.
	Risk *float64 `yaml:"risk,omitempty" json:"risk,omitempty"`
	// FalsePositive marks a recommendation for review at or above this false-positive score.
	FalsePositive *float64 `yaml:"false_positive,omitempty" json:"false_positive,omitempty"`
	// InsufficientEvidence marks a recommendation for review at or above this score.
	InsufficientEvidence *float64 `yaml:"insufficient_evidence,omitempty" json:"insufficient_evidence,omitempty"`
	// DeadBand widens every threshold downward by half its width, so a recommendation
	// scoring inside the band around a threshold is still routed to review.
	DeadBand *float64 `yaml:"dead_band,omitempty" json:"dead_band,omitempty"`
}

// ResolvedScoring is a ScoringConfig with every default applied.
type ResolvedScoring struct {
	Enabled              bool
	Plugin               string
	IdentifierMode       string
	FieldAllowlist       []string
	TimeoutSeconds       int
	RiskThreshold        float64
	FalsePositiveLimit   float64
	InsufficientEvidence float64
	DeadBand             float64
}

// Resolve returns the effective scoring settings with defaults applied. A nil receiver
// yields the disabled defaults.
func (s *ScoringConfig) Resolve() ResolvedScoring {
	out := ResolvedScoring{
		IdentifierMode:       ScoringIdentifierPseudonymized,
		TimeoutSeconds:       DefaultScoringTimeoutSeconds,
		RiskThreshold:        DefaultScoringReviewRisk,
		FalsePositiveLimit:   DefaultScoringReviewFalsePositive,
		InsufficientEvidence: DefaultScoringReviewEvidence,
		DeadBand:             DefaultScoringReviewDeadBand,
	}
	if s == nil {
		return out
	}
	out.Enabled = s.Enabled
	out.Plugin = s.Plugin
	out.FieldAllowlist = slices.Clone(s.FieldAllowlist)
	if mode := strings.ToLower(strings.TrimSpace(s.IdentifierMode)); mode != "" {
		out.IdentifierMode = mode
	}
	if s.TimeoutSeconds > 0 {
		out.TimeoutSeconds = s.TimeoutSeconds
	}
	if r := s.NeedsReview; r != nil {
		if r.Risk != nil {
			out.RiskThreshold = *r.Risk
		}
		if r.FalsePositive != nil {
			out.FalsePositiveLimit = *r.FalsePositive
		}
		if r.InsufficientEvidence != nil {
			out.InsufficientEvidence = *r.InsufficientEvidence
		}
		if r.DeadBand != nil {
			out.DeadBand = *r.DeadBand
		}
	}
	return out
}

// Validate checks the scoring settings.
func (s *ScoringConfig) Validate() error {
	if s == nil {
		return nil
	}
	if s.Enabled && strings.TrimSpace(s.Plugin) == "" {
		return errors.New("scoring.plugin is required when scoring.enabled is true")
	}
	switch strings.ToLower(strings.TrimSpace(s.IdentifierMode)) {
	case "", ScoringIdentifierPseudonymized, ScoringIdentifierRaw, ScoringIdentifierOmitted:
	default:
		return fmt.Errorf("invalid scoring.identifier_mode %q (must be pseudonymized, omitted, or raw)",
			s.IdentifierMode)
	}
	if s.TimeoutSeconds < 0 {
		return fmt.Errorf("invalid scoring.timeout_seconds %d (must not be negative)", s.TimeoutSeconds)
	}
	for _, field := range s.FieldAllowlist {
		if !slices.Contains(ScoringFieldNames, field) {
			return fmt.Errorf("invalid scoring.field_allowlist entry %q (valid: %s)",
				field, strings.Join(ScoringFieldNames, ", "))
		}
	}
	if r := s.NeedsReview; r != nil {
		for name, v := range map[string]*float64{
			scoringReviewKeyRisk:                 r.Risk,
			scoringReviewKeyFalsePositive:        r.FalsePositive,
			scoringReviewKeyInsufficientEvidence: r.InsufficientEvidence,
			scoringReviewKeyDeadBand:             r.DeadBand,
		} {
			if v != nil && (*v < 0 || *v > scoringMaxThreshold) {
				return fmt.Errorf("invalid scoring.needs_review.%s %v (must be between 0 and 1)", name, *v)
			}
		}
	}
	return nil
}

func (c *Config) ensureScoring() *ScoringConfig {
	if c.Scoring == nil {
		c.Scoring = &ScoringConfig{}
	}
	return c.Scoring
}

func (c *Config) getScoringValue(parts []string) (any, error) {
	if len(parts) < 1 {
		return c.Scoring, nil
	}
	resolved := c.Scoring.Resolve()
	switch parts[0] {
	case scoringKeyEnabled:
		return resolved.Enabled, nil
	case scoringKeyPlugin:
		return resolved.Plugin, nil
	case scoringKeyIdentifierMode:
		return resolved.IdentifierMode, nil
	case scoringKeyFieldAllowlist:
		return resolved.FieldAllowlist, nil
	case scoringKeyTimeoutSeconds:
		return resolved.TimeoutSeconds, nil
	case scoringNeedsReviewSubsection:
		return c.getScoringReviewValue(parts[1:], resolved)
	default:
		return nil, fmt.Errorf("unknown scoring setting: %s", parts[0])
	}
}

func (c *Config) getScoringReviewValue(parts []string, resolved ResolvedScoring) (any, error) {
	if len(parts) < 1 {
		return map[string]float64{
			scoringReviewKeyRisk:                 resolved.RiskThreshold,
			scoringReviewKeyFalsePositive:        resolved.FalsePositiveLimit,
			scoringReviewKeyInsufficientEvidence: resolved.InsufficientEvidence,
			scoringReviewKeyDeadBand:             resolved.DeadBand,
		}, nil
	}
	switch parts[0] {
	case scoringReviewKeyRisk:
		return resolved.RiskThreshold, nil
	case scoringReviewKeyFalsePositive:
		return resolved.FalsePositiveLimit, nil
	case scoringReviewKeyInsufficientEvidence:
		return resolved.InsufficientEvidence, nil
	case scoringReviewKeyDeadBand:
		return resolved.DeadBand, nil
	default:
		return nil, fmt.Errorf("unknown scoring.needs_review setting: %s", parts[0])
	}
}

func (c *Config) setScoringValue(parts []string, value string) error {
	if len(parts) < 1 {
		return errors.New("scoring key required (e.g. scoring.enabled)")
	}
	next := *c.ensureScoring()
	next.FieldAllowlist = slices.Clone(next.FieldAllowlist)
	if r := next.NeedsReview; r != nil {
		copied := *r
		next.NeedsReview = &copied
	}

	switch parts[0] {
	case scoringKeyEnabled:
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid scoring.enabled %q: %w", value, err)
		}
		next.Enabled = enabled
	case scoringKeyPlugin:
		next.Plugin = value
	case scoringKeyIdentifierMode:
		next.IdentifierMode = value
	case scoringKeyTimeoutSeconds:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid scoring.timeout_seconds %q: %w", value, err)
		}
		next.TimeoutSeconds = n
	case scoringKeyFieldAllowlist:
		next.FieldAllowlist = nil
		for f := range strings.SplitSeq(value, ",") {
			if f = strings.TrimSpace(f); f != "" {
				next.FieldAllowlist = append(next.FieldAllowlist, f)
			}
		}
	case scoringNeedsReviewSubsection:
		if err := setScoringReviewValue(&next, parts[1:], value); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown scoring setting: %s", parts[0])
	}

	probe := next
	probe.Enabled = false
	if err := probe.Validate(); err != nil {
		return err
	}
	*c.ensureScoring() = next
	return nil
}

func setScoringReviewValue(s *ScoringConfig, parts []string, value string) error {
	if len(parts) != 1 {
		return errors.New("scoring.needs_review key required (risk, false_positive, insufficient_evidence, dead_band)")
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("invalid scoring.needs_review.%s %q: %w", parts[0], value, err)
	}
	if s.NeedsReview == nil {
		s.NeedsReview = &ScoringReviewConfig{}
	}
	switch parts[0] {
	case scoringReviewKeyRisk:
		s.NeedsReview.Risk = &v
	case scoringReviewKeyFalsePositive:
		s.NeedsReview.FalsePositive = &v
	case scoringReviewKeyInsufficientEvidence:
		s.NeedsReview.InsufficientEvidence = &v
	case scoringReviewKeyDeadBand:
		s.NeedsReview.DeadBand = &v
	default:
		return fmt.Errorf("unknown scoring.needs_review setting: %s", parts[0])
	}
	return nil
}
