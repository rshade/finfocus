package usage

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OwnerIndex resolves pods to their top-level controller.
type OwnerIndex struct {
	rsOwner  map[string]*metav1.OwnerReference
	jobOwner map[string]*metav1.OwnerReference
}

// NewOwnerIndex indexes the controllers of ReplicaSets and Jobs.
func NewOwnerIndex(rs []appsv1.ReplicaSet, jobs []batchv1.Job) *OwnerIndex {
	idx := &OwnerIndex{
		rsOwner:  make(map[string]*metav1.OwnerReference, len(rs)),
		jobOwner: make(map[string]*metav1.OwnerReference, len(jobs)),
	}
	for i := range rs {
		idx.rsOwner[rs[i].Namespace+"/"+rs[i].Name] = metav1.GetControllerOf(&rs[i])
	}
	for i := range jobs {
		idx.jobOwner[jobs[i].Namespace+"/"+jobs[i].Name] = metav1.GetControllerOf(&jobs[i])
	}
	return idx
}

// Resolve returns the kind and name of the pod's top-level controller:
// ReplicaSet→Deployment and Job→CronJob are followed one level; a pod with no
// controller is reported as its own "Pod".
func (idx *OwnerIndex) Resolve(
	pod *corev1.Pod,
) (kind, name string) { //nolint:nonamedreturns // named returns document the two identifiers returned
	ref := metav1.GetControllerOf(pod)
	if ref == nil {
		return "Pod", pod.Name
	}
	key := pod.Namespace + "/" + ref.Name
	var parent *metav1.OwnerReference
	switch ref.Kind {
	case "ReplicaSet":
		parent = idx.rsOwner[key]
	case "Job":
		parent = idx.jobOwner[key]
	}
	if parent != nil {
		return parent.Kind, parent.Name
	}
	return ref.Kind, ref.Name
}
