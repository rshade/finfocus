package kubernetes

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeconfigClusters connects using standard kubeconfig loading (KUBECONFIG,
// ~/.kube/config) or in-cluster config, selecting scope as the context.
func KubeconfigClusters(scope string) (*Cluster, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules,
		&clientcmd.ConfigOverrides{CurrentContext: scope})
	raw, err := loader.RawConfig()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "load kubeconfig (%v): %v", rules.GetLoadingPrecedence(), err)
	}
	contextName := scope
	if contextName == "" {
		contextName = raw.CurrentContext
	}
	if scope != "" {
		if _, ok := raw.Contexts[scope]; !ok {
			return nil, status.Errorf(codes.InvalidArgument,
				"kubeconfig context %q not found (searched %v)", scope, rules.GetLoadingPrecedence())
		}
	}
	cfg, err := loader.ClientConfig()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument,
			"build client config for context %q: not running in-cluster and no usable kubeconfig found"+
				" (set KUBECONFIG or place one at %v): %v",
			contextName, rules.GetLoadingPrecedence(), err)
	}
	client, err := k8s.NewForConfig(cfg)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("create kubernetes client: %v", err))
	}
	return &Cluster{Client: client, Host: cfg.Host, Context: contextName}, nil
}
