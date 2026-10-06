package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestPrometheusLabelKeysRoundTrip(t *testing.T) {
	t.Parallel()

	keys := map[string]string{
		"node.kubernetes.io/instance-type":         "label_node_kubernetes_io_instance_type",
		"beta.kubernetes.io/instance-type":         "label_beta_kubernetes_io_instance_type",
		"topology.kubernetes.io/region":            "label_topology_kubernetes_io_region",
		"failure-domain.beta.kubernetes.io/region": "label_failure_domain_beta_kubernetes_io_region",
		"finfocus.dev/provider":                    "label_finfocus_dev_provider",
		"eks.amazonaws.com/capacityType":           "label_eks_amazonaws_com_capacityType",
		"karpenter.sh/capacity-type":               "label_karpenter_sh_capacity_type",
		"eks.amazonaws.com/compute-type":           "label_eks_amazonaws_com_compute_type",
	}
	prom := map[string]string{"unrelated": "ignored"}
	for key, sanitized := range keys {
		assert.Equal(t, sanitized, PrometheusLabelKey(key))
		prom[sanitized] = "v-" + key
	}
	got := KubernetesLabels(prom)
	assert.Len(t, got, len(keys))
	for key := range keys {
		assert.Equal(t, "v-"+key, got[key])
	}
}

func TestDescriptorFromRecordedLabels(t *testing.T) {
	t.Parallel()

	info := map[string]string{"provider_id": "aws:///us-east-1a/i-0abc", "node": "n1"}
	labels := map[string]string{
		PrometheusLabelKey("node.kubernetes.io/instance-type"): "m5.large",
		PrometheusLabelKey("topology.kubernetes.io/region"):    "us-east-1",
	}
	d, ok := Descriptor(JoinNode("n1", "", labels, info))
	require.True(t, ok)
	assert.Equal(t, "aws", d.GetProvider())
	assert.Equal(t, "aws:ec2/instance:Instance", d.GetResourceType())
	assert.Equal(t, "m5.large", d.GetSku())
	assert.Equal(t, "us-east-1", d.GetRegion())
	assert.Equal(t, "n1", d.GetId())
	assert.Equal(t, "node", d.GetTags()["kind"])
	assert.Equal(t, "on-demand", d.GetTags()["capacity_type"])
	assert.Equal(t, "aws:///us-east-1a/i-0abc", d.GetTags()["provider_id"])
	assert.Empty(t, d.GetTags()["cluster"])
	assert.Empty(t, d.GetTags()["node"])

	d, ok = Descriptor(JoinNode("n1", "prod", labels, info))
	require.True(t, ok)
	assert.Equal(t, "n1", d.GetId())
	assert.Equal(t, "prod", d.GetTags()["cluster"])
	assert.Equal(t, "n1", d.GetTags()["node"])
}

func TestDescriptorCapacityAndProvider(t *testing.T) {
	t.Parallel()

	awsInfo := map[string]string{"provider_id": "aws:///us-east-1a/i-0def"}
	base := map[string]string{
		PrometheusLabelKey("node.kubernetes.io/instance-type"): "m5.large",
		PrometheusLabelKey("topology.kubernetes.io/region"):    "us-east-1",
	}

	t.Run("karpenter spot", func(t *testing.T) {
		t.Parallel()
		labels := copyLabels(base)
		labels[PrometheusLabelKey("karpenter.sh/capacity-type")] = "spot"
		d, ok := Descriptor(JoinNode("n2", "", labels, awsInfo))
		require.True(t, ok)
		assert.Equal(t, "spot", d.GetTags()["capacity_type"])
	})

	t.Run("eks spot ignores case", func(t *testing.T) {
		t.Parallel()
		labels := copyLabels(base)
		labels[PrometheusLabelKey("eks.amazonaws.com/capacityType")] = "SPOT"
		d, ok := Descriptor(JoinNode("n2", "", labels, awsInfo))
		require.True(t, ok)
		assert.Equal(t, "spot", d.GetTags()["capacity_type"])
	})

	t.Run("gcp", func(t *testing.T) {
		t.Parallel()
		labels := map[string]string{
			PrometheusLabelKey("node.kubernetes.io/instance-type"): "e2-standard-4",
			PrometheusLabelKey("topology.kubernetes.io/region"):    "us-central1",
		}
		info := map[string]string{"provider_id": "gce://proj/us-central1-a/gke-n1"}
		d, ok := Descriptor(JoinNode("gke-n1", "", labels, info))
		require.True(t, ok)
		assert.Equal(t, "gcp", d.GetProvider())
		assert.Equal(t, "gcp:compute/instance:Instance", d.GetResourceType())
		assert.Equal(t, "e2-standard-4", d.GetSku())
		assert.Equal(t, "us-central1", d.GetRegion())
	})

	t.Run("azure", func(t *testing.T) {
		t.Parallel()
		labels := map[string]string{
			PrometheusLabelKey("node.kubernetes.io/instance-type"): "Standard_D4s_v5",
			PrometheusLabelKey("topology.kubernetes.io/region"):    "eastus",
		}
		info := map[string]string{
			"provider_id": "azure:///subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/aks-n1",
		}
		d, ok := Descriptor(JoinNode("aks-n1", "", labels, info))
		require.True(t, ok)
		assert.Equal(t, "azure", d.GetProvider())
		assert.Equal(t, "azure-native:compute:VirtualMachine", d.GetResourceType())
		assert.Equal(t, "on-demand", d.GetTags()["capacity_type"])
	})

	t.Run("provider label overrides unknown scheme", func(t *testing.T) {
		t.Parallel()
		labels := copyLabels(base)
		labels[PrometheusLabelKey("finfocus.dev/provider")] = "aws"
		info := map[string]string{"provider_id": "kind://docker/kind/kind-worker"}
		_, ok := Descriptor(JoinNode("kind-worker", "", labels, info))
		assert.True(t, ok)
	})

	t.Run("unknown scheme without provider label", func(t *testing.T) {
		t.Parallel()
		_, ok := Descriptor(JoinNode("n3", "", base, map[string]string{"provider_id": "kind://docker/x"}))
		assert.False(t, ok)
	})

	t.Run("missing instance type", func(t *testing.T) {
		t.Parallel()
		labels := map[string]string{PrometheusLabelKey("topology.kubernetes.io/region"): "us-east-1"}
		_, ok := Descriptor(JoinNode("n4", "", labels, awsInfo))
		assert.False(t, ok)
	})

	t.Run("beta aliases", func(t *testing.T) {
		t.Parallel()
		labels := map[string]string{
			PrometheusLabelKey("beta.kubernetes.io/instance-type"):         "m5.large",
			PrometheusLabelKey("failure-domain.beta.kubernetes.io/region"): "us-east-1",
		}
		d, ok := Descriptor(JoinNode("n5", "", labels, awsInfo))
		require.True(t, ok)
		assert.Equal(t, "m5.large", d.GetSku())
		assert.Equal(t, "us-east-1", d.GetRegion())
	})

	t.Run("other provider", func(t *testing.T) {
		t.Parallel()
		labels := copyLabels(base)
		labels[PrometheusLabelKey("finfocus.dev/provider")] = "other"
		d, ok := Descriptor(JoinNode("n6", "", labels, map[string]string{"provider_id": "other://node"}))
		require.True(t, ok)
		assert.Equal(t, "other:compute/instance", d.GetResourceType())
	})
}

func TestDescriptorFargateIsNotPriceable(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		PrometheusLabelKey("eks.amazonaws.com/compute-type"):   "fargate",
		PrometheusLabelKey("node.kubernetes.io/instance-type"): "fargate",
		PrometheusLabelKey("topology.kubernetes.io/region"):    "us-east-1",
	}
	info := map[string]string{"provider_id": "aws:///us-east-1a/i-fg"}
	d, ok := Descriptor(JoinNode("fg", "prod", labels, info))
	assert.False(t, ok)
	assert.Nil(t, d)
	assert.NotEqual(t, "aws:eks/fargate:Pod", resourceTypeOf(d))
}

func TestControlPlaneHosts(t *testing.T) {
	t.Parallel()

	d, ok := ControlPlane("https://ABCDEF.gr7.us-west-2.eks.amazonaws.com", "prod")
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
		desc, found := ControlPlane(host, "x")
		require.True(t, found, host)
		assert.Equal(t, region, desc.GetRegion(), host)
	}

	for _, host := range []string{
		"", "https://127.0.0.1:6443", "https://kind-control-plane:6443",
		"https://X.gr7.us-east-1.eks.amazonaws.com.evil.example",
	} {
		_, found := ControlPlane(host, "x")
		assert.False(t, found, host)
	}
}

func copyLabels(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func resourceTypeOf(d *pbc.ResourceDescriptor) string {
	if d == nil {
		return ""
	}
	return d.GetResourceType()
}
