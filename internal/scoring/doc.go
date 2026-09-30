// Package scoring runs the optional recommendation scoring step.
//
// After the plugin fetch, core can hand the recommendations to a scorer plugin that
// implements finfocus.v1.RecommendationScorerService. The scorer rates each
// recommendation (risk, false-positive likelihood, worth acting, priority, thin
// evidence, duplicate grouping). Scores route work to review and order lists; they never
// dismiss, apply or hide a recommendation.
//
// The step is off by default and adds no network activity of its own: the scorer is a
// plugin, and it owns any model or API access. Before a request leaves the host, this
// package applies the configured identifier mode (pseudonymized by default), the field
// allowlist, and per-request opaque recommendation ids. Scoring degrades gracefully: an
// unavailable, slow or misbehaving scorer yields unscored recommendations and a warning,
// never a failed command.
//
// Scores are cached in the BoltDB cache under the "scores" bucket, keyed on a hash of the
// recommendation content plus the scorer name, plugin version and model. Only extracted
// score values are stored, never raw scorer payloads.
package scoring
