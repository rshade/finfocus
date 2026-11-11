package workload

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	typeDeployment  = "kubernetes:apps/v1:Deployment"
	typeStatefulSet = "kubernetes:apps/v1:StatefulSet"
	typeDaemonSet   = "kubernetes:apps/v1:DaemonSet"
	typeJob         = "kubernetes:batch/v1:Job"
	typeCronJob     = "kubernetes:batch/v1:CronJob"
)

func rated(cpu, mem float64) Config {
	return Config{
		CPUHourlyRate:       Setting[float64]{Value: cpu, Set: true},
		MemoryGiBHourlyRate: Setting[float64]{Value: mem, Set: true},
	}
}

// standard is the worked example's rates: 0.04 per vCPU-hour, 0.005 per GiB-hour.
func standard() Config {
	return rated(0.04, 0.005)
}

func podTemplate(cpu, mem any) map[string]any {
	return map[string]any{"spec": map[string]any{
		"containers": containers(resources(cpuMem(cpu, mem), nil)),
	}}
}

func descriptor(t *testing.T, resourceType string, spec map[string]any) *pbc.ResourceDescriptor {
	t.Helper()
	attrs, err := structpb.NewStruct(map[string]any{"spec": spec})
	require.NoError(t, err)
	return &pbc.ResourceDescriptor{Provider: "kubernetes", ResourceType: resourceType, Attributes: attrs}
}

func replicated(replicas any) map[string]any {
	spec := map[string]any{"template": podTemplate("500m", "1Gi")}
	if replicas != nil {
		spec["replicas"] = replicas
	}
	return spec
}

func TestEstimate_Priced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		desc      func(t *testing.T) *pbc.ResourceDescriptor
		cfg       Config
		monthly   float64
		podHourly float64
		notes     []string
	}{
		{
			name:      "worked example",
			desc:      func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDeployment, replicated(3.0)) },
			cfg:       standard(),
			monthly:   54.75,
			podHourly: 0.025,
			notes: []string{
				"declared requests", "3 pods", "spec.replicas",
				EnvCPUHourlyRate, EnvMemoryGiBHourlyRate, "not a real node price",
			},
		},
		{
			name:      "replicas absent assumes one pod",
			desc:      func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDeployment, replicated(nil)) },
			cfg:       standard(),
			monthly:   18.25,
			podHourly: 0.025,
			notes:     []string{"1 pod", "default"},
		},
		{
			name:      "scaled to zero is a real $0",
			desc:      func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDeployment, replicated(0.0)) },
			cfg:       standard(),
			monthly:   0,
			podHourly: 0.025,
			notes:     []string{"scaled to zero"},
		},
		{
			name: "two containers are summed per pod",
			desc: func(t *testing.T) *pbc.ResourceDescriptor {
				return descriptor(t, typeDeployment, map[string]any{
					"replicas": 3.0,
					"template": map[string]any{"spec": map[string]any{"containers": containers(
						resources(cpuMem("250m", "512Mi"), nil),
						resources(cpuMem("250m", "512Mi"), nil),
					)}},
				})
			},
			cfg:       standard(),
			monthly:   54.75,
			podHourly: 0.025,
		},
		{
			name:      "statefulset is priced like a deployment",
			desc:      func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeStatefulSet, replicated(3.0)) },
			cfg:       standard(),
			monthly:   54.75,
			podHourly: 0.025,
		},
		{
			name:      "a zero rate is a price",
			desc:      func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDeployment, replicated(3.0)) },
			cfg:       rated(0, 0.005),
			monthly:   10.95,
			podHourly: 0.005,
		},
		{
			name: "an undeclared resource counts as zero and says so",
			desc: func(t *testing.T) *pbc.ResourceDescriptor {
				return descriptor(t, typeDeployment, map[string]any{
					"replicas": 2.0, "template": podTemplate("500m", nil),
				})
			},
			cfg:       standard(),
			monthly:   29.2,
			podHourly: 0.02,
			notes:     []string{"memory not declared"},
		},
		{
			name: "limits standing in for requests are noted",
			desc: func(t *testing.T) *pbc.ResourceDescriptor {
				return descriptor(t, typeDeployment, map[string]any{
					"template": map[string]any{"spec": map[string]any{
						"containers": containers(resources(nil, cpuMem("500m", "1Gi"))),
					}},
				})
			},
			cfg:       standard(),
			monthly:   18.25,
			podHourly: 0.025,
			notes:     []string{"limits"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Estimate(tt.desc(t), tt.cfg)
			require.True(t, got.Priced(), got.Reason)
			assert.InDelta(t, tt.monthly, got.Monthly, 1e-9)
			assert.InDelta(t, tt.podHourly, got.PodHourly, 1e-12)
			assert.InDelta(t, HoursPerMonth, got.Hours, 1e-12)
			for _, want := range tt.notes {
				assert.Contains(t, got.Note, want)
			}
		})
	}
}

func TestEstimate_Declined(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desc    func(t *testing.T) *pbc.ResourceDescriptor
		cfg     Config
		reasons []string
	}{
		{
			name: "no attributes",
			desc: func(*testing.T) *pbc.ResourceDescriptor {
				return &pbc.ResourceDescriptor{Provider: "kubernetes", ResourceType: typeDeployment}
			},
			reasons: []string{"no inputs", "upgrade finfocus"},
		},
		{
			name: "unknown replicas",
			desc: func(t *testing.T) *pbc.ResourceDescriptor {
				return descriptor(t, typeDeployment, replicated(PulumiUnknown))
			},
			reasons: []string{"spec.replicas", "not known until deployment"},
		},
		{
			name: "unknown request",
			desc: func(t *testing.T) *pbc.ResourceDescriptor {
				return descriptor(t, typeDeployment, map[string]any{"template": podTemplate(PulumiUnknown, "1Gi")})
			},
			reasons: []string{"resources.requests.cpu", "not known until deployment"},
		},
		{
			name: "invalid quantity names its path",
			desc: func(t *testing.T) *pbc.ResourceDescriptor {
				return descriptor(t, typeDeployment, map[string]any{"template": podTemplate("lots", "1Gi")})
			},
			reasons: []string{"spec.template.spec.containers.0.resources.requests.cpu"},
		},
		{
			name: "fractional replicas",
			desc: func(t *testing.T) *pbc.ResourceDescriptor {
				return descriptor(t, typeDeployment, replicated(1.5))
			},
			cfg:     standard(),
			reasons: []string{"spec.replicas", "whole number"},
		},
		{
			name: "no requests declared",
			desc: func(t *testing.T) *pbc.ResourceDescriptor {
				return descriptor(t, typeDeployment, map[string]any{"template": podTemplate(nil, nil)})
			},
			reasons: []string{"no resource requests", "set resource requests"},
		},
		{
			name:    "no rates",
			desc:    func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDeployment, replicated(3.0)) },
			reasons: []string{EnvCPUHourlyRate, "MEMORY_GIB_HOURLY_RATE", "finfocus cost cluster"},
		},
		{
			name: "cpu rate unset",
			desc: func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDeployment, replicated(3.0)) },
			cfg: Config{
				MemoryGiBHourlyRate: Setting[float64]{Value: 0.005, Set: true},
			},
			reasons: []string{EnvCPUHourlyRate, "finfocus cost cluster"},
		},
		{
			name: "memory rate unset",
			desc: func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDeployment, replicated(3.0)) },
			cfg: Config{
				CPUHourlyRate: Setting[float64]{Value: 0.04, Set: true},
			},
			reasons: []string{EnvMemoryGiBHourlyRate, "finfocus cost cluster"},
		},
		{
			name: "invalid rate names the variable and the parse error",
			desc: func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDeployment, replicated(3.0)) },
			cfg: Config{
				CPUHourlyRate: Setting[float64]{
					Set: true, Err: errors.New(EnvCPUHourlyRate + `="cheap" is not a number`),
				},
				MemoryGiBHourlyRate: Setting[float64]{Value: 0.005, Set: true},
			},
			reasons: []string{EnvCPUHourlyRate, "is not a number"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Estimate(tt.desc(t), tt.cfg)
			require.False(t, got.Priced())
			for _, want := range tt.reasons {
				assert.Contains(t, got.Reason, want)
			}
			assert.LessOrEqual(t, utf8.RuneCountInString(got.Reason), MaxReasonLength)
			assert.Zero(t, got.Monthly)
		})
	}
}

func TestEstimate_ReasonIsCapped(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 300)
	got := Estimate(descriptor(t, typeDeployment, map[string]any{"template": podTemplate(long, "1Gi")}), standard())

	require.False(t, got.Priced())
	assert.LessOrEqual(t, utf8.RuneCountInString(got.Reason), MaxReasonLength)
}

func TestEstimate_DeclineReasons(t *testing.T) {
	t.Parallel()

	t.Run("other providers get the provider reason", func(t *testing.T) {
		t.Parallel()
		for _, desc := range []*pbc.ResourceDescriptor{
			{Provider: "aws", ResourceType: "aws:ec2/instance:Instance"},
			{Provider: "aws", ResourceType: typeDeployment},
		} {
			got := Estimate(desc, standard())
			assert.Equal(t, ReasonWrongProvider, got.Reason)
		}
	})

	t.Run("unpriced kubernetes kinds get the kind-list reason", func(t *testing.T) {
		t.Parallel()
		for _, resourceType := range []string{
			"kubernetes:core/v1:ConfigMap",
			"kubernetes:core/v1:Service",
			"kubernetes:apps/v1:ReplicaSet",
			"kubernetes:apps/v1:DeploymentPatch",
		} {
			desc := &pbc.ResourceDescriptor{Provider: "kubernetes", ResourceType: resourceType}
			got := Estimate(desc, standard())
			assert.Equal(t, ReasonUnsupportedKind(), got.Reason, resourceType)
		}
	})

	t.Run("both reasons fit a note", func(t *testing.T) {
		t.Parallel()
		assert.LessOrEqual(t, utf8.RuneCountInString(ReasonWrongProvider), MaxReasonLength)
		assert.LessOrEqual(t, utf8.RuneCountInString(ReasonUnsupportedKind()), MaxReasonLength)
	})

	t.Run("the kind list names every kind kindFor prices", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t,
			"kubernetes plugin prices Deployment, StatefulSet, DaemonSet, Job, and CronJob only",
			ReasonUnsupportedKind())
		for _, entry := range kindTable() {
			_, ok := kindFor(entry.resourceType)
			require.True(t, ok, entry.resourceType)
			name := entry.resourceType[strings.LastIndex(entry.resourceType, ":")+1:]
			assert.Contains(t, ReasonUnsupportedKind(), name)
		}
	})
}

func TestEstimate_Hinted(t *testing.T) {
	t.Parallel()

	withNodes := standard()
	withNodes.DaemonSetNodeCount = Setting[int]{Value: 4, Set: true}
	withHours := standard()
	withHours.JobHoursPerMonth = Setting[float64]{Value: 10, Set: true}
	daemonSet := map[string]any{"template": podTemplate("500m", "1Gi")}
	job := func(parallelism any) map[string]any {
		spec := map[string]any{"template": podTemplate("500m", "1Gi")}
		if parallelism != nil {
			spec["parallelism"] = parallelism
		}
		return spec
	}
	cronJob := map[string]any{"jobTemplate": map[string]any{"spec": job(3.0)}}

	priced := []struct {
		name    string
		desc    func(t *testing.T) *pbc.ResourceDescriptor
		cfg     Config
		monthly float64
		hours   float64
		notes   []string
	}{
		{
			name:    "daemonset uses the node count",
			desc:    func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDaemonSet, daemonSet) },
			cfg:     withNodes,
			monthly: 73,
			hours:   HoursPerMonth,
			notes:   []string{"4 pods", EnvDaemonSetNodeCount},
		},
		{
			name:    "job uses parallelism and configured hours",
			desc:    func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeJob, job(2.0)) },
			cfg:     withHours,
			monthly: 0.5,
			hours:   10,
			notes:   []string{"2 pods", "spec.parallelism", EnvJobHoursPerMonth},
		},
		{
			name:    "job without parallelism runs one pod",
			desc:    func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeJob, job(nil)) },
			cfg:     withHours,
			monthly: 0.25,
			hours:   10,
			notes:   []string{"1 pod"},
		},
		{
			name:    "cronjob reads the job template",
			desc:    func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeCronJob, cronJob) },
			cfg:     withHours,
			monthly: 0.75,
			hours:   10,
			notes:   []string{"3 pods", "spec.jobTemplate.spec.parallelism", EnvJobHoursPerMonth},
		},
	}
	for _, tt := range priced {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Estimate(tt.desc(t), tt.cfg)
			require.True(t, got.Priced(), got.Reason)
			assert.InDelta(t, tt.monthly, got.Monthly, 1e-9)
			assert.InDelta(t, tt.hours, got.Hours, 1e-12)
			for _, want := range tt.notes {
				assert.Contains(t, got.Note, want)
			}
		})
	}

	declined := []struct {
		name   string
		desc   func(t *testing.T) *pbc.ResourceDescriptor
		reason string
	}{
		{
			name:   "daemonset without a node count",
			desc:   func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDaemonSet, daemonSet) },
			reason: EnvDaemonSetNodeCount,
		},
		{
			name:   "job without hours",
			desc:   func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeJob, job(2.0)) },
			reason: EnvJobHoursPerMonth,
		},
		{
			name:   "cronjob without hours",
			desc:   func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeCronJob, cronJob) },
			reason: EnvJobHoursPerMonth,
		},
	}
	for _, tt := range declined {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Estimate(tt.desc(t), standard())
			require.False(t, got.Priced())
			assert.Contains(t, got.Reason, tt.reason)
			assert.LessOrEqual(t, utf8.RuneCountInString(got.Reason), MaxReasonLength)
		})
	}
}

func TestEstimate_InvalidSettings(t *testing.T) {
	t.Parallel()

	badMemory := standard()
	badMemory.MemoryGiBHourlyRate = Setting[float64]{
		Set: true, Err: errors.New(EnvMemoryGiBHourlyRate + " is not a number"),
	}
	badNodes := standard()
	badNodes.DaemonSetNodeCount = Setting[int]{
		Set: true, Err: errors.New(EnvDaemonSetNodeCount + " must be at least 1"),
	}
	badHours := standard()
	badHours.JobHoursPerMonth = Setting[float64]{
		Set: true, Err: errors.New(EnvJobHoursPerMonth + " must be above 0"),
	}
	daemonSet := map[string]any{"template": podTemplate("500m", "1Gi")}

	tests := []struct {
		name   string
		desc   func(t *testing.T) *pbc.ResourceDescriptor
		cfg    Config
		reason string
	}{
		{
			name:   "invalid memory rate",
			desc:   func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDeployment, replicated(3.0)) },
			cfg:    badMemory,
			reason: EnvMemoryGiBHourlyRate + " is not a number",
		},
		{
			name:   "invalid node count",
			desc:   func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDaemonSet, daemonSet) },
			cfg:    badNodes,
			reason: EnvDaemonSetNodeCount + " must be at least 1",
		},
		{
			name:   "invalid job hours",
			desc:   func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeJob, daemonSet) },
			cfg:    badHours,
			reason: EnvJobHoursPerMonth + " must be above 0",
		},
		{
			name: "replicas that are not a number",
			desc: func(t *testing.T) *pbc.ResourceDescriptor {
				return descriptor(t, typeDeployment, replicated(strings.Repeat("three", 10)))
			},
			cfg:    standard(),
			reason: `spec.replicas="threethreethreethreethre…" is not a whole number of pods`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Estimate(tt.desc(t), tt.cfg)
			require.False(t, got.Priced())
			assert.Equal(t, tt.reason, got.Reason)
		})
	}
}

func rejected[T any](name, raw, problem string) Setting[T] {
	return Setting[T]{Set: true, Err: &SettingError{Name: name, Raw: raw, Problem: problem}}
}

// allInvalid is the walkthrough configuration: every variable set, none valid.
func allInvalid() Config {
	return Config{
		CPUHourlyRate:       rejected[float64](EnvCPUHourlyRate, "-1", "must not be negative"),
		MemoryGiBHourlyRate: rejected[float64](EnvMemoryGiBHourlyRate, "abc", "is not a number"),
		DaemonSetNodeCount:  rejected[int](EnvDaemonSetNodeCount, "0", "must be at least 1"),
		JobHoursPerMonth:    rejected[float64](EnvJobHoursPerMonth, "800", "must be above 0 and at most 744"),
	}
}

func TestSettingError_KeepsTheVariableWording(t *testing.T) {
	t.Parallel()

	err := &SettingError{Name: EnvCPUHourlyRate, Raw: "-1", Problem: "must not be negative"}
	assert.Equal(t, `FINFOCUS_KUBERNETES_CPU_HOURLY_RATE="-1" must not be negative`, err.Error())
}

func TestEstimate_ReportsEveryInvalidSetting(t *testing.T) {
	t.Parallel()

	const rates = `FINFOCUS_KUBERNETES_CPU_HOURLY_RATE="-1" must not be negative; ` +
		`..._MEMORY_GIB_HOURLY_RATE="abc" is not a number`
	const withoutValues = "FINFOCUS_KUBERNETES_CPU_HOURLY_RATE must not be negative; " +
		"..._MEMORY_GIB_HOURLY_RATE is not a number; ..._JOB_HOURS_PER_MONTH must be above 0 and at most 744"
	job := map[string]any{"template": podTemplate("500m", "1Gi")}
	cronJob := map[string]any{"jobTemplate": map[string]any{"spec": job}}

	tests := []struct {
		name   string
		desc   func(t *testing.T) *pbc.ResourceDescriptor
		reason string
	}{
		{
			name:   "deployment names both rates and no hint",
			desc:   func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDeployment, replicated(3.0)) },
			reason: rates,
		},
		{
			name:   "statefulset names both rates and no hint",
			desc:   func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeStatefulSet, replicated(3.0)) },
			reason: rates,
		},
		{
			name:   "daemonset adds the node count",
			desc:   func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeDaemonSet, job) },
			reason: rates + `; ..._DAEMONSET_NODE_COUNT="0" must be at least 1`,
		},
		{
			name:   "job adds the hours and drops the values to fit",
			desc:   func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeJob, job) },
			reason: withoutValues,
		},
		{
			name:   "cronjob adds the hours and drops the values to fit",
			desc:   func(t *testing.T) *pbc.ResourceDescriptor { return descriptor(t, typeCronJob, cronJob) },
			reason: withoutValues,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Estimate(tt.desc(t), allInvalid())
			require.False(t, got.Priced())
			assert.Equal(t, tt.reason, got.Reason)
			assert.LessOrEqual(t, utf8.RuneCountInString(got.Reason), MaxReasonLength)
		})
	}
}

func TestEstimate_InvalidSettingsThatDoNotApplyAreNotReported(t *testing.T) {
	t.Parallel()

	cfg := standard()
	cfg.DaemonSetNodeCount = rejected[int](EnvDaemonSetNodeCount, "0", "must be at least 1")
	cfg.JobHoursPerMonth = rejected[float64](EnvJobHoursPerMonth, "800", "must be above 0 and at most 744")

	got := Estimate(descriptor(t, typeDeployment, replicated(3.0)), cfg)
	assert.True(t, got.Priced(), got.Reason)

	cfg.CPUHourlyRate = rejected[float64](EnvCPUHourlyRate, "-1", "must not be negative")
	got = Estimate(descriptor(t, typeDeployment, replicated(3.0)), cfg)
	assert.Equal(t, `FINFOCUS_KUBERNETES_CPU_HOURLY_RATE="-1" must not be negative`, got.Reason)
}

func TestEstimate_InvalidSettingsFallBackToNames(t *testing.T) {
	t.Parallel()

	cfg := Config{
		CPUHourlyRate:       rejected[float64](EnvCPUHourlyRate, "-1", "must not be negative"),
		MemoryGiBHourlyRate: rejected[float64](EnvMemoryGiBHourlyRate, "-1", "must not be negative"),
		JobHoursPerMonth:    rejected[float64](EnvJobHoursPerMonth, "800", "must be above 0 and at most 744"),
	}
	got := Estimate(descriptor(t, typeJob, map[string]any{"template": podTemplate("500m", "1Gi")}), cfg)

	assert.Equal(t, "fix the invalid FINFOCUS_KUBERNETES_CPU_HOURLY_RATE, ..._MEMORY_GIB_HOURLY_RATE, and "+
		"..._JOB_HOURS_PER_MONTH", got.Reason)
}

func TestEstimate_MissingSettingsComeBeforeInvalidOnes(t *testing.T) {
	t.Parallel()

	daemonSet := map[string]any{"template": podTemplate("500m", "1Gi")}

	cfg := allInvalid()
	cfg.MemoryGiBHourlyRate = Setting[float64]{}
	got := Estimate(descriptor(t, typeDaemonSet, daemonSet), cfg)
	assert.Contains(t, got.Reason, "set "+EnvMemoryGiBHourlyRate)

	cfg = allInvalid()
	cfg.DaemonSetNodeCount = Setting[int]{}
	got = Estimate(descriptor(t, typeDaemonSet, daemonSet), cfg)
	assert.Contains(t, got.Reason, "set "+EnvDaemonSetNodeCount)
}

func TestSetting_Usable(t *testing.T) {
	t.Parallel()

	assert.False(t, Setting[int]{}.Usable())
	assert.False(t, Setting[int]{Set: true, Err: errors.New("bad")}.Usable())
	assert.True(t, Setting[int]{Value: 0, Set: true}.Usable())
}
