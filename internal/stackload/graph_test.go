package stackload

import (
	"context"
	"slices"
	"testing"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	urnStack = "urn:pulumi:dev::demo::pulumi:pulumi:Stack::demo-dev"
	urnGroup = "urn:pulumi:dev::demo::demo:index:Group::group"
	urnDB    = "urn:pulumi:dev::demo::aws:rds/instance:Instance::db"
	urnAPI   = "urn:pulumi:dev::demo::aws:ecs/service:Service::api"
	urnLB    = "urn:pulumi:dev::demo::aws:lb/loadBalancer:LoadBalancer::lb"

	urnProvider = "urn:pulumi:dev::demo::pulumi:providers:kubernetes::k8s"
	providerRef = urnProvider + "::5b1e0c8a-provider-id"
)

func TestBuildDependencyGraph(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		resources     []ingest.StackExportResource
		wantAvailable bool
		wantNodes     map[string][]string
		wantComponent []string
	}{
		{
			name: "dependencies and property dependencies merge sorted without duplicates",
			resources: []ingest.StackExportResource{
				{URN: urnDB, Custom: true},
				{URN: urnLB, Custom: true},
				{
					URN:          urnAPI,
					Custom:       true,
					Dependencies: []string{urnLB, urnDB},
					PropertyDependencies: map[string][]string{
						"dbHost":  {urnDB},
						"network": {urnLB, urnGroup},
					},
				},
			},
			wantAvailable: true,
			wantNodes: map[string][]string{
				urnDB:  nil,
				urnLB:  nil,
				urnAPI: {urnLB, urnDB, urnGroup},
			},
		},
		{
			name: "components keep their flag and their own edges",
			resources: []ingest.StackExportResource{
				{URN: urnStack, Type: "pulumi:pulumi:Stack"},
				{URN: urnGroup, Parent: urnStack, Dependencies: []string{urnDB}},
				{URN: urnDB, Custom: true, Parent: urnGroup},
			},
			wantAvailable: true,
			wantNodes: map[string][]string{
				urnStack: nil,
				urnGroup: {urnDB},
				urnDB:    nil,
			},
			wantComponent: []string{urnStack, urnGroup},
		},
		{
			name: "parent alone is not a dependency",
			resources: []ingest.StackExportResource{
				{URN: urnStack},
				{URN: urnDB, Custom: true, Parent: urnStack},
			},
			wantAvailable: false,
			wantNodes:     map[string][]string{urnStack: nil, urnDB: nil},
			wantComponent: []string{urnStack},
		},
		{
			name:          "empty state has no graph",
			resources:     nil,
			wantAvailable: false,
			wantNodes:     map[string][]string{},
		},
		{
			name: "empty urns are dropped",
			resources: []ingest.StackExportResource{
				{URN: "", Custom: true, Dependencies: []string{urnDB}},
				{URN: urnAPI, Custom: true, Dependencies: []string{"", urnDB}},
			},
			wantAvailable: true,
			wantNodes:     map[string][]string{urnAPI: {urnDB}},
		},
		{
			name: "a provider reference is an edge on the provider urn",
			resources: []ingest.StackExportResource{
				{URN: urnProvider, Type: "pulumi:providers:kubernetes", Custom: true},
				{URN: urnAPI, Custom: true, Provider: providerRef},
			},
			wantAvailable: true,
			wantNodes:     map[string][]string{urnProvider: nil, urnAPI: {urnProvider}},
		},
		{
			name: "malformed and empty provider references add nothing",
			resources: []ingest.StackExportResource{
				{URN: urnAPI, Custom: true, Provider: "not-a-reference"},
				{URN: urnDB, Custom: true, Provider: "::id-only"},
				{URN: urnLB, Custom: true},
			},
			wantAvailable: false,
			wantNodes:     map[string][]string{urnAPI: nil, urnDB: nil, urnLB: nil},
		},
		{
			name: "deletedWith and each replaceWith entry are edges",
			resources: []ingest.StackExportResource{
				{URN: urnAPI, Custom: true, DeletedWith: urnGroup, ReplaceWith: []string{urnDB, urnLB}},
			},
			wantAvailable: true,
			wantNodes:     map[string][]string{urnAPI: {urnLB, urnDB, urnGroup}},
		},
		{
			name: "every link kind merges sorted without duplicates",
			resources: []ingest.StackExportResource{
				{
					URN:                  urnAPI,
					Custom:               true,
					Dependencies:         []string{urnDB},
					PropertyDependencies: map[string][]string{"network": {urnLB}},
					Provider:             providerRef,
					DeletedWith:          urnDB,
					ReplaceWith:          []string{urnLB, ""},
				},
			},
			wantAvailable: true,
			wantNodes:     map[string][]string{urnAPI: {urnLB, urnDB, urnProvider}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			graph := BuildDependencyGraph(tt.resources)

			assert.Equal(t, tt.wantAvailable, graph.Available)
			require.Len(t, graph.Nodes, len(tt.wantNodes))
			for urn, want := range tt.wantNodes {
				node, ok := graph.Nodes[urn]
				require.True(t, ok, "missing node %s", urn)
				assert.Equal(t, urn, node.URN)
				assert.Equal(t, want, node.DependsOn)
				assert.Equal(t, engine.StatusActive, node.Status)
			}
			for urn := range tt.wantNodes {
				assert.Equal(t, slices.Contains(tt.wantComponent, urn), graph.Nodes[urn].Component, urn)
			}
		})
	}
}

func TestSnapshotGraphFromLoadedState(t *testing.T) {
	t.Parallel()

	const state = `{"version":3,"deployment":{"resources":[
	  {"urn":"` + urnStack + `","type":"pulumi:pulumi:Stack"},
	  {"urn":"` + urnDB + `","type":"aws:rds/instance:Instance","custom":true,"parent":"` + urnStack + `"},
	  {"urn":"` + urnAPI + `","type":"aws:ecs/service:Service","custom":true,"dependencies":["` + urnDB + `"]}
	]}}`

	snap, err := LoadState(context.Background(), Source{StatePath: inputFile(t, "state.json", state)})
	require.NoError(t, err)

	graph := snap.Graph()
	assert.True(t, graph.Available)
	assert.Equal(t, []string{urnDB}, graph.Nodes[urnAPI].DependsOn)
	assert.True(t, graph.Nodes[urnStack].Component)
}

func TestSnapshotGraphUnavailableWithoutState(t *testing.T) {
	t.Parallel()

	plan, err := LoadPlan(context.Background(), Source{PlanPath: inputFile(t, "plan.json", planJSON)})
	require.NoError(t, err)
	assert.False(t, plan.Graph().Available)

	empty, err := LoadState(context.Background(), Source{
		StatePath: inputFile(t, "empty.json", `{"version":3,"deployment":{"resources":[]}}`),
	})
	require.NoError(t, err)
	assert.False(t, empty.Graph().Available)
	assert.Empty(t, empty.Graph().Nodes)
}

const (
	urnCache = "urn:pulumi:dev::demo::aws:elasticache/cluster:Cluster::cache"
	urnQueue = "urn:pulumi:dev::demo::aws:sqs/queue:Queue::queue"
)

func overlayState(t *testing.T) *Snapshot {
	t.Helper()
	const state = `{"version":3,"deployment":{"resources":[
	  {"urn":"` + urnDB + `","type":"aws:rds/instance:Instance","custom":true},
	  {"urn":"` + urnAPI + `","type":"aws:ecs/service:Service","custom":true,"dependencies":["` + urnDB + `"]},
	  {"urn":"` + urnLB + `","type":"aws:lb/loadBalancer:LoadBalancer","custom":true,"dependencies":["` + urnAPI + `"]}
	]}}`
	snap, err := LoadState(context.Background(), Source{StatePath: inputFile(t, "state.json", state)})
	require.NoError(t, err)
	return snap
}

func stepWithDeps(op, urn string, deps ...string) ingest.PulumiStep {
	step := ingest.PulumiStep{Op: op, URN: urn, Type: "aws:x/y:Y"}
	state := &ingest.PulumiState{URN: urn, Type: "aws:x/y:Y", Dependencies: deps}
	if op == "delete" {
		step.OldState = state
	} else {
		step.NewState = state
	}
	return step
}

func TestWithPlanOverlaysGraph(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		steps []ingest.PulumiStep
		check func(t *testing.T, base, overlaid map[string][]string, deleting []string)
	}{
		{
			name:  "create adds a node with its new edges",
			steps: []ingest.PulumiStep{stepWithDeps("create", urnCache, urnDB)},
			check: func(t *testing.T, _, overlaid map[string][]string, _ []string) {
				assert.Equal(t, []string{urnDB}, overlaid[urnCache])
				assert.Equal(t, []string{urnDB}, overlaid[urnAPI])
			},
		},
		{
			name:  "update replaces the edges of the node",
			steps: []ingest.PulumiStep{stepWithDeps("update", urnLB, urnDB)},
			check: func(t *testing.T, _, overlaid map[string][]string, _ []string) {
				assert.Equal(t, []string{urnDB}, overlaid[urnLB])
			},
		},
		{
			name:  "replace replaces the edges of the node",
			steps: []ingest.PulumiStep{stepWithDeps("replace", urnAPI, urnQueue)},
			check: func(t *testing.T, _, overlaid map[string][]string, _ []string) {
				assert.Equal(t, []string{urnQueue}, overlaid[urnAPI])
			},
		},
		{
			name:  "delete keeps the node and flags it",
			steps: []ingest.PulumiStep{stepWithDeps("delete", urnAPI, urnDB)},
			check: func(t *testing.T, _, overlaid map[string][]string, deleting []string) {
				assert.Equal(t, []string{urnDB}, overlaid[urnAPI])
				assert.Equal(t, []string{urnAPI}, deleting)
			},
		},
		{
			name: "delete of a replaced resource does not flag it",
			steps: []ingest.PulumiStep{
				stepWithDeps("create-replacement", urnAPI, urnDB),
				stepWithDeps("delete-replaced", urnAPI, urnDB),
			},
			check: func(t *testing.T, _, _ map[string][]string, deleting []string) {
				assert.Empty(t, deleting)
			},
		},
		{
			name:  "same adds nothing",
			steps: []ingest.PulumiStep{stepWithDeps("same", urnQueue, urnDB)},
			check: func(t *testing.T, base, overlaid map[string][]string, _ []string) {
				assert.Equal(t, base, overlaid)
			},
		},
		{
			name:  "a resource missing from the preview keeps its state edges",
			steps: []ingest.PulumiStep{stepWithDeps("create", urnQueue)},
			check: func(t *testing.T, _, overlaid map[string][]string, _ []string) {
				assert.Equal(t, []string{urnAPI}, overlaid[urnLB])
				assert.Equal(t, []string{urnDB}, overlaid[urnAPI])
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			snap := overlayState(t)
			baseEdges := graphEdges(snap.Graph())

			overlaid := snap.WithPlan(&ingest.PulumiPlan{Steps: tt.steps}).Graph()

			var deleting []string
			for urn, node := range overlaid.Nodes {
				if node.Status == engine.StatusDeleting {
					deleting = append(deleting, urn)
				}
			}
			tt.check(t, baseEdges, graphEdges(overlaid), deleting)
			assert.Equal(t, baseEdges, graphEdges(snap.Graph()), "the receiver's graph must not change")
		})
	}
}

func TestOverlayPlanGraphDescribesPlanOnlyNodes(t *testing.T) {
	t.Parallel()

	custom, notCustom := true, false
	tests := []struct {
		name          string
		resourceType  string
		custom        *bool
		wantComponent bool
	}{
		{"custom resource", "aws:x/y:Y", &custom, false},
		{"component recorded by the preview", "my:index:Group", &notCustom, true},
		{"no custom field on a provider type", "aws:x/y:Y", nil, false},
		{"no custom field on a pulumi type", "pulumi:pulumi:Stack", nil, true},
		{"no custom field on a provider resource", "pulumi:providers:aws", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			step := ingest.PulumiStep{Op: "create", URN: urnCache, NewState: &ingest.PulumiState{
				URN: urnCache, Type: tt.resourceType, Custom: tt.custom, Dependencies: []string{urnDB},
			}}

			node := OverlayPlanGraph(engine.DependencyGraph{}, &ingest.PulumiPlan{Steps: []ingest.PulumiStep{step}}).Nodes[urnCache]

			assert.Equal(t, tt.resourceType, node.Type)
			assert.Equal(t, engine.StatusCreating, node.Status)
			assert.Equal(t, tt.wantComponent, node.Component)
		})
	}
}

func TestOverlayPlanGraphRecordsStepStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		op   string
		want engine.ResourceStatus
	}{
		{"create", engine.StatusCreating},
		{"update", engine.StatusUpdating},
		{"replace", engine.StatusReplacing},
		{"create-replacement", engine.StatusReplacing},
		{"delete", engine.StatusDeleting},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			t.Parallel()
			plan := &ingest.PulumiPlan{Steps: []ingest.PulumiStep{stepWithDeps(tt.op, urnCache, urnDB)}}

			node := OverlayPlanGraph(engine.DependencyGraph{}, plan).Nodes[urnCache]

			assert.Equal(t, tt.want, node.Status)
		})
	}
}

func TestOverlayPlanGraphFollowsEveryLinkKind(t *testing.T) {
	t.Parallel()

	base := BuildDependencyGraph([]ingest.StackExportResource{
		{URN: urnAPI, Custom: true, Dependencies: []string{urnDB}, Provider: providerRef},
	})
	tests := []struct {
		name string
		step ingest.PulumiStep
		urn  string
		want []string
	}{
		{
			name: "a provider reference with the unknown id still names the provider",
			step: ingest.PulumiStep{Op: "create", URN: urnCache, NewState: &ingest.PulumiState{
				URN:      urnCache,
				Type:     "kubernetes:apps/v1:Deployment",
				Provider: urnProvider + "::" + proto.UnknownPulumiValue,
			}},
			urn:  urnCache,
			want: []string{urnProvider},
		},
		{
			name: "an update replaces the whole link set",
			step: ingest.PulumiStep{Op: "update", URN: urnAPI, NewState: &ingest.PulumiState{
				URN: urnAPI, Type: "aws:ecs/service:Service", DeletedWith: urnGroup, ReplaceWith: []string{urnLB},
			}},
			urn:  urnAPI,
			want: []string{urnLB, urnGroup},
		},
		{
			name: "a step without state uses the step's own provider",
			step: ingest.PulumiStep{
				Op: "create", URN: urnCache, Type: "kubernetes:apps/v1:Deployment",
				Provider: providerRef, Dependencies: []string{urnDB},
			},
			urn:  urnCache,
			want: []string{urnDB, urnProvider},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			graph := OverlayPlanGraph(base, &ingest.PulumiPlan{Steps: []ingest.PulumiStep{tt.step}})

			assert.Equal(t, tt.want, graph.Nodes[tt.urn].DependsOn)
			assert.Equal(t, []string{urnDB, urnProvider}, base.Nodes[urnAPI].DependsOn, "base must not change")
		})
	}
}

func TestWithPlanNilReturnsStateGraph(t *testing.T) {
	t.Parallel()

	snap := overlayState(t)
	withPlan := snap.WithPlan(&ingest.PulumiPlan{Steps: []ingest.PulumiStep{stepWithDeps("create", urnCache, urnDB)}})
	require.Contains(t, withPlan.Graph().Nodes, urnCache)

	stateOnly := withPlan.WithPlan(nil).Graph()

	assert.Equal(t, graphEdges(snap.Graph()), graphEdges(stateOnly))
	assert.NotContains(t, stateOnly.Nodes, urnCache)
}

func TestWithPlanBuildsGraphForPlanOnlySnapshot(t *testing.T) {
	t.Parallel()

	plan := &ingest.PulumiPlan{Steps: []ingest.PulumiStep{
		stepWithDeps("create", urnDB),
		stepWithDeps("create", urnAPI, urnDB),
	}}
	snap, err := LoadPlan(context.Background(), Source{PlanPath: inputFile(t, "plan.json", planJSON)})
	require.NoError(t, err)

	graph := snap.WithPlan(plan).Graph()

	assert.True(t, graph.Available)
	assert.Equal(t, []string{urnDB}, graph.Nodes[urnAPI].DependsOn)
}

func graphEdges(graph engine.DependencyGraph) map[string][]string {
	edges := make(map[string][]string, len(graph.Nodes))
	for urn, node := range graph.Nodes {
		edges[urn] = node.DependsOn
	}
	return edges
}

func TestClusterRadiusReachesWorkloadsThroughItsProvider(t *testing.T) {
	t.Parallel()

	const urnCluster = "urn:pulumi:dev::demo::civo:index/kubernetesCluster:KubernetesCluster::cluster"
	workloads := []string{
		"urn:pulumi:dev::demo::kubernetes:apps/v1:Deployment::game",
		"urn:pulumi:dev::demo::kubernetes:core/v1:Namespace::tailscale",
		"urn:pulumi:dev::demo::kubernetes:core/v1:Secret::oauth",
		"urn:pulumi:dev::demo::kubernetes:core/v1:Service::game",
		"urn:pulumi:dev::demo::kubernetes:helm.sh/v3:Release::operator",
		"urn:pulumi:dev::demo::kubernetes:networking.k8s.io/v1:Ingress::game",
		"urn:pulumi:dev::demo::kubernetes:rbac.authorization.k8s.io/v1:ClusterRoleBinding::admin",
	}
	resources := []ingest.StackExportResource{
		{URN: urnCluster, Type: "civo:index/kubernetesCluster:KubernetesCluster", Custom: true},
		{
			URN: urnProvider, Type: "pulumi:providers:kubernetes", Custom: true,
			PropertyDependencies: map[string][]string{"kubeconfig": {urnCluster}},
		},
	}
	rows := []engine.OverviewRow{{URN: urnProvider, Type: "pulumi:providers:kubernetes", Status: engine.StatusActive}}
	for _, urn := range workloads {
		resources = append(resources, ingest.StackExportResource{URN: urn, Custom: true, Provider: providerRef})
		rows = append(rows, engine.OverviewRow{URN: urn, Status: engine.StatusActive})
	}
	resources[2].DeletedWith = urnCluster
	rows[1].ProjectedCost = &engine.ProjectedCostData{MonthlyCost: 12.5, Currency: "USD"}

	radius := engine.ComputeBlastRadius(BuildDependencyGraph(resources), rows, urnCluster)

	require.True(t, radius.Available)
	assert.Equal(t, 8, radius.Count)
	require.Len(t, radius.Dependents, 8)
	depth := make(map[string]int, len(radius.Dependents))
	for _, d := range radius.Dependents {
		depth[d.URN] = d.Depth
	}
	assert.Equal(t, 1, depth[urnProvider])
	assert.Equal(t, 1, depth[workloads[0]], "deletedWith reaches it directly, at its shortest distance")
	for _, urn := range workloads[1:] {
		assert.Equal(t, 2, depth[urn], urn)
	}
	require.NotNil(t, radius.MonthlyTotal)
	assert.InDelta(t, 12.5, *radius.MonthlyTotal, 0.001)
	assert.Equal(t, 7, radius.LeftOut.Unpriced)
}
