package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ctrl(kind, name string) []metav1.OwnerReference {
	yes := true
	return []metav1.OwnerReference{{Kind: kind, Name: name, Controller: &yes}}
}

func TestOwnerIndex_Resolve(t *testing.T) {
	t.Parallel()

	rs := []appsv1.ReplicaSet{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "api-7d9", OwnerReferences: ctrl("Deployment", "api")}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "bare-rs"}},
	}
	jobs := []batchv1.Job{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "ops",
				Name:            "backup-2891",
				OwnerReferences: ctrl("CronJob", "backup"),
			},
		},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "ops", Name: "one-off"}},
	}
	idx := NewOwnerIndex(rs, jobs)
	pod := func(ns, name string, owners []metav1.OwnerReference) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, OwnerReferences: owners}}
	}
	tests := []struct {
		name           string
		pod            *corev1.Pod
		kind, ctrlName string
	}{
		{"deployment via replicaset", pod("app", "api-7d9-x", ctrl("ReplicaSet", "api-7d9")), "Deployment", "api"},
		{"bare replicaset", pod("app", "b-x", ctrl("ReplicaSet", "bare-rs")), "ReplicaSet", "bare-rs"},
		{"replicaset not listed", pod("app", "c-x", ctrl("ReplicaSet", "gone")), "ReplicaSet", "gone"},
		{"cronjob via job", pod("ops", "backup-2891-x", ctrl("Job", "backup-2891")), "CronJob", "backup"},
		{"bare job", pod("ops", "one-off-x", ctrl("Job", "one-off")), "Job", "one-off"},
		{"statefulset", pod("db", "pg-0", ctrl("StatefulSet", "pg")), "StatefulSet", "pg"},
		{"daemonset", pod("kube-system", "aws-node-x", ctrl("DaemonSet", "aws-node")), "DaemonSet", "aws-node"},
		{"bare pod", pod("dev", "debug", nil), "Pod", "debug"},
		{
			"same rs name other namespace",
			pod("other", "api-7d9-x", ctrl("ReplicaSet", "api-7d9")),
			"ReplicaSet",
			"api-7d9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kind, name := idx.Resolve(tt.pod)
			assert.Equal(t, tt.kind, kind)
			assert.Equal(t, tt.ctrlName, name)
		})
	}
}
