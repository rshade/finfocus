package identity

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	controlPlaneNotConnected = "control plane omitted: live API is not connected"
	controlPlaneNotEKS       = "control plane omitted: API host is not an EKS cluster"

	// IncompletePrefix starts every warning that makes a stats response
	// incomplete. Core reads it to set ClusterResult.Incomplete.
	IncompletePrefix = "incomplete:"
)

// SelectCluster chooses the cluster subject from stored cluster label values
// and the request scope.
//
// No cluster label keeps scope, which may be empty. One value matches an empty
// or equal scope and rejects any other scope. Several values require scope to
// name one of them. A rejection is InvalidArgument, and the message lists the
// values the store held.
func SelectCluster(values []string, scope string) (string, error) {
	clusters := uniqueNonEmpty(values)
	switch len(clusters) {
	case 0:
		return scope, nil
	case 1:
		if scope == "" || scope == clusters[0] {
			return clusters[0], nil
		}
		return "", status.Errorf(codes.InvalidArgument,
			"cluster scope %q does not match stored cluster %q", scope, clusters[0])
	default:
		joined := strings.Join(clusters, ", ")
		if slices.Contains(clusters, scope) {
			return scope, nil
		}
		if scope == "" {
			return "", status.Errorf(codes.InvalidArgument,
				"prometheus store has multiple clusters (%s); set scope to one of them", joined)
		}
		return "", status.Errorf(codes.InvalidArgument,
			"cluster scope %q is not one of the stored clusters (%s)", scope, joined)
	}
}

// Priceables resolves priceable identity for node names.
//
// A recorded identity that can build a descriptor wins over the live object.
// A node with no recorded identity is read from live when the client is set,
// and a node priced that way adds a warning naming the fields the live object
// supplied, without the incomplete prefix. Neither source omits the priceable and warns with the incomplete prefix.
// kubeconfigErr warns with that prefix and skips the live client.
//
// An EKS control-plane priceable is emitted only when live is set and apiHost
// matches the regex in plugins/kubernetes/usage/nodes.go. Every other case adds
// one warning that does not use the incomplete prefix. Fargate nodes are not
// allocatable priceables, and no Fargate price is added.
func Priceables(
	ctx context.Context,
	names []string,
	recorded map[string]NodeLabels,
	live kubernetes.Interface,
	cluster, apiHost string,
	kubeconfigErr error,
) ([]*pbc.ResourceDescriptor, []string) {
	var warnings []string
	if kubeconfigErr != nil {
		warnings = append(warnings, fmt.Sprintf("%s kubeconfig: %s", IncompletePrefix, kubeconfigErr.Error()))
		live = nil
	}
	var priced []*pbc.ResourceDescriptor
	for _, name := range uniqueNonEmpty(names) {
		desc, warning := oneNode(ctx, name, recorded, live, cluster)
		if warning != "" {
			warnings = append(warnings, warning)
		}
		if desc != nil {
			priced = append(priced, desc)
		}
	}
	return addControlPlane(priced, warnings, live, cluster, apiHost)
}

func oneNode(
	ctx context.Context,
	name string,
	recorded map[string]NodeLabels,
	live kubernetes.Interface,
	cluster string,
) (*pbc.ResourceDescriptor, string) {
	rec, ok := recorded[name]
	if ok {
		rec.Cluster = cluster
		if desc, done := descriptorOrFargate(rec); done {
			return desc, ""
		}
	}
	if current, found := liveNodeLabels(ctx, live, name, cluster); found {
		if desc, done := descriptorOrFargate(current); done {
			return desc, liveFieldsWarning(desc, rec, current)
		}
	}
	return nil, fmt.Sprintf("%s node %s: cannot determine provider, instance type, or region", IncompletePrefix, name)
}

func liveNodeLabels(ctx context.Context, live kubernetes.Interface, name, cluster string) (NodeLabels, bool) {
	if live == nil {
		return NodeLabels{}, false
	}
	node, err := live.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return NodeLabels{}, false
	}
	return NodeLabelsFromObject(node, cluster), true
}

// liveFieldsWarning names the pricing fields a live node supplied that the
// recorded series did not hold with the same value. The window can predate the
// node's current labels, so the price may not match the node that ran then.
// It does not use the incomplete prefix: the node is priced. A nil desc is a
// Fargate node, which is not priced and gets no warning.
func liveFieldsWarning(desc *pbc.ResourceDescriptor, recorded, current NodeLabels) string {
	if desc == nil {
		return ""
	}
	rec, now := pricingFieldsOf(recorded), pricingFieldsOf(current)
	var fields []string
	for _, field := range []struct{ label, rec, now string }{
		{"provider", rec.provider, now.provider},
		{"instance type", rec.sku, now.sku},
		{"region", rec.region, now.region},
		{"capacity type", rec.capacity, now.capacity},
	} {
		if field.rec != field.now {
			fields = append(fields, field.label)
		}
	}
	if len(fields) == 0 {
		return ""
	}
	return fmt.Sprintf("node %s: %s taken from the live node, not from metrics recorded for the window; "+
		"the window may predate the node's current labels", current.Name, strings.Join(fields, ", "))
}

// descriptorOrFargate reports done when the node is identified. A Fargate node
// is identified and has no allocatable priceable. A descriptor that built is
// identified. Anything else is not done, so the caller tries the next source.
func descriptorOrFargate(n NodeLabels) (*pbc.ResourceDescriptor, bool) {
	if IsFargate(n.Labels) {
		return nil, true
	}
	desc, ok := Descriptor(n)
	if !ok {
		return nil, false
	}
	return desc, true
}

func addControlPlane(
	priced []*pbc.ResourceDescriptor,
	warnings []string,
	live kubernetes.Interface,
	cluster, apiHost string,
) ([]*pbc.ResourceDescriptor, []string) {
	if live == nil {
		return priced, append(warnings, controlPlaneNotConnected)
	}
	desc, ok := ControlPlane(apiHost, cluster)
	if !ok {
		return priced, append(warnings, controlPlaneNotEKS)
	}
	return append(priced, desc), warnings
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	return slices.Sorted(maps.Keys(seen))
}
