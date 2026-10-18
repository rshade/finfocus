package scorer

import (
	"fmt"

	"github.com/rshade/finfocus/plugins/scorer/internal/backends/clef"
	"github.com/rshade/finfocus/plugins/scorer/internal/backends/jev"
	"github.com/rshade/finfocus/plugins/scorer/internal/scoring"
)

// newBackend constructs only the selected backend. Missing credentials keep
// the plugin available for discovery while scoring returns UNAUTHENTICATED.
func newBackend(cfg Config) (scoring.Backend, error) {
	if cfg.Backend != backendJev && cfg.Backend != backendClef {
		return nil, fmt.Errorf("%s must be jev or clef", EnvBackend)
	}
	if cfg.Backend == backendClef && cfg.Model != backendClef {
		return nil, fmt.Errorf("%s must be clef for the clef backend", EnvModel)
	}
	if cfg.APIKey == "" {
		return nil, nil //nolint:nilnil // A nil backend intentionally leaves scoring unauthenticated.
	}
	clientCfg := jev.Config{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Timeout: cfg.Timeout}
	if cfg.Backend == backendClef {
		if cfg.ClefAccountID == "" {
			return nil, fmt.Errorf("%s is required for the clef backend", EnvClefAccountID)
		}
		client, err := clef.New(clientCfg, cfg.ClefAccountID)
		if err != nil {
			return nil, fmt.Errorf("configure clef backend (%s): %w", EnvClefAccountID, err)
		}
		return client, nil
	}
	client, err := jev.New(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("configure jev backend: %w", err)
	}
	return client, nil
}
