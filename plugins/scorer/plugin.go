// Package scorer is a finfocus scorer plugin. It implements
// RecommendationScorerService by asking the selected Jev or Clef backend
// to judge recommendations that other plugins produced. Scores rank; they are
// never approval to act.
package scorer

import (
	"context"
	"strconv"
	"time"

	"github.com/rshade/finfocus/plugins/scorer/internal/scoring"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// PluginName is the registry and binary name suffix.
const PluginName = "scorer"

// Plugin serves ScoreRecommendations.
type Plugin struct {
	*pluginsdk.BasePlugin

	scorer *scoring.Scorer
}

// New builds the plugin. A missing API key is not an error: the plugin starts
// and every scoring call returns UNAUTHENTICATED. An invalid base URL is.
func New(cfg Config) (*Plugin, error) {
	scorerCfg := scoring.Config{
		Model:              cfg.Model,
		BatchSize:          cfg.BatchSize,
		DuplicateThreshold: cfg.DuplicateThreshold,
		Logger:             cfg.Logger,
	}
	backend, err := newBackend(cfg)
	if err != nil {
		return nil, err
	}
	return &Plugin{
		BasePlugin: pluginsdk.NewBasePlugin(PluginName),
		scorer:     scoring.New(backend, scorerCfg),
	}, nil
}

// Info declares capabilities explicitly so hosts never route price queries
// here.
func Info(version string) *pluginsdk.PluginInfo {
	return pluginsdk.NewPluginInfo(PluginName, version,
		pluginsdk.WithProviders("*"),
		pluginsdk.WithCapabilities(pbc.PluginCapability_PLUGIN_CAPABILITY_RECOMMENDATION_SCORING),
		pluginsdk.WithSettings(
			&pbc.PluginSetting{
				Key: "backend", Type: pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING,
				DefaultValue: backendJev, Description: "Scoring backend: jev or clef.", Env: EnvBackend,
			},
			&pbc.PluginSetting{
				Key: "clef_account_id", Type: pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING,
				Description: "Cloudflare account ID; required with a Clef token.", Env: EnvClefAccountID,
			},
			&pbc.PluginSetting{
				Key:         "api_key",
				Type:        pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING,
				Required:    false,
				Secret:      true,
				Description: "API key or token for the selected backend.",
				Env:         EnvAPIKey,
			},
			&pbc.PluginSetting{
				Key:          "base_url",
				Type:         pbc.PluginSettingType_PLUGIN_SETTING_TYPE_URL,
				DefaultValue: "",
				Description:  "Backend origin; defaults to TypeSafe AI for Jev or Cloudflare for Clef.",
				Env:          EnvBaseURL,
			},
			&pbc.PluginSetting{
				Key:          "model",
				Type:         pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING,
				DefaultValue: "",
				Description:  "Model id; defaults to jev-1.13.0 for Jev or clef for Clef.",
				Env:          EnvModel,
			},
			&pbc.PluginSetting{
				Key:  "timeout",
				Type: pbc.PluginSettingType_PLUGIN_SETTING_TYPE_DURATION,
				// Duration.String renders 60s as 1m0s. Advertise whole seconds.
				DefaultValue: strconv.FormatInt(int64(DefaultTimeout/time.Second), 10) + "s",
				Description:  "Per-attempt request timeout.",
				Env:          EnvTimeout,
			},
			&pbc.PluginSetting{
				Key:          "batch_size",
				Type:         pbc.PluginSettingType_PLUGIN_SETTING_TYPE_INT,
				DefaultValue: strconv.Itoa(scoring.DefaultBatchSize),
				Description:  "Recommendations sent in one backend request.",
				Env:          EnvBatchSize,
			},
			&pbc.PluginSetting{
				Key: "duplicate_threshold",
				// PluginSetting has no float type; the plugin parses this text.
				Type:         pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING,
				DefaultValue: strconv.FormatFloat(scoring.DefaultDuplicateThreshold, 'f', -1, 64),
				Description:  "Yes probability that marks two recommendations as duplicates.",
				Env:          EnvDuplicateThreshold,
			},
		),
	)
}

// ScoreRecommendations rates each recommendation. See the README for what is
// sent to the backend.
func (p *Plugin) ScoreRecommendations(
	ctx context.Context, req *pbc.ScoreRecommendationsRequest,
) (*pbc.ScoreRecommendationsResponse, error) {
	return p.scorer.Score(ctx, req)
}

// Supports declines every pricing request; this plugin prices nothing.
func (p *Plugin) Supports(_ context.Context, _ *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
	return &pbc.SupportsResponse{Supported: false, Reason: "scorer plugin scores recommendations only"}, nil
}
