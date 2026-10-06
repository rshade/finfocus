package prometheus

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// openKubeconfig loads the default kubeconfig and selects scope as the context.
// An empty scope keeps the kubeconfig's current context. The returned host is
// the API server address. Callers treat an error as a warning and still query
// Prometheus.
func openKubeconfig(scope string) (kubernetes.Interface, string, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if scope != "" {
		overrides.CurrentContext = scope
	}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	cfg, err := loader.ClientConfig()
	if err != nil {
		return nil, "", err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, "", err
	}
	return client, cfg.Host, nil
}
