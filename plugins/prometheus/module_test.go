package prometheus

import (
	"fmt"
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
// drop before the plugin packages import these libraries, and requires the
// finfocus-spec release the historical stats fields first shipped in.
func TestModuleRequirements(t *testing.T) {
	t.Parallel()

	var major, minor, patch int
	_, err := fmt.Sscanf(pluginsdk.SpecVersion, "v%d.%d.%d", &major, &minor, &patch)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, major*1_000_000+minor*1_000+patch, 7_007,
		"historical stats fields need finfocus-spec v0.7.7 or newer")

	cfg := api.Config{Address: "http://prometheus.example"}
	assert.NotEmpty(t, cfg.Address)

	var pod corev1.Pod
	assert.Empty(t, pod.Name)

	port := intstr.FromInt(80)
	assert.Equal(t, int32(80), port.IntVal)

	var client kubernetes.Interface
	require.Nil(t, client)
}
