package kubernetes

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/rshade/finfocus/plugins/kubernetes/workload"
)

// maxJobHoursPerMonth is the longest month, 31 days of 24 hours.
const maxJobHoursPerMonth = 744

// Config is the projected-cost configuration; see workload.Config.
type Config = workload.Config

// LoadConfig reads the projected-cost settings through getenv (os.Getenv in
// production). Rates must be finite and not negative (0 is a free price), the
// node count a whole number of at least 1, and hours finite, above 0, and at
// most 744.
func LoadConfig(getenv func(string) string) Config {
	return Config{
		CPUHourlyRate:       parseRate(getenv, workload.EnvCPUHourlyRate),
		MemoryGiBHourlyRate: parseRate(getenv, workload.EnvMemoryGiBHourlyRate),
		DaemonSetNodeCount:  parseNodeCount(getenv, workload.EnvDaemonSetNodeCount),
		JobHoursPerMonth:    parseHours(getenv, workload.EnvJobHoursPerMonth),
	}
}

func parseRate(getenv func(string) string, name string) workload.Setting[float64] {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return workload.Setting[float64]{}
	}
	value, err := parseFinite(name, raw)
	if err == nil && value < 0 {
		err = fmt.Errorf("%s=%q must not be negative", name, raw)
	}
	return workload.Setting[float64]{Value: value, Set: true, Err: err}
}

func parseNodeCount(getenv func(string) string, name string) workload.Setting[int] {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return workload.Setting[int]{}
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return workload.Setting[int]{Set: true, Err: fmt.Errorf("%s=%q is not a whole number", name, raw)}
	}
	if value < 1 {
		err = fmt.Errorf("%s=%q must be at least 1", name, raw)
	}
	return workload.Setting[int]{Value: value, Set: true, Err: err}
}

func parseHours(getenv func(string) string, name string) workload.Setting[float64] {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return workload.Setting[float64]{}
	}
	value, err := parseFinite(name, raw)
	if err == nil && (value <= 0 || value > maxJobHoursPerMonth) {
		err = fmt.Errorf("%s=%q must be above 0 and at most %d", name, raw, maxJobHoursPerMonth)
	}
	return workload.Setting[float64]{Value: value, Set: true, Err: err}
}

func parseFinite(name, raw string) (float64, error) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not a number", name, raw)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%s=%q must be finite", name, raw)
	}
	return value, nil
}
