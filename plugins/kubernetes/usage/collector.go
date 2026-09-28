package usage

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const listPageSize = 500

// Options scopes a collection.
type Options struct {
	Cluster       string // cluster subject value (kube context name)
	Namespace     string // "" = all namespaces
	LabelSelector string // pod label selector
	APIServerHost string // used to detect EKS; "" skips control-plane detection
}

// Collect lists nodes, pods, ReplicaSets, and Jobs and returns run-rate usage.
func Collect(ctx context.Context, cs kubernetes.Interface, opts Options) (*pbc.GetStatsResponse, error) {
	nodes, err := listAll(ctx, "nodes", "", func(o metav1.ListOptions) ([]corev1.Node, string, error) {
		l, err := cs.CoreV1().Nodes().List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods, err := listAll(ctx, "pods", opts.Namespace, func(o metav1.ListOptions) ([]corev1.Pod, string, error) {
		l, err := cs.CoreV1().Pods(opts.Namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}, metav1.ListOptions{LabelSelector: opts.LabelSelector})
	if err != nil {
		return nil, err
	}
	rs, err := listAll(
		ctx,
		"replicasets",
		opts.Namespace,
		func(o metav1.ListOptions) ([]appsv1.ReplicaSet, string, error) {
			l, err := cs.AppsV1().ReplicaSets(opts.Namespace).List(ctx, o)
			if err != nil {
				return nil, "", err
			}
			return l.Items, l.Continue, nil
		},
		metav1.ListOptions{},
	)
	if err != nil {
		return nil, err
	}
	jobs, err := listAll(ctx, "jobs", opts.Namespace, func(o metav1.ListOptions) ([]batchv1.Job, string, error) {
		l, err := cs.BatchV1().Jobs(opts.Namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resp := &pbc.GetStatsResponse{Mode: pbc.StatsMode_STATS_MODE_RUN_RATE}
	collectNodes(resp, nodes, opts)
	if d, ok := ControlPlaneDescriptor(opts.APIServerHost, opts.Cluster); ok {
		resp.Priceable = append(resp.Priceable, d)
	}
	collectPods(resp, pods, rs, jobs, opts)
	return resp, nil
}

func collectNodes(resp *pbc.GetStatsResponse, nodes []corev1.Node, opts Options) {
	for i := range nodes {
		n := &nodes[i]
		if IsFargate(n) {
			continue
		}
		subject := map[string]string{
			pluginsdk.SubjectKind:    pluginsdk.KindNode,
			pluginsdk.SubjectNode:    n.Name,
			pluginsdk.SubjectCluster: opts.Cluster,
		}
		cpu := quantity(n.Status.Allocatable, corev1.ResourceCPU)
		mem := quantity(n.Status.Allocatable, corev1.ResourceMemory) / bytesPerGiB
		resp.Rows = append(
			resp.Rows,
			&pbc.UsageRow{
				Subject: subject,
				Metric:  pluginsdk.MetricCPUAllocatable,
				Amount:  cpu,
				Unit:    pluginsdk.UnitCore,
			},
			&pbc.UsageRow{
				Subject: subject,
				Metric:  pluginsdk.MetricMemAllocatable,
				Amount:  mem,
				Unit:    pluginsdk.UnitGiB,
			},
		)
		if d, ok := NodeDescriptor(n); ok {
			resp.Priceable = append(resp.Priceable, d)
		} else {
			resp.Warnings = append(resp.Warnings,
				fmt.Sprintf("node %s: cannot determine provider, instance type, or region; not priced", n.Name))
		}
	}
}

func collectPods(
	resp *pbc.GetStatsResponse,
	pods []corev1.Pod,
	rs []appsv1.ReplicaSet,
	jobs []batchv1.Job,
	opts Options,
) {
	owners := NewOwnerIndex(rs, jobs)
	for i := range pods {
		p := &pods[i]
		if p.Spec.NodeName == "" || p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			continue
		}
		kind, name := owners.Resolve(p)
		subject := map[string]string{
			pluginsdk.SubjectKind:           pluginsdk.KindWorkload,
			pluginsdk.SubjectCluster:        opts.Cluster,
			pluginsdk.SubjectNamespace:      p.Namespace,
			pluginsdk.SubjectPod:            p.Name,
			pluginsdk.SubjectNode:           p.Spec.NodeName,
			pluginsdk.SubjectControllerKind: kind,
			pluginsdk.SubjectController:     name,
		}
		for k, v := range p.Labels {
			subject[pluginsdk.SubjectLabelPrefix+k] = v
		}
		cpu, mem := EffectiveRequests(p.Spec)
		resp.Rows = append(resp.Rows,
			&pbc.UsageRow{Subject: subject, Metric: pluginsdk.MetricCPURequest, Amount: cpu, Unit: pluginsdk.UnitCore},
			&pbc.UsageRow{Subject: subject, Metric: pluginsdk.MetricMemRequest, Amount: mem, Unit: pluginsdk.UnitGiB},
		)
	}
}

// listAll pages through a list call and maps API errors to gRPC status codes.
func listAll[T any](
	ctx context.Context,
	resource, namespace string,
	list func(metav1.ListOptions) ([]T, string, error),
	opts metav1.ListOptions,
) ([]T, error) {
	var out []T
	opts.Limit = listPageSize
	for {
		items, cont, err := list(opts)
		if err != nil {
			return nil, apiError(resource, namespace, err)
		}
		out = append(out, items...)
		if cont == "" {
			return out, nil
		}
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		opts.Continue = cont
	}
}

func apiError(resource, namespace string, err error) error {
	scope := "cluster-wide"
	if namespace != "" {
		scope = "in namespace " + namespace
	}
	switch {
	case apierrors.IsForbidden(err):
		return status.Errorf(codes.PermissionDenied,
			"kubernetes RBAC: cannot list %s %s (grant list on %s; see docs for the minimal ClusterRole): %v",
			resource, scope, resource, err)
	case apierrors.IsUnauthorized(err):
		return status.Errorf(codes.Unauthenticated, "kubernetes authentication failed listing %s: %v", resource, err)
	default:
		return status.Errorf(codes.Unavailable, "list %s %s: %v", resource, scope, err)
	}
}
