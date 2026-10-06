package collect

import (
	"strings"

	"github.com/prometheus/common/model"
)

const promLabelPrefix = "label_"

// indexPodLabels maps namespace/pod to the Kubernetes label key recorded by
// kube-state-metrics. The label_ prefix is removed. Characters kube-state-metrics
// replaced stay replaced; selector matching sanitizes the requested key the
// same way.
func indexPodLabels(vec model.Vector) map[string]map[string]string {
	out := make(map[string]map[string]string)
	for _, sample := range vec {
		namespace := string(sample.Metric["namespace"])
		pod := string(sample.Metric["pod"])
		if namespace == "" || pod == "" {
			continue
		}
		labels := make(map[string]string)
		for key, value := range sample.Metric {
			name := string(key)
			rest, ok := strings.CutPrefix(name, promLabelPrefix)
			if !ok || rest == "" {
				continue
			}
			labels[rest] = string(value)
		}
		out[namespace+"/"+pod] = labels
	}
	return out
}

func indexPodNodes(vec model.Vector) map[string]string {
	out := make(map[string]string)
	for _, sample := range vec {
		namespace := string(sample.Metric["namespace"])
		pod := string(sample.Metric["pod"])
		node := string(sample.Metric["node"])
		if namespace == "" || pod == "" || node == "" {
			continue
		}
		out[namespace+"/"+pod] = node
	}
	return out
}

func labelsMatch(got, want map[string]string) bool {
	for key, value := range want {
		if got[sanitizeLabel(key)] != value {
			return false
		}
	}
	return true
}

// sanitizeLabel matches the kube-state-metrics label sanitizer: every
// character outside [A-Za-z0-9] becomes '_'.
func sanitizeLabel(key string) string {
	var b strings.Builder
	b.Grow(len(key))
	for _, r := range key {
		if isLabelRune(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}

func isLabelRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
