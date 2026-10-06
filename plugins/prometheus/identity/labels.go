// Package identity builds priceable descriptors from node labels recorded by
// kube-state-metrics, and falls back to the live API for a node that is still
// present. Descriptor rules are copied from plugins/kubernetes/usage/nodes.go.
// This package does not import that module.
package identity

import (
	"maps"
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

	providerAWS   = "aws"
	providerGCP   = "gcp"
	providerAzure = "azure"

	capacitySpot     = "spot"
	capacityOnDemand = "on-demand"
	computeFargate   = "fargate"

	promLabelPrefix = "label_"
	providerIDLabel = "provider_id"

	// priceableKindCluster is the priced-resource tag. pluginsdk.KindCluster is
	// "__cluster__" and is reserved for allocator output rows.
	priceableKindCluster = "cluster"
)

//nolint:gochecknoglobals // Read-only lookup tables and a compiled regex.
var (
	promLabelKeys   = buildPromLabelKeys()
	providerSchemes = map[string]string{
		"aws": providerAWS, "gce": providerGCP, "azure": providerAzure,
	}
	// eksHostPattern is copied from plugins/kubernetes/usage/nodes.go.
	eksHostPattern = regexp.MustCompile(`\.([a-z]{2}(?:-[a-z]+)+-\d)\.eks\.amazonaws\.com(?:\.cn)?(?::\d+)?/?$`)
)

// NodeLabels is the identity plugins/kubernetes/usage/nodes.go NodeDescriptor
// reads off a node.
type NodeLabels struct {
	Name       string
	ProviderID string
	Labels     map[string]string
	Cluster    string
}

// PrometheusLabelKey is the kube-state-metrics name for a Kubernetes label.
// Every character outside [A-Za-z0-9] becomes '_', then the name is prefixed
// with label_.
func PrometheusLabelKey(key string) string {
	return promLabelPrefix + sanitizeLabel(key)
}

// KubernetesLabels maps a kube_node_labels sample back to Kubernetes label
// keys. Only the keys NodeDescriptor reads are restored. Sanitizing is not
// reversible for any other key, because '.' and '/' and '-' all become '_'.
func KubernetesLabels(prom map[string]string) map[string]string {
	out := make(map[string]string)
	for key, value := range prom {
		k8s, ok := promLabelKeys[key]
		if !ok || value == "" {
			continue
		}
		out[k8s] = value
	}
	return out
}

// JoinNode combines one kube_node_labels sample and one kube_node_info sample.
// provider_id comes from kube_node_info.
func JoinNode(name, cluster string, nodeLabels, nodeInfo map[string]string) NodeLabels {
	providerID := ""
	if nodeInfo != nil {
		providerID = nodeInfo[providerIDLabel]
	}
	return NodeLabels{
		Name: name, Cluster: cluster, ProviderID: providerID, Labels: KubernetesLabels(nodeLabels),
	}
}

// IsFargate reports an EKS Fargate virtual node. Those nodes are not
// allocatable priceables, and this package does not add a Fargate pod price.
func IsFargate(labels map[string]string) bool {
	return labels[labelEKSComputeType] == computeFargate
}

// Descriptor maps recorded node labels to a priceable descriptor. It returns
// false when the node is Fargate or the provider, instance type, or region
// cannot be determined. The rules are copied from
// plugins/kubernetes/usage/nodes.go NodeDescriptor.
func Descriptor(n NodeLabels) (*pbc.ResourceDescriptor, bool) {
	if IsFargate(n.Labels) {
		return nil, false
	}
	provider := n.Labels[labelProvider]
	if provider == "" {
		scheme, _, _ := strings.Cut(n.ProviderID, "://")
		provider = providerSchemes[scheme]
	}
	sku := firstLabel(n.Labels, labelInstanceType, labelInstanceTypeOld)
	region := firstLabel(n.Labels, labelRegion, labelRegionOld)
	if provider == "" || sku == "" || region == "" {
		return nil, false
	}
	resourceType := resourceTypeFor(provider)
	capacity := capacityOnDemand
	if strings.EqualFold(n.Labels[labelEKSCapacity], capacitySpot) ||
		strings.EqualFold(n.Labels[labelKarpenterCap], capacitySpot) {
		capacity = capacitySpot
	}
	tags := map[string]string{
		pluginsdk.SubjectKind: pluginsdk.KindNode,
		providerIDLabel:       n.ProviderID,
		"capacity_type":       capacity,
	}
	if n.Cluster != "" {
		tags[pluginsdk.SubjectCluster] = n.Cluster
		tags[pluginsdk.SubjectNode] = n.Name
	}
	return &pbc.ResourceDescriptor{
		Provider: provider, ResourceType: resourceType, Sku: sku, Region: region, Id: n.Name,
		Tags: tags,
	}, true
}

// ControlPlane detects an EKS API server host and returns the control plane
// as a priceable descriptor. The host regex is copied from
// plugins/kubernetes/usage/nodes.go.
func ControlPlane(host, cluster string) (*pbc.ResourceDescriptor, bool) {
	m := eksHostPattern.FindStringSubmatch(host)
	if m == nil {
		return nil, false
	}
	return &pbc.ResourceDescriptor{
		Provider: providerAWS, ResourceType: "aws:eks/cluster:Cluster", Sku: "cluster", Region: m[1], Id: cluster,
		Tags: map[string]string{pluginsdk.SubjectKind: priceableKindCluster},
	}, true
}

// NodeLabelsFromObject copies a live node into the recorded-label shape.
func NodeLabelsFromObject(n *corev1.Node, cluster string) NodeLabels {
	labels := make(map[string]string, len(n.Labels))
	maps.Copy(labels, n.Labels)
	return NodeLabels{Name: n.Name, ProviderID: n.Spec.ProviderID, Labels: labels, Cluster: cluster}
}

func resourceTypeFor(provider string) string {
	switch provider {
	case providerAWS:
		return "aws:ec2/instance:Instance"
	case providerGCP:
		return "gcp:compute/instance:Instance"
	case providerAzure:
		return "azure-native:compute:VirtualMachine"
	default:
		return provider + ":compute/instance"
	}
}

func firstLabel(labels map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := labels[key]; value != "" {
			return value
		}
	}
	return ""
}

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

func buildPromLabelKeys() map[string]string {
	keys := []string{
		labelInstanceType,
		labelInstanceTypeOld,
		labelRegion,
		labelRegionOld,
		labelProvider,
		labelEKSCapacity,
		labelKarpenterCap,
		labelEKSComputeType,
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[PrometheusLabelKey(key)] = key
	}
	return out
}
