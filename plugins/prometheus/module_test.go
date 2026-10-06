package prometheus

import (
	"testing"

	"github.com/prometheus/client_golang/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

// TestModuleRequirements keeps the module graph go mod tidy would otherwise
// drop before the plugin packages import these libraries, and pins the
// finfocus-spec version the historical stats fields come from.
func TestModuleRequirements(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "v0.7.5", pluginsdk.SpecVersion)

	cfg := api.Config{Address: "http://prometheus.example"}
	assert.NotEmpty(t, cfg.Address)

	var pod corev1.Pod
	assert.Empty(t, pod.Name)

	port := intstr.FromInt(80)
	assert.Equal(t, int32(80), port.IntVal)

	var client kubernetes.Interface
	require.Nil(t, client)
}
