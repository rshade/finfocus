package usage

import (
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	labelProvider        = "finfocus.dev/provider"
	labelInstanceType    = "node.kubernetes.io/instance-type"
	labelInstanceTypeOld = "beta.kubernetes.io/instance-type"
	labelRegion          = "topology.kubernetes.io/region"
	labelRegionOld       = "failure-domain.beta.kubernetes.io/region"
	labelEKSCapacity     = "eks.amazonaws.com/capacityType"
	labelKarpenterCap    = "karpenter.sh/capacity-type"
	labelEKSComputeType  = "eks.amazonaws.com/compute-type"

	providerAWS   = "aws"
	providerGCP   = "gcp"
	providerAzure = "azure"

	capacitySpot     = "spot"
	capacityOnDemand = "on-demand"

	// FargateKind is the priceable tag that marks one EKS Fargate pod.
	// aws-public prices resource type FargateResourceType from the cpu and
	// memory_gib tags. The allocator matches those tags back to the pod.
	FargateKind         = "fargate"
	FargateResourceType = "aws:eks/fargate:Pod"
	tagCPU              = "cpu"
	tagMemoryGiB        = "memory_gib"
)

//nolint:gochecknoglobals // Zero-allocation lookup table and compiled regex, both read-only after init.
var (
	providerSchemes = map[string]string{"aws": providerAWS, "gce": providerGCP, "azure": providerAzure}
	eksHostPattern  = regexp.MustCompile(`\.([a-z]{2}(?:-[a-z]+)+-\d)\.eks\.amazonaws\.com(?:\.cn)?(?::\d+)?/?$`)
)

// IsFargate reports whether the node is an EKS Fargate virtual node.
func IsFargate(n *corev1.Node) bool {
	return n.Labels[labelEKSComputeType] == "fargate"
}

// NodeDescriptor maps a node to a priceable descriptor. It returns false when
// the provider, instance type, or region cannot be determined. ResourceType is
// a Pulumi resource type token specific to the provider: aws:ec2/instance:Instance,
// gcp:compute/instance:Instance, or azure-native:compute:VirtualMachine.
func NodeDescriptor(n *corev1.Node) (*pbc.ResourceDescriptor, bool) {
	provider := n.Labels[labelProvider]
	if provider == "" {
		scheme, _, _ := strings.Cut(n.Spec.ProviderID, "://")
		provider = providerSchemes[scheme]
	}
	sku := firstLabel(n, labelInstanceType, labelInstanceTypeOld)
	region := firstLabel(n, labelRegion, labelRegionOld)
	if provider == "" || sku == "" || region == "" {
		return nil, false
	}
	// ResourceType must be a real Pulumi resource type token
	// (provider:module[/submodule]:TypeName) so pricing plugins can match it.
	// Azure uses the azure-native provider token; the legacy azure provider's
	// azure:compute/virtualMachine:VirtualMachine token is a possible follow-up.
	var resourceType string
	switch provider {
	case providerAWS:
		resourceType = "aws:ec2/instance:Instance"
	case providerGCP:
		resourceType = "gcp:compute/instance:Instance"
	case providerAzure:
		resourceType = "azure-native:compute:VirtualMachine"
	default:
		resourceType = provider + ":compute/instance"
	}
	capacity := capacityOnDemand
	if strings.EqualFold(n.Labels[labelEKSCapacity], capacitySpot) ||
		strings.EqualFold(n.Labels[labelKarpenterCap], capacitySpot) {
		capacity = capacitySpot
	}
	return &pbc.ResourceDescriptor{
		Provider: provider, ResourceType: resourceType, Sku: sku, Region: region, Id: n.Name,
		Tags: map[string]string{
			pluginsdk.SubjectKind: pluginsdk.KindNode,
			"provider_id":         n.Spec.ProviderID,
			"capacity_type":       capacity,
		},
	}, true
}

// ControlPlaneDescriptor detects an EKS API server host and returns the
// control plane as a priceable descriptor.
func ControlPlaneDescriptor(host, cluster string) (*pbc.ResourceDescriptor, bool) {
	m := eksHostPattern.FindStringSubmatch(host)
	if m == nil {
		return nil, false
	}
	return &pbc.ResourceDescriptor{
		Provider: providerAWS, ResourceType: "aws:eks/cluster:Cluster", Sku: "cluster", Region: m[1], Id: cluster,
		Tags: map[string]string{pluginsdk.SubjectKind: "cluster"},
	}, true
}

// FargatePodDescriptor is the priceable entry for one pod on an EKS Fargate
// node. region comes from the virtual node's topology label. An empty region
// cannot be priced, so the descriptor is omitted.
func FargatePodDescriptor(
	cluster, namespace, pod, node, region string,
	cpu, mem float64,
) (*pbc.ResourceDescriptor, bool) {
	if region == "" || namespace == "" || pod == "" {
		return nil, false
	}
	return &pbc.ResourceDescriptor{
		Provider:     providerAWS,
		ResourceType: FargateResourceType,
		Sku:          "fargate",
		Region:       region,
		Id:           cluster + "/" + namespace + "/" + pod,
		Tags: map[string]string{
			pluginsdk.SubjectKind:      FargateKind,
			pluginsdk.SubjectCluster:   cluster,
			pluginsdk.SubjectNamespace: namespace,
			pluginsdk.SubjectPod:       pod,
			pluginsdk.SubjectNode:      node,
			tagCPU:                     strconv.FormatFloat(cpu, 'g', -1, 64),
			tagMemoryGiB:               strconv.FormatFloat(mem, 'g', -1, 64),
		},
	}, true
}

func firstLabel(n *corev1.Node, keys ...string) string {
	for _, k := range keys {
		if v := n.Labels[k]; v != "" {
			return v
		}
	}
	return ""
}
