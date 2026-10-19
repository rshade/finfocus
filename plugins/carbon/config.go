package carbon

import (
	"fmt"
	"math"
	"strconv"
)

const defaultUtilization = 0.5

// EnvUtilization configures the projected CPU utilization fraction.
const EnvUtilization = "FINFOCUS_CARBON_UTILIZATION"

// Config contains the explicit assumptions for projected carbon.
type Config struct{ Utilization float64 }

// ConfigFromEnv reads and validates utilization, defaulting to 50 percent.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	cfg := Config{Utilization: defaultUtilization}
	if raw := getenv(EnvUtilization); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return Config{}, fmt.Errorf("%s must be a finite fraction in [0, 1]: %w", EnvUtilization, err)
		}
		cfg.Utilization = value
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if math.IsNaN(c.Utilization) || math.IsInf(c.Utilization, 0) || c.Utilization < 0 || c.Utilization > 1 {
		return fmt.Errorf("%s must be a finite fraction in [0, 1]", EnvUtilization)
	}
	return nil
}
