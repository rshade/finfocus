// Package workload prices Kubernetes workloads declared in a Pulumi plan from
// their pod templates' resource requests and rates the operator configures.
package workload

import "fmt"

// Environment variables the plugin reads at startup.
const (
	EnvCPUHourlyRate       = "FINFOCUS_KUBERNETES_CPU_HOURLY_RATE"
	EnvMemoryGiBHourlyRate = "FINFOCUS_KUBERNETES_MEMORY_GIB_HOURLY_RATE"
	EnvDaemonSetNodeCount  = "FINFOCUS_KUBERNETES_DAEMONSET_NODE_COUNT"
	EnvJobHoursPerMonth    = "FINFOCUS_KUBERNETES_JOB_HOURS_PER_MONTH"
)

// envPrefix is shared by every variable; a reason that names several writes
// the later names as "..._" plus the rest, as the no-rate reason does.
const envPrefix = "FINFOCUS_KUBERNETES_"

// SettingError explains why a present value was rejected. Its message reads
// `<Name>="<Raw>" <Problem>`; Estimate also renders it without the value, or
// by name alone, when several no longer fit in one decline reason.
type SettingError struct {
	Name    string
	Raw     string
	Problem string
}

func (e *SettingError) Error() string {
	return fmt.Sprintf("%s=%q %s", e.Name, e.Raw, e.Problem)
}

// Setting is one configuration value: Set reports whether the variable was
// present, and Err why a present value was rejected. Value is meaningful only
// when Set is true and Err is nil.
type Setting[T any] struct {
	Value T
	Set   bool
	Err   error
}

// Usable reports whether the setting holds an accepted value.
func (s Setting[T]) Usable() bool {
	return s.Set && s.Err == nil
}

// Config holds projected-cost pricing inputs. An unset or invalid value never
// stops the plugin; pricing that needs it declines with a reason naming it.
type Config struct {
	CPUHourlyRate       Setting[float64]
	MemoryGiBHourlyRate Setting[float64]
	DaemonSetNodeCount  Setting[int]
	JobHoursPerMonth    Setting[float64]
}
