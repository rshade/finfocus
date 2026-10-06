package collect

import "github.com/prometheus/common/model"

// controllerRef is one controller owner. ReplicaSet and Job are followed one
// level, the same walk as plugins/kubernetes/usage.
type controllerRef struct {
	kind string
	name string
}

type ownerIndex struct {
	pods map[string]controllerRef
	rs   map[string]controllerRef
	jobs map[string]controllerRef
}

func newOwnerIndex(data vectors) ownerIndex {
	// kube-state-metrics names the Job "job_name" because "job" is reserved in Prometheus.
	return ownerIndex{
		pods: indexControllers(data.podOwner, "pod"),
		rs:   indexControllers(data.rsOwner, "replicaset"),
		jobs: indexControllers(data.jobOwner, "job_name"),
	}
}

// resolve returns the top-level controller. ReplicaSet resolves to its
// Deployment and Job resolves to its CronJob when that parent was recorded.
// The bool is false when the pod has no controller owner.
func (idx ownerIndex) resolve(namespace, pod string) (controllerRef, bool) {
	ref, ok := idx.pods[namespace+"/"+pod]
	if !ok {
		return controllerRef{}, false
	}
	switch ref.kind {
	case "ReplicaSet":
		if parent, found := idx.rs[namespace+"/"+ref.name]; found {
			return parent, true
		}
	case "Job":
		if parent, found := idx.jobs[namespace+"/"+ref.name]; found {
			return parent, true
		}
	}
	return ref, true
}

func indexControllers(vec model.Vector, nameLabel string) map[string]controllerRef {
	out := make(map[string]controllerRef)
	for _, sample := range vec {
		if string(sample.Metric["owner_is_controller"]) != "true" {
			continue
		}
		namespace := string(sample.Metric["namespace"])
		name := string(sample.Metric[model.LabelName(nameLabel)])
		kind := string(sample.Metric["owner_kind"])
		owner := string(sample.Metric["owner_name"])
		if namespace == "" || name == "" || kind == "" || owner == "" {
			continue
		}
		out[namespace+"/"+name] = controllerRef{kind: kind, name: owner}
	}
	return out
}
