package workload

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

const podPath = "spec.template.spec"

func resources(requests, limits map[string]any) map[string]any {
	out := map[string]any{}
	if requests != nil {
		out["requests"] = requests
	}
	if limits != nil {
		out["limits"] = limits
	}
	return map[string]any{"resources": out}
}

func cpuMem(cpu, mem any) map[string]any {
	out := map[string]any{}
	if cpu != nil {
		out["cpu"] = cpu
	}
	if mem != nil {
		out["memory"] = mem
	}
	return out
}

// deploymentAttrs wraps a pod spec at spec.template.spec.
func deploymentAttrs(t *testing.T, podSpec map[string]any) *structpb.Struct {
	t.Helper()
	attrs, err := structpb.NewStruct(map[string]any{
		"spec": map[string]any{"template": map[string]any{"spec": podSpec}},
	})
	require.NoError(t, err)
	return attrs
}

func containers(items ...map[string]any) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

func TestReadPodSpec(t *testing.T) {
	t.Parallel()

	gib := float64(1 << 30)
	tests := []struct {
		name    string
		podSpec map[string]any
		want    PodRequests
	}{
		{
			name:    "one container",
			podSpec: map[string]any{"containers": containers(resources(cpuMem("500m", "1Gi"), nil))},
			want:    PodRequests{CPUCores: 0.5, MemoryGiB: 1, CPUDeclared: true, MemoryDeclared: true},
		},
		{
			name: "two containers are summed",
			podSpec: map[string]any{"containers": containers(
				resources(cpuMem("250m", "512Mi"), nil),
				resources(cpuMem("750m", "1536Mi"), nil),
			)},
			want: PodRequests{CPUCores: 1, MemoryGiB: 2, CPUDeclared: true, MemoryDeclared: true},
		},
		{
			name:    "limits fill absent requests",
			podSpec: map[string]any{"containers": containers(resources(nil, cpuMem("2", "1Gi")))},
			want: PodRequests{
				CPUCores: 2, MemoryGiB: 1, CPUDeclared: true, MemoryDeclared: true, UsedLimits: true,
			},
		},
		{
			name: "a larger init container wins",
			podSpec: map[string]any{
				"containers":     containers(resources(cpuMem("500m", "1Gi"), nil)),
				"initContainers": containers(resources(cpuMem("2", "4Gi"), nil)),
			},
			want: PodRequests{CPUCores: 2, MemoryGiB: 4, CPUDeclared: true, MemoryDeclared: true},
		},
		{
			name: "a sidecar adds to the app sum",
			podSpec: map[string]any{
				"containers": containers(resources(cpuMem("500m", "1Gi"), nil)),
				"initContainers": containers(func() map[string]any {
					sidecar := resources(cpuMem("250m", "256Mi"), nil)
					sidecar["restartPolicy"] = "Always"
					return sidecar
				}()),
			},
			want: PodRequests{CPUCores: 0.75, MemoryGiB: 1.25, CPUDeclared: true, MemoryDeclared: true},
		},
		{
			name: "pod-level requests override containers",
			podSpec: map[string]any{
				"containers": containers(resources(cpuMem("500m", "1Gi"), nil)),
				"resources":  map[string]any{"requests": cpuMem("3", "6Gi")},
			},
			want: PodRequests{CPUCores: 3, MemoryGiB: 6, CPUDeclared: true, MemoryDeclared: true},
		},
		{
			name:    "decimal cpu and decimal memory suffixes",
			podSpec: map[string]any{"containers": containers(resources(cpuMem("0.5", "1G"), nil))},
			want:    PodRequests{CPUCores: 0.5, MemoryGiB: 1e9 / gib, CPUDeclared: true, MemoryDeclared: true},
		},
		{
			name:    "numbers and exponents",
			podSpec: map[string]any{"containers": containers(resources(cpuMem(float64(2), "1e9"), nil))},
			want:    PodRequests{CPUCores: 2, MemoryGiB: 1e9 / gib, CPUDeclared: true, MemoryDeclared: true},
		},
		{
			name:    "numeric memory",
			podSpec: map[string]any{"containers": containers(resources(cpuMem(nil, float64(1e9)), nil))},
			want:    PodRequests{MemoryGiB: 1e9 / gib, MemoryDeclared: true},
		},
		{
			name:    "cpu only",
			podSpec: map[string]any{"containers": containers(resources(cpuMem("500m", nil), nil))},
			want:    PodRequests{CPUCores: 0.5, CPUDeclared: true},
		},
		{
			name:    "memory only",
			podSpec: map[string]any{"containers": containers(resources(cpuMem(nil, "512Mi"), nil))},
			want:    PodRequests{MemoryGiB: 0.5, MemoryDeclared: true},
		},
		{
			name:    "no containers",
			podSpec: map[string]any{"restartPolicy": "Always"},
			want:    PodRequests{},
		},
		{
			name: "an unknown in an unrelated field is ignored",
			podSpec: map[string]any{
				"terminationGracePeriodSeconds": PulumiUnknown,
				"containers":                    containers(resources(cpuMem("500m", "1Gi"), nil)),
			},
			want: PodRequests{CPUCores: 0.5, MemoryGiB: 1, CPUDeclared: true, MemoryDeclared: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ReadPodSpec(deploymentAttrs(t, tt.podSpec), podPath)
			require.NoError(t, err)
			assert.InDelta(t, tt.want.CPUCores, got.CPUCores, 1e-9)
			assert.InDelta(t, tt.want.MemoryGiB, got.MemoryGiB, 1e-9)
			assert.Equal(t, tt.want.CPUDeclared, got.CPUDeclared)
			assert.Equal(t, tt.want.MemoryDeclared, got.MemoryDeclared)
			assert.Equal(t, tt.want.UsedLimits, got.UsedLimits)
		})
	}
}

func TestReadPodSpec_InvalidQuantity(t *testing.T) {
	t.Parallel()

	attrs := deploymentAttrs(t, map[string]any{"containers": containers(
		resources(cpuMem("500m", "1Gi"), nil),
		resources(cpuMem("500m", "lots"), nil),
	)})

	_, err := ReadPodSpec(attrs, podPath)

	var invalid *InvalidQuantityError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "spec.template.spec.containers.1.resources.requests.memory", invalid.Path)
	assert.Contains(t, err.Error(), "spec.template.spec.containers.1.resources.requests.memory")
}

func TestReadPodSpec_Unknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		podSpec map[string]any
		path    string
	}{
		{
			name:    "unknown request",
			podSpec: map[string]any{"containers": containers(resources(cpuMem(PulumiUnknown, "1Gi"), nil))},
			path:    "spec.template.spec.containers.0.resources.requests.cpu",
		},
		{
			name:    "unknown container list",
			podSpec: map[string]any{"containers": PulumiUnknown},
			path:    "spec.template.spec.containers",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ReadPodSpec(deploymentAttrs(t, tt.podSpec), podPath)
			var unknown *UnknownValueError
			require.ErrorAs(t, err, &unknown)
			assert.Equal(t, tt.path, unknown.Path)
		})
	}
}

func TestReadPodSpec_RejectsNonQuantities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		podSpec map[string]any
		path    string
	}{
		{
			name:    "a boolean request",
			podSpec: map[string]any{"containers": containers(resources(cpuMem(true, "1Gi"), nil))},
			path:    "spec.template.spec.containers.0.resources.requests.cpu",
		},
		{
			name:    "a negative limit",
			podSpec: map[string]any{"containers": containers(resources(nil, cpuMem("-1", nil)))},
			path:    "spec.template.spec.containers.0.resources.limits.cpu",
		},
		{
			name: "an invalid pod-level request",
			podSpec: map[string]any{
				"containers": containers(resources(cpuMem("500m", "1Gi"), nil)),
				"resources":  map[string]any{"requests": cpuMem("2", "big")},
			},
			path: "spec.template.spec.resources.requests.memory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ReadPodSpec(deploymentAttrs(t, tt.podSpec), podPath)
			invalid, ok := errors.AsType[*InvalidQuantityError](err)
			require.True(t, ok, "%v", err)
			assert.Equal(t, tt.path, invalid.Path)
		})
	}
}

func TestReadPodSpec_NullRequestIsAbsent(t *testing.T) {
	t.Parallel()

	got, err := ReadPodSpec(deploymentAttrs(t, map[string]any{
		"containers": containers(resources(map[string]any{"cpu": nil, "memory": "1Gi"}, nil)),
	}), podPath)

	require.NoError(t, err)
	assert.False(t, got.CPUDeclared)
	assert.True(t, got.MemoryDeclared)
}
