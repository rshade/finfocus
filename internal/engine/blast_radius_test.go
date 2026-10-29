package engine

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const blastURNPrefix = "urn:pulumi:dev::app::"

func blastURN(kind, name string) string {
	return blastURNPrefix + kind + "::" + name
}

func blastGraph(nodes ...DependencyNode) DependencyGraph {
	graph := DependencyGraph{Available: true, Nodes: make(map[string]DependencyNode, len(nodes))}
	for _, n := range nodes {
		graph.Nodes[n.URN] = n
	}
	return graph
}

func blastRow(urn, resourceType string, monthly float64) OverviewRow {
	return OverviewRow{
		URN:           urn,
		Type:          resourceType,
		Status:        StatusActive,
		ProjectedCost: &ProjectedCostData{MonthlyCost: monthly, Currency: "USD"},
	}
}

func blastURNs(radius BlastRadius) []string {
	urns := make([]string, 0, len(radius.Dependents))
	for _, d := range radius.Dependents {
		urns = append(urns, d.URN)
	}
	return urns
}

func TestComputeBlastRadius_Chain(t *testing.T) {
	t.Parallel()

	db := blastURN("aws:rds/instance:Instance", "db")
	svc := blastURN("aws:ecs/service:Service", "svc")
	lb := blastURN("aws:lb/loadBalancer:LoadBalancer", "lb")
	graph := blastGraph(
		DependencyNode{URN: db},
		DependencyNode{URN: svc, DependsOn: []string{db}},
		DependencyNode{URN: lb, DependsOn: []string{svc}},
	)
	rows := []OverviewRow{
		blastRow(db, "aws:rds/instance:Instance", 100),
		blastRow(svc, "aws:ecs/service:Service", 40),
		blastRow(lb, "aws:lb/loadBalancer:LoadBalancer", 20),
	}

	radius := ComputeBlastRadius(graph, rows, db)

	require.True(t, radius.Available)
	require.Len(t, radius.Dependents, 2)
	assert.Equal(t, svc, radius.Dependents[0].URN)
	assert.Equal(t, 1, radius.Dependents[0].Depth)
	assert.Equal(t, db, radius.Dependents[0].Via)
	assert.Equal(t, "svc", radius.Dependents[0].DisplayName)
	assert.Equal(t, "aws:ecs/service:Service", radius.Dependents[0].Type)
	assert.Equal(t, lb, radius.Dependents[1].URN)
	assert.Equal(t, 2, radius.Dependents[1].Depth)
	assert.Equal(t, svc, radius.Dependents[1].Via)
	assert.Equal(t, 2, radius.Count)
	require.NotNil(t, radius.MonthlyTotal)
	assert.InDelta(t, 60.0, *radius.MonthlyTotal, 0.001)
	assert.False(t, radius.MixedCurrency)
}

func TestComputeBlastRadius_Shapes(t *testing.T) {
	t.Parallel()

	root := blastURN("t:m:Root", "root")
	a := blastURN("t:m:A", "a")
	b := blastURN("t:m:B", "b")
	c := blastURN("t:m:C", "c")

	tests := []struct {
		name  string
		nodes []DependencyNode
		want  []string
		depth map[string]int
		via   map[string]string
	}{
		{
			name: "diamond lists the join once at its shortest depth",
			nodes: []DependencyNode{
				{URN: root},
				{URN: a, DependsOn: []string{root}},
				{URN: b, DependsOn: []string{root}},
				{URN: c, DependsOn: []string{a, b}},
			},
			want:  []string{a, b, c},
			depth: map[string]int{a: 1, b: 1, c: 2},
			via:   map[string]string{c: a},
		},
		{
			name: "shortest path wins over a longer one",
			nodes: []DependencyNode{
				{URN: root},
				{URN: a, DependsOn: []string{root}},
				{URN: b, DependsOn: []string{a}},
				{URN: c, DependsOn: []string{root, b}},
			},
			want:  []string{a, c, b},
			depth: map[string]int{a: 1, c: 1, b: 2},
			via:   map[string]string{c: root, b: a},
		},
		{
			name: "cycle terminates and never lists the selected resource",
			nodes: []DependencyNode{
				{URN: root, DependsOn: []string{b}},
				{URN: a, DependsOn: []string{root}},
				{URN: b, DependsOn: []string{a}},
			},
			want:  []string{a, b},
			depth: map[string]int{a: 1, b: 2},
		},
		{
			name: "dependents sort by URN within a level",
			nodes: []DependencyNode{
				{URN: root},
				{URN: c, DependsOn: []string{root}},
				{URN: a, DependsOn: []string{root}},
				{URN: b, DependsOn: []string{root}},
			},
			want:  []string{a, b, c},
			depth: map[string]int{a: 1, b: 1, c: 1},
		},
		{
			name: "leaf has no dependents",
			nodes: []DependencyNode{
				{URN: root, DependsOn: []string{a}},
				{URN: a},
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rows := []OverviewRow{
				blastRow(root, "t:m:Root", 1),
				blastRow(a, "t:m:A", 1),
				blastRow(b, "t:m:B", 1),
				blastRow(c, "t:m:C", 1),
			}
			radius := ComputeBlastRadius(blastGraph(tt.nodes...), rows, root)

			require.True(t, radius.Available)
			assert.Equal(t, tt.want, blastURNs(radius))
			for _, d := range radius.Dependents {
				assert.Equal(t, tt.depth[d.URN], d.Depth, d.URN)
				if wantVia, ok := tt.via[d.URN]; ok {
					assert.Equal(t, wantVia, d.Via, d.URN)
				}
			}
			assert.Equal(t, len(tt.want), radius.Count)
		})
	}
}

func TestComputeBlastRadius_ViaPrecedesDependent(t *testing.T) {
	t.Parallel()

	root := blastURN("t:m:Root", "root")
	urns := []string{
		blastURN("t:m:N", "n1"), blastURN("t:m:N", "n2"), blastURN("t:m:N", "n3"),
		blastURN("t:m:N", "n4"), blastURN("t:m:N", "n5"),
	}
	graph := blastGraph(
		DependencyNode{URN: root},
		DependencyNode{URN: urns[0], DependsOn: []string{root}},
		DependencyNode{URN: urns[1], DependsOn: []string{root}},
		DependencyNode{URN: urns[2], DependsOn: []string{urns[1], urns[0]}},
		DependencyNode{URN: urns[3], DependsOn: []string{urns[2]}},
		DependencyNode{URN: urns[4], DependsOn: []string{urns[3], urns[1]}},
	)

	radius := ComputeBlastRadius(graph, nil, root)

	seen := map[string]bool{root: true}
	for _, d := range radius.Dependents {
		assert.True(t, seen[d.Via], "%s listed before its via %s", d.URN, d.Via)
		assert.False(t, seen[d.URN], "%s listed twice or is the root", d.URN)
		seen[d.URN] = true
	}
	assert.Len(t, radius.Dependents, len(urns))
}

func TestComputeBlastRadius_Deterministic(t *testing.T) {
	t.Parallel()

	root := blastURN("t:m:Root", "root")
	nodes := []DependencyNode{{URN: root}}
	for _, name := range []string{"e", "b", "d", "a", "c"} {
		nodes = append(nodes, DependencyNode{URN: blastURN("t:m:N", name), DependsOn: []string{root}})
	}
	graph := blastGraph(nodes...)

	first := ComputeBlastRadius(graph, nil, root)
	for range 20 {
		assert.Equal(t, first, ComputeBlastRadius(graph, nil, root))
	}
}

func TestComputeBlastRadius_Availability(t *testing.T) {
	t.Parallel()

	root := blastURN("t:m:Root", "root")

	t.Run("unavailable graph", func(t *testing.T) {
		t.Parallel()

		radius := ComputeBlastRadius(DependencyGraph{}, nil, root)
		assert.Equal(t, BlastRadius{URN: root}, radius)
	})

	t.Run("unknown URN is empty and available", func(t *testing.T) {
		t.Parallel()

		graph := blastGraph(
			DependencyNode{URN: root},
			DependencyNode{URN: blastURN("t:m:A", "a"), DependsOn: []string{root}},
		)
		radius := ComputeBlastRadius(graph, nil, blastURN("t:m:Nope", "nope"))
		assert.True(t, radius.Available)
		assert.Empty(t, radius.Dependents)
		assert.Zero(t, radius.Count)
	})
}

func TestComputeBlastRadius_ComponentMembers(t *testing.T) {
	t.Parallel()

	db1 := blastURN("aws:rds/instance:Instance", "db1")
	db2 := blastURN("aws:rds/instance:Instance", "db2")
	app := blastURN("aws:ecs/service:Service", "app")
	graph := blastGraph(
		DependencyNode{URN: db1},
		DependencyNode{URN: db2},
		DependencyNode{URN: app, DependsOn: []string{db1, db2}},
	)
	rows := []OverviewRow{
		blastRow(db1, "aws:rds/instance:Instance", 10),
		blastRow(db2, "aws:rds/instance:Instance", 10),
		blastRow(app, "aws:ecs/service:Service", 5),
	}

	for _, member := range []string{db1, db2} {
		radius := ComputeBlastRadius(graph, rows, member)
		assert.Equal(t, []string{app}, blastURNs(radius), member)
	}
}

func TestComputeBlastRadius_Total(t *testing.T) {
	t.Parallel()

	root := blastURN("t:m:Root", "root")
	priced := blastURN("t:m:P", "priced")
	free := blastURN("t:m:F", "free")
	graph := blastGraph(
		DependencyNode{URN: root},
		DependencyNode{URN: priced, DependsOn: []string{root}},
		DependencyNode{URN: free, DependsOn: []string{root}},
	)
	rows := []OverviewRow{
		blastRow(priced, "t:m:P", 12.5),
		blastRow(free, "t:m:F", 0),
	}

	radius := ComputeBlastRadius(graph, rows, root)

	assert.Equal(t, 2, radius.Count)
	require.NotNil(t, radius.MonthlyTotal)
	assert.InDelta(t, 12.5, *radius.MonthlyTotal, 0.001)
	for _, d := range radius.Dependents {
		assert.True(t, d.Priced, d.URN)
		require.NotNil(t, d.MonthlyCost, d.URN)
	}
}

func TestComputeBlastRadius_MidChangeStatuses(t *testing.T) {
	t.Parallel()

	root := blastURN("t:m:Root", "root")
	graphOf := func(urns ...string) DependencyGraph {
		nodes := []DependencyNode{{URN: root}}
		for _, urn := range urns {
			nodes = append(nodes, DependencyNode{URN: urn, DependsOn: []string{root}})
		}
		return blastGraph(nodes...)
	}
	withStatus := func(urn string, status ResourceStatus, monthly float64) OverviewRow {
		row := blastRow(urn, "t:m:D", monthly)
		row.Status = status
		return row
	}

	tests := []struct {
		name         string
		status       ResourceStatus
		wantTotal    float64
		wantDeleting int
	}{
		{name: "creating counts", status: StatusCreating, wantTotal: 30},
		{name: "replacing counts", status: StatusReplacing, wantTotal: 30},
		{name: "updating counts", status: StatusUpdating, wantTotal: 30},
		{name: "deleting is listed but excluded", status: StatusDeleting, wantTotal: 0, wantDeleting: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dep := blastURN("t:m:D", "dep")

			radius := ComputeBlastRadius(graphOf(dep), []OverviewRow{withStatus(dep, tt.status, 30)}, root)

			require.Len(t, radius.Dependents, 1)
			assert.Equal(t, tt.status, radius.Dependents[0].Status)
			assert.Equal(t, 1, radius.Count)
			assert.Equal(t, tt.wantDeleting, radius.LeftOut.Deleting)
			assert.Zero(t, radius.LeftOut.Unpriced)
			if tt.wantDeleting == 1 {
				assert.Nil(t, radius.Dependents[0].MonthlyCost)
				assert.Nil(t, radius.MonthlyTotal)
				return
			}
			require.NotNil(t, radius.MonthlyTotal)
			assert.InDelta(t, tt.wantTotal, *radius.MonthlyTotal, 0.001)
		})
	}

	t.Run("a node the preview deletes is excluded even when its row is active", func(t *testing.T) {
		t.Parallel()
		dep := blastURN("t:m:D", "dep")
		graph := graphOf(dep)
		node := graph.Nodes[dep]
		node.Status = StatusDeleting
		graph.Nodes[dep] = node

		radius := ComputeBlastRadius(graph, []OverviewRow{blastRow(dep, "t:m:D", 30)}, root)

		assert.Equal(t, 1, radius.LeftOut.Deleting)
		assert.Nil(t, radius.MonthlyTotal)
	})

	for _, status := range []ResourceStatus{StatusCreating, StatusUpdating, StatusReplacing} {
		t.Run("a dependent with no row takes the preview status "+status.String(), func(t *testing.T) {
			t.Parallel()
			dep := blastURN("t:m:D", "dep")
			graph := graphOf(dep)
			node := graph.Nodes[dep]
			node.Status = status
			graph.Nodes[dep] = node

			radius := ComputeBlastRadius(graph, nil, root)

			require.Len(t, radius.Dependents, 1)
			assert.Equal(t, status, radius.Dependents[0].Status)
			assert.NotEmpty(t, radius.Dependents[0].StatusLabel)
			assert.Equal(t, 1, radius.LeftOut.Unpriced)
		})
	}
}

func TestComputeBlastRadius_Pricing(t *testing.T) {
	t.Parallel()

	root := blastURN("t:m:Root", "root")
	priced := blastURN("t:m:P", "priced")
	free := blastURN("t:m:F", "free")
	noCost := blastURN("t:m:N", "nocost")
	failed := blastURN("t:m:X", "failed")
	absent := blastURN("t:m:A", "absent")
	graph := blastGraph(
		DependencyNode{URN: root},
		DependencyNode{URN: priced, DependsOn: []string{root}},
		DependencyNode{URN: free, DependsOn: []string{root}},
		DependencyNode{URN: noCost, DependsOn: []string{root}},
		DependencyNode{URN: failed, DependsOn: []string{root}},
		DependencyNode{URN: absent, DependsOn: []string{root}},
	)
	failedRow := blastRow(failed, "t:m:X", 99)
	failedRow.ProjectedPricingFailed = true
	noCostRow := blastRow(noCost, "t:m:N", 0)
	noCostRow.ProjectedCost = nil
	rows := []OverviewRow{blastRow(priced, "t:m:P", 10), blastRow(free, "t:m:F", 0), noCostRow, failedRow}

	radius := ComputeBlastRadius(graph, rows, root)

	assert.Equal(t, 5, radius.Count)
	assert.Equal(t, 3, radius.LeftOut.Unpriced)
	require.NotNil(t, radius.MonthlyTotal)
	assert.InDelta(t, 10, *radius.MonthlyTotal, 0.001)
	byURN := map[string]BlastDependent{}
	for _, d := range radius.Dependents {
		byURN[d.URN] = d
	}
	assert.True(t, byURN[free].Priced, "a $0.00 price is a price")
	require.NotNil(t, byURN[free].MonthlyCost)
	for _, urn := range []string{noCost, failed, absent} {
		assert.False(t, byURN[urn].Priced, urn)
		assert.Nil(t, byURN[urn].MonthlyCost, urn)
	}
}

func TestComputeBlastRadius_ComponentsAreNotLeftOut(t *testing.T) {
	t.Parallel()

	root := blastURN("t:m:Root", "root")
	group := blastURN("demo:index:Group", "group")
	member := blastURN("t:m:M", "member")
	graph := blastGraph(
		DependencyNode{URN: root},
		DependencyNode{URN: group, DependsOn: []string{root}, Component: true},
		DependencyNode{URN: member, DependsOn: []string{group}},
	)
	rows := []OverviewRow{blastRow(group, "demo:index:Group", 5), blastRow(member, "t:m:M", 7)}

	radius := ComputeBlastRadius(graph, rows, root)

	require.Len(t, radius.Dependents, 2)
	assert.True(t, radius.Dependents[0].Component)
	assert.Nil(t, radius.Dependents[0].MonthlyCost)
	assert.False(t, radius.Dependents[0].Priced)
	assert.Equal(t, 1, radius.Count)
	assert.Zero(t, radius.LeftOut.Unpriced)
	require.NotNil(t, radius.MonthlyTotal)
	assert.InDelta(t, 7, *radius.MonthlyTotal, 0.001)
}

func TestComputeBlastRadius_DanglingReference(t *testing.T) {
	t.Parallel()

	root := blastURN("t:m:Root", "root")
	dep := blastURN("t:m:D", "dep")
	gone := blastURN("t:m:G", "gone")
	graph := blastGraph(
		DependencyNode{URN: root},
		DependencyNode{URN: dep, DependsOn: []string{gone, root}},
	)

	radius := ComputeBlastRadius(graph, []OverviewRow{blastRow(dep, "t:m:D", 4)}, root)

	assert.Equal(t, 1, radius.Count)
	assert.Equal(t, 1, radius.LeftOut.Unresolved)
}

func TestComputeBlastRadius_Currency(t *testing.T) {
	t.Parallel()

	root := blastURN("t:m:Root", "root")
	a := blastURN("t:m:A", "a")
	b := blastURN("t:m:B", "b")
	graph := blastGraph(
		DependencyNode{URN: root},
		DependencyNode{URN: a, DependsOn: []string{root}},
		DependencyNode{URN: b, DependsOn: []string{root}},
	)
	rowIn := func(urn, currency string, monthly float64) OverviewRow {
		row := blastRow(urn, "t:m:X", monthly)
		row.ProjectedCost.Currency = currency
		return row
	}

	tests := []struct {
		name         string
		a, b         string
		wantCurrency string
		wantMixed    bool
	}{
		{name: "one currency keeps the total", a: "EUR", b: "EUR", wantCurrency: "EUR"},
		{name: "blank is USD", a: "", b: "USD", wantCurrency: "USD"},
		{name: "mixed currencies withhold the total", a: "USD", b: "EUR", wantMixed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			radius := ComputeBlastRadius(graph, []OverviewRow{rowIn(a, tt.a, 10), rowIn(b, tt.b, 5)}, root)

			assert.Equal(t, tt.wantMixed, radius.MixedCurrency)
			assert.Equal(t, 2, radius.Count)
			if tt.wantMixed {
				assert.Nil(t, radius.MonthlyTotal)
				return
			}
			assert.Equal(t, tt.wantCurrency, radius.Currency)
			require.NotNil(t, radius.MonthlyTotal)
			assert.InDelta(t, 15, *radius.MonthlyTotal, 0.001)
		})
	}
}

// largeBlastStack builds one root with a chain of chainLen resources below it
// and fanLen direct dependents, all priced at $1.
func largeBlastStack(chainLen, fanLen int) (DependencyGraph, []OverviewRow, string) {
	root := blastURN("aws:rds/instance:Instance", "root")
	nodes := []DependencyNode{{URN: root}}
	rows := []OverviewRow{blastRow(root, "aws:rds/instance:Instance", 100)}
	previous := root
	for i := range chainLen {
		urn := blastURN("aws:ecs/service:Service", fmt.Sprintf("chain-%d", i))
		nodes = append(nodes, DependencyNode{URN: urn, DependsOn: []string{previous}})
		rows = append(rows, blastRow(urn, "aws:ecs/service:Service", 1))
		previous = urn
	}
	for i := range fanLen {
		urn := blastURN("aws:lb/loadBalancer:LoadBalancer", fmt.Sprintf("fan-%d", i))
		nodes = append(nodes, DependencyNode{URN: urn, DependsOn: []string{root}})
		rows = append(rows, blastRow(urn, "aws:lb/loadBalancer:LoadBalancer", 1))
	}
	return blastGraph(nodes...), rows, root
}

func TestComputeBlastRadius_LargeStack(t *testing.T) {
	t.Parallel()

	const chainLen, fanLen = 250, 249
	graph, rows, root := largeBlastStack(chainLen, fanLen)
	require.Len(t, rows, 500)

	start := time.Now()
	radius := ComputeBlastRadius(graph, rows, root)
	elapsed := time.Since(start)

	require.True(t, radius.Available)
	assert.Equal(t, chainLen+fanLen, radius.Count)
	require.NotNil(t, radius.MonthlyTotal)
	assert.InDelta(t, float64(chainLen+fanLen), *radius.MonthlyTotal, 0.001)
	assert.Equal(t, chainLen, radius.Dependents[len(radius.Dependents)-1].Depth)
	assert.Less(t, elapsed, 2*time.Second, "a 500-resource stack must stay interactive")
}

func BenchmarkComputeBlastRadius500(b *testing.B) {
	graph, rows, root := largeBlastStack(250, 249)
	b.ResetTimer()
	for range b.N {
		ComputeBlastRadius(graph, rows, root)
	}
}
