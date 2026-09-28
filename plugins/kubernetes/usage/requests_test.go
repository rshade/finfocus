package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func ctr(cpu, mem string) corev1.Container {
	return corev1.Container{
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{ //nolint:exhaustive // fixture only sets cpu/memory requests
				corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(mem),
			},
		},
	}
}

func sidecar(cpu, mem string) corev1.Container {
	c := ctr(cpu, mem)
	always := corev1.ContainerRestartPolicyAlways
	c.RestartPolicy = &always
	return c
}

func TestEffectiveRequests(t *testing.T) {
	tests := []struct {
		name        string
		spec        corev1.PodSpec
		cpu, memGiB float64
	}{
		{
			"sums app containers",
			corev1.PodSpec{Containers: []corev1.Container{ctr("250m", "1Gi"), ctr("250m", "1Gi")}},
			0.5,
			2,
		},
		{"large init dominates", corev1.PodSpec{
			InitContainers: []corev1.Container{ctr("2", "4Gi")},
			Containers:     []corev1.Container{ctr("500m", "1Gi")}}, 2, 4},
		{"sidecar adds to app sum", corev1.PodSpec{
			InitContainers: []corev1.Container{sidecar("100m", "256Mi")},
			Containers:     []corev1.Container{ctr("500m", "1Gi")}}, 0.6, 1.25},
		{"init after sidecar includes sidecar", corev1.PodSpec{
			InitContainers: []corev1.Container{sidecar("500m", "1Gi"), ctr("1", "1Gi")},
			Containers:     []corev1.Container{ctr("100m", "128Mi")}}, 1.5, 2},
		{
			"overhead added",
			corev1.PodSpec{
				Containers: []corev1.Container{ctr("1", "1Gi")},
				Overhead: corev1.ResourceList{ //nolint:exhaustive // fixture only sets cpu/memory overhead
					corev1.ResourceCPU:    resource.MustParse("250m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			1.25,
			1.5,
		},
		{"no requests is zero", corev1.PodSpec{Containers: []corev1.Container{{}}}, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, mem := EffectiveRequests(tt.spec)
			assert.InDelta(t, tt.cpu, cpu, 1e-9)
			assert.InDelta(t, tt.memGiB, mem, 1e-9)
		})
	}
}
