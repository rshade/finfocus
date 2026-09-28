package kubernetes

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	testHostContextA = "https://127.0.0.1:6443"
	testHostContextB = "https://X.gr7.us-east-1.eks.amazonaws.com"
)

// writeTestKubeconfig writes a kubeconfig with two contexts (context-a
// current, context-b) pointing at dummy servers. No network is contacted:
// both clusters skip TLS verification and KubeconfigClusters never dials.
func writeTestKubeconfig(t *testing.T) string {
	t.Helper()

	cfg := clientcmdapi.NewConfig()
	cfg.CurrentContext = "context-a"
	cfg.Clusters["cluster-a"] = &clientcmdapi.Cluster{Server: testHostContextA, InsecureSkipTLSVerify: true}
	cfg.Clusters["cluster-b"] = &clientcmdapi.Cluster{Server: testHostContextB, InsecureSkipTLSVerify: true}
	cfg.Contexts["context-a"] = &clientcmdapi.Context{Cluster: "cluster-a", AuthInfo: "user-a"}
	cfg.Contexts["context-b"] = &clientcmdapi.Context{Cluster: "cluster-b", AuthInfo: "user-b"}
	cfg.AuthInfos["user-a"] = &clientcmdapi.AuthInfo{Token: "dummy-token-a"}
	cfg.AuthInfos["user-b"] = &clientcmdapi.AuthInfo{Token: "dummy-token-b"}

	path := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, clientcmd.WriteToFile(*cfg, path))
	return path
}

func TestKubeconfigClusters_EmptyScopeUsesCurrentContext(t *testing.T) {
	t.Setenv("KUBECONFIG", writeTestKubeconfig(t))

	cluster, err := KubeconfigClusters("")
	require.NoError(t, err)
	assert.Equal(t, "context-a", cluster.Context)
	assert.Equal(t, testHostContextA, cluster.Host)
}

func TestKubeconfigClusters_ExplicitScopeSelectsContext(t *testing.T) {
	t.Setenv("KUBECONFIG", writeTestKubeconfig(t))

	cluster, err := KubeconfigClusters("context-b")
	require.NoError(t, err)
	assert.Equal(t, "context-b", cluster.Context)
	assert.Equal(t, testHostContextB, cluster.Host)
}

func TestKubeconfigClusters_UnknownScope(t *testing.T) {
	t.Setenv("KUBECONFIG", writeTestKubeconfig(t))

	_, err := KubeconfigClusters("context-missing")
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "context-missing")
}

func TestKubeconfigClusters_NoKubeconfigNotInCluster(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("HOME", t.TempDir())

	_, err := KubeconfigClusters("")
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "KUBECONFIG")
}
