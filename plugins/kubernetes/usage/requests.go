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
// plus the sidecars started before it), plus pod overhead. When the pod sets
// spec.Resources (the PodLevelResources feature, beta and enabled by default
// since Kubernetes v1.34), the pod-level request for a resource replaces the
// container-aggregated value for that resource, matching the scheduler
// semantics of k8s.io/component-helpers/resource.PodRequests.
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
	base := max(app+sidecars, initPeak)
	if pod, ok := podLevelRequest(spec, name); ok {
		base = pod
	}
	return base + quantity(spec.Overhead, name)
}

// podLevelRequest reports the pod-level request for name from spec.Resources,
// and whether one is set. Pod-level resources only support cpu, memory, and
// hugepages; effective() is only called for cpu and memory, so every set
// value is a supported pod-level resource.
func podLevelRequest(spec corev1.PodSpec, name corev1.ResourceName) (float64, bool) {
	if spec.Resources == nil {
		return 0, false
	}
	if _, ok := spec.Resources.Requests[name]; !ok {
		return 0, false
	}
	return quantity(spec.Resources.Requests, name), true
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
