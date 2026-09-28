// Package usage reads a cluster through the Kubernetes API and reports
// per-workload requests and priceable nodes.
package usage

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	bytesPerGiB  = 1 << 30
	milliPerCore = 1000
)

// EffectiveRequests returns a pod's scheduling requests using the Kubernetes
// rule: max(sum of app containers + all sidecars, the largest init container
// plus the sidecars started before it), plus pod overhead.
func EffectiveRequests(
	spec corev1.PodSpec,
) (cpuCores, memGiB float64) { //nolint:nonamedreturns // named returns document the two units returned
	cpu := effective(spec, corev1.ResourceCPU)
	mem := effective(spec, corev1.ResourceMemory)
	return cpu, mem / bytesPerGiB
}

func effective(spec corev1.PodSpec, name corev1.ResourceName) float64 {
	var sidecars, initPeak float64
	for _, c := range spec.InitContainers {
		req := quantity(c.Resources.Requests, name)
		if c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			sidecars += req
			initPeak = max(initPeak, sidecars)
			continue
		}
		initPeak = max(initPeak, req+sidecars)
	}
	var app float64
	for _, c := range spec.Containers {
		app += quantity(c.Resources.Requests, name)
	}
	return max(app+sidecars, initPeak) + quantity(spec.Overhead, name)
}

func quantity(list corev1.ResourceList, name corev1.ResourceName) float64 {
	q, ok := list[name]
	if !ok {
		return 0
	}
	if name == corev1.ResourceCPU {
		return float64(q.MilliValue()) / milliPerCore
	}
	return float64(q.Value())
}
