package identity

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestSelectCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  []string
		scope   string
		want    string
		wantErr bool
	}{
		{name: "no label keeps scope", values: nil, scope: "prod", want: "prod"},
		{name: "no label empty scope", values: []string{""}, scope: "", want: ""},
		{name: "one value empty scope", values: []string{"east"}, scope: "", want: "east"},
		{name: "one value equal scope", values: []string{"east"}, scope: "east", want: "east"},
		{name: "duplicate values are one cluster", values: []string{"east", "east"}, scope: "", want: "east"},
		{name: "one value disagreeing scope", values: []string{"east"}, scope: "west", wantErr: true},
		{name: "several values empty scope", values: []string{"west", "east"}, scope: "", wantErr: true},
		{name: "several values unmatched scope", values: []string{"east", "west"}, scope: "north", wantErr: true},
		{name: "several values matching scope", values: []string{"west", "east"}, scope: "west", want: "west"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := SelectCluster(tt.values, tt.scope)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, codes.InvalidArgument, status.Code(err))
				for _, value := range tt.values {
					if value != "" {
						assert.Contains(t, err.Error(), value)
					}
				}
				if tt.scope != "" {
					assert.Contains(t, err.Error(), tt.scope)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPriceablesRecordedWins(t *testing.T) {
	t.Parallel()

	live := fake.NewSimpleClientset(liveNode("n1", "aws:///us-west-2a/i-new", map[string]string{
		"node.kubernetes.io/instance-type": "m5.xlarge",
		"topology.kubernetes.io/region":    "us-west-2",
	}))
	live.PrependReactor("get", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		assert.Fail(t, "live API must not be read when recorded identity is complete")
		return true, nil, errors.New("live API must not be read when recorded identity is complete")
	})
	recorded := map[string]NodeLabels{
		"n1": {
			Name: "n1", ProviderID: "aws:///us-east-1a/i-old", Cluster: "other",
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": "m5.large",
				"topology.kubernetes.io/region":    "us-east-1",
			},
		},
	}
	priced, warnings := Priceables(context.Background(), []string{"n1"}, recorded, live, "prod",
		"https://ABCDEF.gr7.us-west-2.eks.amazonaws.com", nil)
	node := priceableByID(t, priced, "n1")
	assert.Equal(t, "m5.large", node.GetSku())
	assert.Equal(t, "us-east-1", node.GetRegion())
	assert.Equal(t, "aws:///us-east-1a/i-old", node.GetTags()["provider_id"])
	assert.Equal(t, "prod", node.GetTags()["cluster"])
	cp := priceableByType(t, priced, "aws:eks/cluster:Cluster")
	assert.Equal(t, "us-west-2", cp.GetRegion())
	assert.Empty(t, warnings)
}

func TestPriceablesLiveFallback(t *testing.T) {
	t.Parallel()

	live := fake.NewSimpleClientset(liveNode("n1", "aws:///us-east-1a/i-live", map[string]string{
		"node.kubernetes.io/instance-type": "m5.xlarge",
		"topology.kubernetes.io/region":    "us-east-1",
	}))
	recorded := map[string]NodeLabels{
		"n1": {Name: "n1", Labels: map[string]string{"topology.kubernetes.io/region": "eu-west-1"}},
	}
	priced, warnings := Priceables(context.Background(), []string{"n1"}, recorded, live, "prod", "", nil)
	require.Len(t, priced, 1)
	assert.Equal(t, "m5.xlarge", priced[0].GetSku())
	assert.Equal(t, "us-east-1", priced[0].GetRegion())
	assert.Equal(t, "aws:///us-east-1a/i-live", priced[0].GetTags()["provider_id"])
	assert.Equal(t, "node", priced[0].GetTags()["kind"])
	assert.Empty(t, warningsWithPrefix(warnings, IncompletePrefix))
	assert.Equal(t, []string{controlPlaneNotEKS}, warningsWithoutIncomplete(warnings))
}

func TestPriceablesNeitherSource(t *testing.T) {
	t.Parallel()

	live := fake.NewSimpleClientset()
	priced, warnings := Priceables(context.Background(), []string{"gone"}, nil, live, "prod", "", nil)
	assert.Empty(t, priced)
	assert.Contains(t, warnings, "incomplete: node gone: cannot determine provider, instance type, or region")
}

func TestPriceablesKubeconfigFailure(t *testing.T) {
	t.Parallel()

	live := fake.NewSimpleClientset(liveNode("n1", "aws:///us-east-1a/i-live", map[string]string{
		"node.kubernetes.io/instance-type": "m5.large",
		"topology.kubernetes.io/region":    "us-east-1",
	}))
	priced, warnings := Priceables(context.Background(), []string{"n1"}, nil, live, "prod",
		"https://ABCDEF.gr7.us-east-1.eks.amazonaws.com", errors.New("context missing"))
	assert.Empty(t, priced)
	joined := strings.Join(warnings, "\n")
	assert.Contains(t, joined, "incomplete:")
	assert.Contains(t, joined, "context missing")
	assert.Contains(t, joined, "incomplete: node n1: cannot determine provider, instance type, or region")
	assert.Equal(t, []string{controlPlaneNotConnected}, warningsWithoutIncomplete(warnings))
}

func TestPriceablesFargateLiveNodeIsOmitted(t *testing.T) {
	t.Parallel()

	live := fake.NewSimpleClientset(liveNode("fargate-ip-1", "aws:///us-east-1a/fargate", map[string]string{
		"eks.amazonaws.com/compute-type":   "fargate",
		"node.kubernetes.io/instance-type": "fargate",
		"topology.kubernetes.io/region":    "us-east-1",
	}))
	priced, warnings := Priceables(context.Background(), []string{"fargate-ip-1"}, nil, live, "prod", "", nil)
	assert.Empty(t, priced)
	for _, warning := range warnings {
		assert.NotContains(t, warning, "fargate-ip-1")
	}
	assert.NotContains(t, resourceTypes(priced), "aws:eks/fargate:Pod")
}

func TestPriceablesRecordedFargateBeatsLiveNode(t *testing.T) {
	t.Parallel()

	live := fake.NewSimpleClientset(liveNode("n1", "aws:///us-east-1a/i-live", map[string]string{
		"node.kubernetes.io/instance-type": "m5.large",
		"topology.kubernetes.io/region":    "us-east-1",
	}))
	recorded := map[string]NodeLabels{
		"n1": {Name: "n1", Labels: map[string]string{"eks.amazonaws.com/compute-type": "fargate"}},
	}
	priced, _ := Priceables(context.Background(), []string{"n1"}, recorded, live, "prod", "", nil)
	assert.Empty(t, priced)
}

func TestPriceablesControlPlaneOnlyOnLivePath(t *testing.T) {
	t.Parallel()

	const eksHost = "https://ABCDEF.gr7.us-west-2.eks.amazonaws.com"
	recorded := map[string]NodeLabels{
		"n1": {
			Name: "n1", ProviderID: "aws:///us-east-1a/i-old",
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": "m5.large",
				"topology.kubernetes.io/region":    "us-east-1",
			},
		},
	}

	t.Run("live eks host", func(t *testing.T) {
		t.Parallel()
		live := fake.NewSimpleClientset(liveNode("n1", "aws:///us-west-2a/i-live", map[string]string{
			"node.kubernetes.io/instance-type": "m5.large",
			"topology.kubernetes.io/region":    "us-west-2",
		}))
		priced, warnings := Priceables(context.Background(), []string{"n1"}, nil, live, "prod", eksHost, nil)
		require.Len(t, priced, 2)
		cp := priceableByType(t, priced, "aws:eks/cluster:Cluster")
		assert.Equal(t, "us-west-2", cp.GetRegion())
		assert.Equal(t, "prod", cp.GetId())
		assert.Equal(t, "cluster", cp.GetSku())
		assert.Equal(t, "cluster", cp.GetTags()["kind"])
		assert.Empty(t, warningsWithoutIncomplete(warnings))
	})

	t.Run("live non-eks host", func(t *testing.T) {
		t.Parallel()
		live := fake.NewSimpleClientset()
		priced, warnings := Priceables(context.Background(), nil, nil, live, "prod", "https://127.0.0.1:6443", nil)
		assert.NotContains(t, resourceTypes(priced), "aws:eks/cluster:Cluster")
		assert.Equal(t, []string{controlPlaneNotEKS}, warnings)
		for _, warning := range warnings {
			assert.NotContains(t, warning, IncompletePrefix)
		}
	})

	t.Run("recorded only ignores eks host", func(t *testing.T) {
		t.Parallel()
		priced, warnings := Priceables(context.Background(), []string{"n1"}, recorded, nil, "prod", eksHost, nil)
		require.Len(t, priced, 1)
		assert.Equal(t, "n1", priced[0].GetId())
		assert.NotContains(t, resourceTypes(priced), "aws:eks/cluster:Cluster")
		assert.Equal(t, []string{controlPlaneNotConnected}, warningsWithoutIncomplete(warnings))
	})
}

func liveNode(name, providerID string, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec:       corev1.NodeSpec{ProviderID: providerID},
	}
}

func warningsWithoutIncomplete(warnings []string) []string {
	return warningsWithPrefix(warnings, "")
}

func warningsWithPrefix(warnings []string, prefix string) []string {
	var out []string
	for _, warning := range warnings {
		incomplete := strings.HasPrefix(warning, IncompletePrefix)
		if prefix == "" && !incomplete {
			out = append(out, warning)
		}
		if prefix != "" && incomplete {
			out = append(out, warning)
		}
	}
	return out
}

func resourceTypes(priced []*pbc.ResourceDescriptor) []string {
	out := make([]string, 0, len(priced))
	for _, desc := range priced {
		out = append(out, desc.GetResourceType())
	}
	return out
}

func priceableByType(t *testing.T, priced []*pbc.ResourceDescriptor, resourceType string) *pbc.ResourceDescriptor {
	t.Helper()
	for _, desc := range priced {
		if desc.GetResourceType() == resourceType {
			return desc
		}
	}
	require.FailNowf(t, "priceable not found", "type %s", resourceType)
	return nil
}

func priceableByID(t *testing.T, priced []*pbc.ResourceDescriptor, id string) *pbc.ResourceDescriptor {
	t.Helper()
	for _, desc := range priced {
		if desc.GetId() == id {
			return desc
		}
	}
	require.FailNowf(t, "priceable not found", "id %s", id)
	return nil
}
