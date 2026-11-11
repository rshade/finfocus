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
// most 744. A rejected value's Err is a *workload.SettingError.
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
	value, problem := parseFinite(raw)
	if problem == "" && value < 0 {
		problem = "must not be negative"
	}
	return workload.Setting[float64]{Value: value, Set: true, Err: rejected(name, raw, problem)}
}

func parseNodeCount(getenv func(string) string, name string) workload.Setting[int] {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return workload.Setting[int]{}
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return workload.Setting[int]{Set: true, Err: rejected(name, raw, "is not a whole number")}
	}
	problem := ""
	if value < 1 {
		problem = "must be at least 1"
	}
	return workload.Setting[int]{Value: value, Set: true, Err: rejected(name, raw, problem)}
}

func parseHours(getenv func(string) string, name string) workload.Setting[float64] {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return workload.Setting[float64]{}
	}
	value, problem := parseFinite(raw)
	if problem == "" && (value <= 0 || value > maxJobHoursPerMonth) {
		problem = fmt.Sprintf("must be above 0 and at most %d", maxJobHoursPerMonth)
	}
	return workload.Setting[float64]{Value: value, Set: true, Err: rejected(name, raw, problem)}
}

// parseFinite returns the value, or why raw is not a finite number.
func parseFinite(raw string) (float64, string) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, "is not a number"
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, "must be finite"
	}
	return value, ""
}

// rejected is the error for a value with problem, or nil when it has none.
func rejected(name, raw, problem string) error {
	if problem == "" {
		return nil
	}
	return &workload.SettingError{Name: name, Raw: raw, Problem: problem}
}
