package usage

import (
	"regexp"
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

	providerAWS = "aws"

	capacitySpot     = "spot"
	capacityOnDemand = "on-demand"
)

//nolint:gochecknoglobals // Zero-allocation lookup table and compiled regex, both read-only after init.
var (
	providerSchemes = map[string]string{"aws": providerAWS, "gce": "gcp", "azure": "azure"}
	eksHostPattern  = regexp.MustCompile(`\.([a-z]{2}(?:-[a-z]+)+-\d)\.eks\.amazonaws\.com(?::\d+)?/?$`)
)

// IsFargate reports whether the node is an EKS Fargate virtual node.
func IsFargate(n *corev1.Node) bool {
	return n.Labels[labelEKSComputeType] == "fargate"
}

// NodeDescriptor maps a node to a priceable descriptor. It returns false when
// the provider, instance type, or region cannot be determined.
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
	resourceType := provider + ":compute/instance"
	if provider == providerAWS {
		resourceType = "aws:ec2/instance:Instance"
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

func firstLabel(n *corev1.Node, keys ...string) string {
	for _, k := range keys {
		if v := n.Labels[k]; v != "" {
			return v
		}
	}
	return ""
}
