// Package workload prices Kubernetes workloads declared in a Pulumi plan from
// their pod templates' resource requests and rates the operator configures.
package workload

// Environment variables the plugin reads at startup.
const (
	EnvCPUHourlyRate       = "FINFOCUS_KUBERNETES_CPU_HOURLY_RATE"
	EnvMemoryGiBHourlyRate = "FINFOCUS_KUBERNETES_MEMORY_GIB_HOURLY_RATE"
	EnvDaemonSetNodeCount  = "FINFOCUS_KUBERNETES_DAEMONSET_NODE_COUNT"
	EnvJobHoursPerMonth    = "FINFOCUS_KUBERNETES_JOB_HOURS_PER_MONTH"
)

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
