package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsClusterResource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		typeToken string
		want      bool
	}{
		{"eks aws", "aws:eks/cluster:Cluster", true},
		{"eks component", "eks:index:Cluster", true},
		{"gke classic", "gcp:container/cluster:Cluster", true},
		{"gke native", "google-native:container/v1:Cluster", true},
		{"aks classic", "azure:containerservice/kubernetesCluster:KubernetesCluster", true},
		{"aks native", "azure-native:containerservice:ManagedCluster", true},
		{"ec2 instance", "aws:ec2/instance:Instance", false},
		{"kubernetes deployment", "kubernetes:apps/v1:Deployment", false},
		{"kubeconfig", "kubernetes:core/v1:ConfigMap", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsClusterResource(tt.typeToken))
		})
	}
}

func TestIsWorkloadResource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		typeToken string
		want      bool
	}{
		{"deployment", "kubernetes:apps/v1:Deployment", true},
		{"statefulset", "kubernetes:apps/v1:StatefulSet", true},
		{"daemonset", "kubernetes:apps/v1:DaemonSet", true},
		{"job", "kubernetes:batch/v1:Job", true},
		{"cronjob", "kubernetes:batch/v1:CronJob", true},
		{"configmap", "kubernetes:core/v1:ConfigMap", false},
		{"service", "kubernetes:core/v1:Service", false},
		{"deployment patch", "kubernetes:apps/v1:DeploymentPatch", false},
		{"eks cluster", "aws:eks/cluster:Cluster", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsWorkloadResource(tt.typeToken))
		})
	}
}

func clusterRow(urn string) OverviewRow {
	return OverviewRow{
		URN:    urn,
		Type:   "aws:eks/cluster:Cluster",
		Status: StatusActive,
		Properties: map[string]any{
			"name": "prod-cluster",
			"arn":  "arn:aws:eks:us-east-1:123456789012:cluster/prod-cluster",
		},
	}
}

func workloadRow(urn string) OverviewRow {
	return OverviewRow{
		URN:    urn,
		Type:   "kubernetes:apps/v1:Deployment",
		Status: StatusActive,
		ProjectedCost: &ProjectedCostData{
			MonthlyCost: 54.75,
			Currency:    "USD",
		},
	}
}

func TestExpandClustersProjected(t *testing.T) {
	t.Parallel()
	const clusterURN = "urn:pulumi:prod::myapp::aws:eks/cluster:Cluster::cluster"
	const apiURN = "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::api"
	const workerURN = "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::worker"

	t.Run("single cluster groups workloads", func(t *testing.T) {
		t.Parallel()
		rows := []OverviewRow{
			clusterRow(clusterURN),
			workloadRow(apiURN),
			{URN: "urn:pulumi:prod::myapp::kubernetes:core/v1:ConfigMap::cfg",
				Type: "kubernetes:core/v1:ConfigMap", Status: StatusActive},
			workloadRow(workerURN),
		}
		out := ExpandClustersProjected(rows)

		require.Len(t, out, 4)
		// Parent first, children immediately after, then the ConfigMap.
		assert.Equal(t, clusterURN, out[0].URN)
		assert.Equal(t, apiURN, out[1].URN)
		assert.Equal(t, workerURN, out[2].URN)
		assert.Equal(t, "urn:pulumi:prod::myapp::kubernetes:core/v1:ConfigMap::cfg", out[3].URN)

		assert.Equal(t, []string{apiURN, workerURN}, out[0].ChildURNs)
		for _, child := range out[1:3] {
			assert.Equal(t, clusterURN, child.ParentURN)
			assert.Equal(t, ExpansionSourceProjected, child.ExpansionSource)
			assert.NotNil(t, child.ProjectedCost, "child keeps its enriched cost")
		}
		assert.Empty(t, out[3].ParentURN)
	})

	t.Run("no clusters leaves rows untouched", func(t *testing.T) {
		t.Parallel()
		rows := []OverviewRow{workloadRow(apiURN), workloadRow(workerURN)}
		out := ExpandClustersProjected(rows)
		require.Len(t, out, 2)
		assert.Empty(t, out[0].ParentURN)
		assert.Empty(t, out[1].ParentURN)
	})

	t.Run("multiple clusters leave workloads flat", func(t *testing.T) {
		t.Parallel()
		rows := []OverviewRow{
			clusterRow(clusterURN),
			clusterRow("urn:pulumi:prod::myapp::gcp:container/cluster:Cluster::gke"),
			workloadRow(apiURN),
		}
		out := ExpandClustersProjected(rows)
		require.Len(t, out, 3)
		for _, r := range out {
			assert.Empty(t, r.ParentURN)
			assert.Empty(t, r.ChildURNs)
		}
	})

	t.Run("cluster without workloads gets no children", func(t *testing.T) {
		t.Parallel()
		rows := []OverviewRow{clusterRow(clusterURN)}
		out := ExpandClustersProjected(rows)
		require.Len(t, out, 1)
		assert.Empty(t, out[0].ChildURNs)
	})

	t.Run("input rows are not modified", func(t *testing.T) {
		t.Parallel()
		rows := []OverviewRow{workloadRow(apiURN), clusterRow(clusterURN)}
		out := ExpandClustersProjected(rows)
		require.Len(t, out, 2)
		assert.Equal(t, clusterURN, out[0].URN)
		assert.Equal(t, apiURN, rows[0].URN)
		assert.Empty(t, rows[0].ParentURN)
		assert.Empty(t, rows[0].ExpansionSource)
		assert.Empty(t, rows[1].ChildURNs)
	})

	t.Run("deleting or errored cluster is not expanded", func(t *testing.T) {
		t.Parallel()
		deleting := clusterRow(clusterURN)
		deleting.Status = StatusDeleting
		errored := clusterRow(clusterURN)
		errored.Error = &OverviewRowError{URN: clusterURN, ErrorType: ErrorTypeNetwork, Message: "boom"}
		for _, cluster := range []OverviewRow{deleting, errored} {
			out := ExpandClustersProjected([]OverviewRow{cluster, workloadRow(apiURN)})
			require.Len(t, out, 2)
			assert.Empty(t, out[0].ChildURNs)
			assert.Empty(t, out[1].ParentURN)
		}
	})

	t.Run("idempotent on already-grouped rows", func(t *testing.T) {
		t.Parallel()
		rows := []OverviewRow{clusterRow(clusterURN), workloadRow(apiURN)}
		once := ExpandClustersProjected(rows)
		twice := ExpandClustersProjected(once)
		require.Len(t, twice, 2)
		assert.Equal(t, []string{apiURN}, twice[0].ChildURNs)
		assert.Equal(t, clusterURN, twice[1].ParentURN)
	})
}

func liveResult() *ClusterResult {
	return &ClusterResult{
		Mode:     ModeRunRate,
		Currency: "USD",
		Rows: []ClusterRow{
			{Subject: map[string]string{"kind": "workload", "namespace": "payments"},
				CPUCost: 80.10, MemCost: 40.40, TotalCost: 120.50},
			{Subject: map[string]string{"kind": "workload", "namespace": "default"},
				CPUCost: 10, MemCost: 20, TotalCost: 30},
			{Subject: map[string]string{"kind": "__idle__"}, CPUCost: 5, MemCost: 5, TotalCost: 10},
			{
				Subject:   map[string]string{"kind": "__cluster__"},
				CPUCost:   0,
				MemCost:   0,
				TotalCost: 73,
			},
		},
		Total: 233.50,
		Idle:  10,
	}
}

func TestLiveChildrenFromResult(t *testing.T) {
	t.Parallel()
	const clusterURN = "urn:pulumi:prod::myapp::aws:eks/cluster:Cluster::cluster"

	children, err := LiveChildrenFromResult(clusterURN, liveResult())
	require.NoError(t, err)
	// Groups sorted by total cost desc: payments, default, __idle__; __cluster__ skipped.
	require.Len(t, children, 3)

	assert.Equal(t, clusterURN+"#ns/payments", children[0].URN)
	assert.Equal(t, "finfocus:k8s/namespace:Allocation", children[0].Type)
	assert.Equal(t, StatusActive, children[0].Status)
	assert.Equal(t, clusterURN, children[0].ParentURN)
	assert.Equal(t, ExpansionSourceLive, children[0].ExpansionSource)
	require.NotNil(t, children[0].ProjectedCost)
	assert.InDelta(t, 120.50, children[0].ProjectedCost.MonthlyCost, 1e-9)
	assert.Equal(t, "USD", children[0].ProjectedCost.Currency)
	assert.InDelta(t, 80.10, children[0].ProjectedCost.Breakdown["cpu"], 1e-9)
	assert.InDelta(t, 40.40, children[0].ProjectedCost.Breakdown["memory"], 1e-9)

	assert.Equal(t, clusterURN+"#ns/default", children[1].URN)
	assert.Equal(t, clusterURN+"#ns/__idle__", children[2].URN)
}

func TestApplyLiveExpansion(t *testing.T) {
	t.Parallel()
	const clusterURN = "urn:pulumi:prod::myapp::aws:eks/cluster:Cluster::cluster"
	const apiURN = "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::api"
	const workerURN = "urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::worker"

	t.Run("live children replace projected children", func(t *testing.T) {
		t.Parallel()
		rows := ExpandClustersProjected([]OverviewRow{
			clusterRow(clusterURN), workloadRow(apiURN), workloadRow(workerURN),
		})
		children, err := LiveChildrenFromResult(clusterURN, liveResult())
		require.NoError(t, err)

		out, suppressed := ApplyLiveExpansion(rows, clusterURN, children)
		assert.Equal(t, 2, suppressed)
		// cluster + 3 live children (payments, default, idle).
		require.Len(t, out, 4)
		assert.Equal(t, clusterURN, out[0].URN)
		wantChildURNs := []string{
			clusterURN + "#ns/payments", clusterURN + "#ns/default", clusterURN + "#ns/__idle__",
		}
		assert.Equal(t, wantChildURNs, out[0].ChildURNs)
		for i, want := range wantChildURNs {
			assert.Equal(t, want, out[i+1].URN)
			assert.Equal(t, ExpansionSourceLive, out[i+1].ExpansionSource)
		}
		// Projected workload rows are gone from the flat list.
		for _, r := range out {
			assert.NotEqual(t, apiURN, r.URN)
			assert.NotEqual(t, workerURN, r.URN)
		}
	})

	t.Run("no children is a no-op", func(t *testing.T) {
		t.Parallel()
		rows := []OverviewRow{clusterRow(clusterURN), workloadRow(apiURN)}
		out, suppressed := ApplyLiveExpansion(rows, clusterURN, nil)
		assert.Equal(t, 0, suppressed)
		assert.Len(t, out, 2)
	})

	t.Run("unknown cluster is a no-op", func(t *testing.T) {
		t.Parallel()
		rows := []OverviewRow{clusterRow(clusterURN)}
		children, err := LiveChildrenFromResult(clusterURN, liveResult())
		require.NoError(t, err)
		out, suppressed := ApplyLiveExpansion(
			rows,
			"urn:pulumi:other::x::aws:eks/cluster:Cluster::nope",
			children,
		)
		assert.Equal(t, 0, suppressed)
		assert.Len(t, out, 1)
		assert.Empty(t, out[0].ChildURNs)
	})

	t.Run("second live pass replaces the first", func(t *testing.T) {
		t.Parallel()
		rows := []OverviewRow{clusterRow(clusterURN)}
		children, err := LiveChildrenFromResult(clusterURN, liveResult())
		require.NoError(t, err)
		once, _ := ApplyLiveExpansion(rows, clusterURN, children)
		twice, suppressed := ApplyLiveExpansion(once, clusterURN, children)
		assert.Equal(t, 0, suppressed, "live children are not counted as suppressed projected rows")
		require.Len(t, twice, 4)
	})
}

func TestAggregateSkipsLiveRows(t *testing.T) {
	t.Parallel()
	const clusterURN = "urn:pulumi:prod::myapp::aws:eks/cluster:Cluster::cluster"
	rows := ExpandClustersProjected([]OverviewRow{
		clusterRow(
			clusterURN,
		), workloadRow("urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::api"),
	})
	children, err := LiveChildrenFromResult(clusterURN, liveResult())
	require.NoError(t, err)
	out, suppressed := ApplyLiveExpansion(rows, clusterURN, children)
	require.Equal(t, 1, suppressed)

	totals := summarizeOverviewRows(out)
	assert.InDelta(t, 0, totals.TotalProjected, 1e-9,
		"live children are excluded from totals and the projected child was suppressed")

	// Without live expansion, the projected child still contributes.
	totals = summarizeOverviewRows(rows)
	assert.InDelta(t, 54.75, totals.TotalProjected, 1e-9)
}

func TestClusterRowName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		row  OverviewRow
		want string
	}{
		{"name property", clusterRow("urn:x::y::aws:eks/cluster:Cluster::c"), "prod-cluster"},
		{"arn fallback", OverviewRow{
			URN: "urn:x::y::aws:eks/cluster:Cluster::c", Type: "aws:eks/cluster:Cluster",
			Properties: map[string]any{"arn": "arn:aws:eks:us-east-1:123:cluster/from-arn"},
		}, "from-arn"},
		{"urn fallback", OverviewRow{
			URN: "urn:x::y::gcp:container/cluster:Cluster::gke-main", Type: "gcp:container/cluster:Cluster",
		}, "gke-main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ClusterRowName(tt.row))
		})
	}
}

func TestOverviewRowValidateExpansionSource(t *testing.T) {
	t.Parallel()
	row := workloadRow("urn:pulumi:prod::myapp::kubernetes:apps/v1:Deployment::api")
	require.NoError(t, row.Validate())

	row.ExpansionSource = "bogus"
	err := row.Validate()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrOverviewValidation)

	row.ExpansionSource = ExpansionSourceLive
	assert.NoError(t, row.Validate())
}

func TestIsExpandableCluster(t *testing.T) {
	t.Parallel()
	const urn = "urn:pulumi:prod::myapp::aws:eks/cluster:Cluster::cluster"
	active := clusterRow(urn)
	deleting := clusterRow(urn)
	deleting.Status = StatusDeleting
	errored := clusterRow(urn)
	errored.Error = &OverviewRowError{URN: urn, ErrorType: ErrorTypeNetwork, Message: "boom"}
	child := clusterRow(urn)
	child.ParentURN = "urn:pulumi:prod::myapp::other"

	assert.True(t, IsExpandableCluster(active))
	assert.False(t, IsExpandableCluster(deleting))
	assert.False(t, IsExpandableCluster(errored))
	assert.False(t, IsExpandableCluster(child))
	assert.False(t, IsExpandableCluster(workloadRow(urn)))
}

func TestLiveChildName(t *testing.T) {
	t.Parallel()
	const urn = "urn:pulumi:prod::myapp::aws:eks/cluster:Cluster::cluster"
	children, err := LiveChildrenFromResult(urn, liveResult())
	require.NoError(t, err)
	names := make([]string, len(children))
	for i, c := range children {
		names[i] = LiveChildName(c)
	}
	assert.Equal(t, []string{"ns/payments", "ns/default", "(idle)"}, names)
	assert.Empty(t, LiveChildName(workloadRow(urn)))
	assert.Empty(t, LiveChildName(OverviewRow{URN: urn, ExpansionSource: ExpansionSourceLive}))
}

func TestClusterARNName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "prod", ClusterARNName("arn:aws:eks:us-east-1:123456789012:cluster/prod"))
	assert.Equal(t, "prod", ClusterARNName("arn:partition:service:region:account:prod"))
	assert.Empty(t, ClusterARNName("noseparator"))
	assert.Empty(t, ClusterARNName(""))
}
