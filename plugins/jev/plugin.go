// Package jev is a finfocus scorer plugin. It implements
// RecommendationScorerService by asking TypeSafe AI's Jev (System One) model
// to judge recommendations that other plugins produced. Scores rank; they are
// never approval to act.
package jev

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/rshade/finfocus/plugins/jev/internal/jevapi"
	"github.com/rshade/finfocus/plugins/jev/internal/scoring"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// PluginName is the registry and binary name suffix.
const PluginName = "jev"

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
	var backend scoring.Backend
	if cfg.APIKey != "" {
		client, err := jevapi.New(jevapi.Config{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
			Timeout: cfg.Timeout,
		})
		if err != nil {
			return nil, fmt.Errorf("configure jev client: %w", err)
		}
		backend = client
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
				Key:         "api_key",
				Type:        pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING,
				Required:    true,
				Secret:      true,
				Description: "TypeSafe AI API key.",
				Env:         EnvAPIKey,
			},
			&pbc.PluginSetting{
				Key:          "base_url",
				Type:         pbc.PluginSettingType_PLUGIN_SETTING_TYPE_URL,
				DefaultValue: DefaultBaseURL,
				Description:  "TypeSafe AI API origin.",
				Env:          EnvBaseURL,
			},
			&pbc.PluginSetting{
				Key:          "model",
				Type:         pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING,
				DefaultValue: DefaultModel,
				Description:  "Model id sent with scoring requests.",
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
	return &pbc.SupportsResponse{Supported: false, Reason: "jev plugin scores recommendations only"}, nil
}
