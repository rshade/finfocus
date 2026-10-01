package usage

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	plugintesting "github.com/rshade/finfocus-spec/sdk/go/testing"
)

func readyNode(
	name string,
	cpu, mem string, //nolint:unparam // fixture kept generic for future tests with a different node CPU size
	labels map[string]string,
) *corev1.Node {
	n := mkNode(name, "aws:///us-east-1a/i-"+name, labels)
	n.Status.Allocatable = corev1.ResourceList{ //nolint:exhaustive // fixture only sets cpu/memory allocatable
		corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(mem),
	}
	return n
}

func runningPod(ns, name, node string, phase corev1.PodPhase, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, Labels: labels},
		Spec:       corev1.PodSpec{NodeName: node, Containers: []corev1.Container{ctr("500m", "1Gi")}},
		Status:     corev1.PodStatus{Phase: phase},
	}
}

//nolint:gochecknoglobals // shared read-only fixture reused across table-style test cases
var awsLabels = map[string]string{
	"node.kubernetes.io/instance-type": "m5.large",
	"topology.kubernetes.io/region":    "us-east-1",
}

func rowsFor(resp *pbc.GetStatsResponse, key, val string) []*pbc.UsageRow {
	var out []*pbc.UsageRow
	for _, r := range resp.GetRows() {
		if r.GetSubject()[key] == val {
			out = append(out, r)
		}
	}
	return out
}

func TestCollect_RunRate(t *testing.T) {
	t.Parallel()

	fargate := readyNode("fargate-ip-1", "2", "4Gi", map[string]string{"eks.amazonaws.com/compute-type": "fargate"})
	cs := fake.NewSimpleClientset(
		readyNode("n1", "2", "8Gi", awsLabels), fargate,
		runningPod("app", "api-1", "n1", corev1.PodRunning, map[string]string{"team": "payments"}),
		runningPod("app", "done", "n1", corev1.PodSucceeded, nil),
		runningPod("app", "pending", "", corev1.PodPending, nil),
		runningPod("app", "fg", "fargate-ip-1", corev1.PodRunning, nil),
	)
	resp, err := Collect(context.Background(), cs, Options{Cluster: "prod"})
	require.NoError(t, err)
	assert.Equal(t, pbc.StatsMode_STATS_MODE_RUN_RATE, resp.GetMode())

	api := rowsFor(resp, "pod", "api-1")
	require.Len(t, api, 2)
	s := api[0].GetSubject()
	assert.Equal(t, "workload", s["kind"])
	assert.Equal(t, "prod", s["cluster"])
	assert.Equal(t, "Pod", s["controller_kind"])
	assert.Equal(t, "payments", s["label.team"])

	assert.Empty(t, rowsFor(resp, "pod", "done"), "succeeded pods are skipped")
	assert.Empty(t, rowsFor(resp, "pod", "pending"), "unscheduled pods are skipped")
	assert.Len(t, rowsFor(resp, "pod", "fg"), 2, "fargate pods still reported")

	require.Len(t, resp.GetPriceable(), 1, "fargate node is not priceable")
	assert.Equal(t, "n1", resp.GetPriceable()[0].GetId())
	for _, r := range rowsFor(resp, "node", "fargate-ip-1") {
		assert.NotEqual(t, "node", r.GetSubject()["kind"], "no capacity rows for fargate nodes")
	}

	require.NoError(t, plugintesting.ValidateStatsResponse(resp))
}

func TestCollect_NamespaceScope(t *testing.T) {
	t.Parallel()

	cs := fake.NewSimpleClientset(
		readyNode("n1", "2", "8Gi", awsLabels),
		runningPod("a", "p1", "n1", corev1.PodRunning, nil),
		runningPod("b", "p2", "n1", corev1.PodRunning, nil),
	)
	resp, err := Collect(context.Background(), cs, Options{Cluster: "c", Namespace: "a"})
	require.NoError(t, err)
	assert.NotEmpty(t, rowsFor(resp, "pod", "p1"))
	assert.Empty(t, rowsFor(resp, "pod", "p2"))
}

func TestCollect_Forbidden(t *testing.T) {
	t.Parallel()

	for _, resourceName := range []string{"nodes", "pods", "replicasets", "jobs"} {
		t.Run(resourceName, func(t *testing.T) {
			t.Parallel()
			cs := fake.NewSimpleClientset(readyNode("n1", "2", "8Gi", awsLabels))
			cs.PrependReactor("list", resourceName, func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: resourceName}, "", nil)
			})
			_, err := Collect(context.Background(), cs, Options{Cluster: "c"})
			require.Error(t, err)
			assert.Equal(t, codes.PermissionDenied, status.Code(err))
			assert.Contains(t, err.Error(), "list "+resourceName)
		})
	}
}

func TestCollect_ControlPlane(t *testing.T) {
	t.Parallel()

	cs := fake.NewSimpleClientset(readyNode("n1", "2", "8Gi", awsLabels))
	resp, err := Collect(context.Background(), cs, Options{
		Cluster: "prod", APIServerHost: "https://X.gr7.us-east-1.eks.amazonaws.com",
	})
	require.NoError(t, err)
	var kinds []string
	for _, d := range resp.GetPriceable() {
		kinds = append(kinds, d.GetTags()["kind"])
	}
	assert.ElementsMatch(t, []string{"node", "cluster"}, kinds)
}

func TestCollect_Unauthorized(t *testing.T) {
	t.Parallel()

	cs := fake.NewSimpleClientset(readyNode("n1", "2", "8Gi", awsLabels))
	cs.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewUnauthorized("token expired")
	})
	_, err := Collect(context.Background(), cs, Options{Cluster: "c"})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "nodes")
}

func TestCollect_Unavailable(t *testing.T) {
	t.Parallel()

	cs := fake.NewSimpleClientset(readyNode("n1", "2", "8Gi", awsLabels))
	cs.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection refused")
	})
	_, err := Collect(context.Background(), cs, Options{Cluster: "c", Namespace: "team-a"})
	require.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
	assert.Contains(t, err.Error(), "in namespace team-a")
}

func TestCollect_APIErrorCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{
			name: "bad request",
			err:  apierrors.NewBadRequest("invalid label selector"),
			want: codes.InvalidArgument,
		},
		{
			name: "invalid",
			err: apierrors.NewInvalid(
				schema.GroupKind{Kind: "Pod"},
				"pods",
				field.ErrorList{field.Invalid(field.NewPath("metadata", "labels"), "a b", "invalid label key")},
			),
			want: codes.InvalidArgument,
		},
		{
			name: "context canceled in-flight",
			err:  context.Canceled,
			want: codes.Canceled,
		},
		{
			name: "context deadline exceeded in-flight",
			err:  context.DeadlineExceeded,
			want: codes.DeadlineExceeded,
		},
		{
			name: "wrapped context canceled in-flight",
			err:  fmt.Errorf("list pods: %w", context.Canceled),
			want: codes.Canceled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cs := fake.NewSimpleClientset(readyNode("n1", "2", "8Gi", awsLabels))
			cs.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, tt.err
			})
			_, err := Collect(context.Background(), cs, Options{Cluster: "c"})
			require.Error(t, err)
			assert.Equal(t, tt.want, status.Code(err))
		})
	}
}

func TestCollect_NodeNotPriceable(t *testing.T) {
	t.Parallel()

	unknown := mkNode("unknown-provider", "kind://docker/x", nil)
	unknown.Status.Allocatable = corev1.ResourceList{ //nolint:exhaustive // fixture only sets cpu/memory allocatable
		corev1.ResourceCPU: resource.MustParse("2"), corev1.ResourceMemory: resource.MustParse("4Gi"),
	}
	cs := fake.NewSimpleClientset(unknown)
	resp, err := Collect(context.Background(), cs, Options{Cluster: "c"})
	require.NoError(t, err)
	assert.Empty(t, resp.GetPriceable())
	require.Len(t, resp.GetWarnings(), 1)
	assert.Contains(t, resp.GetWarnings()[0], "unknown-provider")
	assert.NotEmpty(t, rowsFor(resp, "node", "unknown-provider"), "capacity rows still reported")
}

func TestListAll_Pagination(t *testing.T) {
	t.Parallel()

	pages := [][]int{{1, 2}, {3}}
	calls := 0
	items, err := listAll(context.Background(), "widgets", "", func(o metav1.ListOptions) ([]int, string, error) {
		page := pages[calls]
		calls++
		cont := ""
		if calls < len(pages) {
			cont = "next"
		}
		assert.Equal(t, int64(listPageSize), o.Limit)
		return page, cont, nil
	}, metav1.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, items)
	assert.Equal(t, 2, calls)
}

func TestListAll_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := listAll(ctx, "widgets", "", func(metav1.ListOptions) ([]int, string, error) {
		calls++
		if calls == 1 {
			cancel()
			return []int{1}, "next", nil
		}
		return nil, "", nil
	}, metav1.ListOptions{})
	require.Error(t, err)
	assert.Equal(t, codes.Canceled, status.Code(err))
}
