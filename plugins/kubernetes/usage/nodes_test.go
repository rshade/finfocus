package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func mkNode(name, providerID string, labels map[string]string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec: corev1.NodeSpec{ProviderID: providerID}}
}

func TestNodeDescriptor(t *testing.T) {
	std := map[string]string{
		"node.kubernetes.io/instance-type": "m5.large",
		"topology.kubernetes.io/region":    "us-east-1",
	}
	d, ok := NodeDescriptor(mkNode("n1", "aws:///us-east-1a/i-0abc", std))
	require.True(t, ok)
	assert.Equal(t, "aws", d.GetProvider())
	assert.Equal(t, "aws:ec2/instance:Instance", d.GetResourceType())
	assert.Equal(t, "m5.large", d.GetSku())
	assert.Equal(t, "us-east-1", d.GetRegion())
	assert.Equal(t, "n1", d.GetId())
	assert.Equal(t, "node", d.GetTags()["kind"])
	assert.Equal(t, "on-demand", d.GetTags()["capacity_type"])
	assert.Equal(t, "aws:///us-east-1a/i-0abc", d.GetTags()["provider_id"])

	spot := map[string]string{"karpenter.sh/capacity-type": "spot"}
	for k, v := range std {
		spot[k] = v
	}
	d, ok = NodeDescriptor(mkNode("n2", "aws:///us-east-1a/i-0def", spot))
	require.True(t, ok)
	assert.Equal(t, "spot", d.GetTags()["capacity_type"])

	kind := map[string]string{"finfocus.dev/provider": "aws"}
	for k, v := range std {
		kind[k] = v
	}
	_, ok = NodeDescriptor(mkNode("kind-worker", "kind://docker/kind/kind-worker", kind))
	assert.True(t, ok, "provider label overrides unknown providerID scheme")

	_, ok = NodeDescriptor(mkNode("n3", "kind://docker/x", std))
	assert.False(t, ok, "unknown provider without label is not priceable")
	_, ok = NodeDescriptor(
		mkNode("n4", "aws:///us-east-1a/i-1", map[string]string{"topology.kubernetes.io/region": "us-east-1"}),
	)
	assert.False(t, ok, "missing instance type is not priceable")
}

func TestControlPlaneDescriptor(t *testing.T) {
	d, ok := ControlPlaneDescriptor("https://ABCDEF.gr7.us-west-2.eks.amazonaws.com", "prod")
	require.True(t, ok)
	assert.Equal(t, "aws:eks/cluster:Cluster", d.GetResourceType())
	assert.Equal(t, "cluster", d.GetSku())
	assert.Equal(t, "us-west-2", d.GetRegion())
	assert.Equal(t, "prod", d.GetId())
	assert.Equal(t, "cluster", d.GetTags()["kind"])

	for host, region := range map[string]string{
		"https://X.gr7.us-east-1.eks.amazonaws.com:443/":     "us-east-1",
		"https://X.gr7.us-gov-west-1.eks.amazonaws.com":      "us-gov-west-1",
		"https://X.gr7.cn-north-1.eks.amazonaws.com.cn":      "cn-north-1",
		"https://X.yl4.cn-northwest-1.eks.amazonaws.com.cn/": "cn-northwest-1",
		"https://X.yl4.cn-north-1.eks.amazonaws.com.cn:443":  "cn-north-1",
	} {
		d, ok := ControlPlaneDescriptor(host, "x")
		require.True(t, ok, host)
		assert.Equal(t, region, d.GetRegion(), host)
	}

	for _, host := range []string{
		"", "https://127.0.0.1:6443", "https://kind-control-plane:6443",
		"https://X.gr7.us-east-1.eks.amazonaws.com.evil.example",
	} {
		_, ok := ControlPlaneDescriptor(host, "x")
		assert.False(t, ok, host)
	}
}
