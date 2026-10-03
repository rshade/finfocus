package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiff_AddedResources(t *testing.T) {
	t.Parallel()
	from := diffSnap(1, day(time.March, 15), 10, res("urn:old", "aws:ec2:Instance", 10))
	to := diffSnap(2, day(time.June, 1), 30,
		res("urn:old", "aws:ec2:Instance", 10),
		res("urn:new", "aws:rds:Instance", 20),
	)
	got, err := DiffSnapshots(from, to, 0)
	require.NoError(t, err)
	require.Len(t, got.Added, 1)
	assert.Equal(t, "urn:new", got.Added[0].URN)
	assert.InDelta(t, 20, got.Added[0].Delta, 0.001)
	assert.Empty(t, got.Removed)
}

func TestDiff_RemovedResources(t *testing.T) {
	t.Parallel()
	from := diffSnap(1, day(time.March, 15), 30,
		res("urn:keep", "aws:ec2:Instance", 10),
		res("urn:gone", "aws:ec2:Instance", 20),
	)
	to := diffSnap(2, day(time.June, 1), 10, res("urn:keep", "aws:ec2:Instance", 10))
	got, err := DiffSnapshots(from, to, 0)
	require.NoError(t, err)
	require.Len(t, got.Removed, 1)
	assert.Equal(t, "urn:gone", got.Removed[0].URN)
	assert.InDelta(t, -20, got.Removed[0].Delta, 0.001)
}

func TestDiff_ChangedResources(t *testing.T) {
	t.Parallel()
	from := diffSnap(1, day(time.March, 15), 10, res("urn:db", "aws:rds:Instance", 10))
	to := diffSnap(2, day(time.June, 1), 25, res("urn:db", "aws:rds:Instance", 25))
	got, err := DiffSnapshots(from, to, 0)
	require.NoError(t, err)
	require.Len(t, got.Changed, 1)
	assert.Equal(t, "urn:db", got.Changed[0].URN)
	assert.InDelta(t, 15, got.Changed[0].Delta, 0.001)
	assert.Equal(t, 0, got.UnchangedCount)
}

func TestDiff_UnchangedResources(t *testing.T) {
	t.Parallel()
	from := diffSnap(1, day(time.March, 15), 10, res("urn:db", "aws:rds:Instance", 10))
	same := res("urn:db", "aws:rds:Instance", 10.005)
	to := diffSnap(2, day(time.June, 1), 10.005, same)
	got, err := DiffSnapshots(from, to, 0)
	require.NoError(t, err)
	assert.Empty(t, got.Changed)
	assert.Equal(t, 1, got.UnchangedCount)
	assert.InDelta(t, 10.005, got.UnchangedTotal, 0.0001)
}

func TestDiff_EmptySnapshots(t *testing.T) {
	t.Parallel()
	empty := diffSnap(1, day(time.March, 15), 0)
	other := diffSnap(2, day(time.June, 1), 0)
	got, err := DiffSnapshots(empty, other, 0)
	require.NoError(t, err)
	assert.Empty(t, got.Added)
	assert.Empty(t, got.Removed)
	assert.Equal(t, 0, got.UnchangedCount)

	with := diffSnap(3, day(time.June, 2), 8, res("urn:new", "aws:ec2:Instance", 8))
	added, err := DiffSnapshots(empty, with, 0)
	require.NoError(t, err)
	require.Len(t, added.Added, 1)
	removed, err := DiffSnapshots(with, empty, 0)
	require.NoError(t, err)
	require.Len(t, removed.Removed, 1)
}

func TestDiff_IdenticalSnapshots(t *testing.T) {
	t.Parallel()
	snap := diffSnap(4, day(time.March, 15), 12, res("urn:a", "aws:ec2:Instance", 12))
	got, err := DiffSnapshots(snap, snap, 0)
	require.NoError(t, err)
	assert.Empty(t, got.Added)
	assert.Empty(t, got.Removed)
	assert.Empty(t, got.Changed)
	assert.Equal(t, 1, got.UnchangedCount)
	assert.InDelta(t, 0, got.Delta, 0.001)
}

func TestDiff_DestroyVersion(t *testing.T) {
	t.Parallel()
	from := diffSnap(5, day(time.March, 15), 40, res("urn:web", "aws:ec2:Instance", 40))
	to := diffSnap(6, day(time.March, 16), 0)
	got, err := DiffSnapshots(from, to, 0)
	require.NoError(t, err)
	assert.InDelta(t, -40, got.Delta, 0.001)
	require.Len(t, got.Removed, 1)
	assert.Equal(t, "urn:web", got.Removed[0].URN)
	assert.Empty(t, got.Added)
	assert.Equal(t, 0, got.UnchangedCount)
}

func TestDiff_ThresholdFilter(t *testing.T) {
	t.Parallel()
	from := diffSnap(1, day(time.March, 15), 30,
		res("urn:small", "aws:ec2:Instance", 10),
		res("urn:edge", "aws:ec2:Instance", 10),
		res("urn:big", "aws:ec2:Instance", 10),
	)
	to := diffSnap(2, day(time.June, 1), 56,
		res("urn:small", "aws:ec2:Instance", 11),
		res("urn:edge", "aws:ec2:Instance", 20),
		res("urn:big", "aws:ec2:Instance", 25),
	)
	got, err := DiffSnapshots(from, to, 10)
	require.NoError(t, err)
	require.Len(t, got.Changed, 1)
	assert.Equal(t, "urn:big", got.Changed[0].URN)
	_, err = DiffSnapshots(from, to, -1)
	require.ErrorContains(t, err, "threshold must be >= 0")
}

func TestDiff_JSONOutput(t *testing.T) {
	t.Parallel()
	from := diffSnap(35, day(time.March, 15), 10, res("urn:old", "aws:ec2:Instance", 10))
	to := diffSnap(42, day(time.June, 1), 30, res("urn:new", "aws:rds:Instance", 30))
	got, err := DiffSnapshots(from, to, 0)
	require.NoError(t, err)
	assert.Equal(t, 35, got.From.Version)
	assert.Equal(t, 42, got.To.Version)
	assert.InDelta(t, 20, got.Delta, 0.001)
	assert.Equal(t, "USD", got.Currency)
	require.Len(t, got.Added, 1)
	assert.Equal(t, "urn:new", got.Added[0].URN)
	require.Len(t, got.Removed, 1)
	assert.Equal(t, "urn:old", got.Removed[0].URN)
}

func TestDiff_DateLookup(t *testing.T) {
	t.Parallel()
	snaps := []CostSnapshot{
		diffSnap(1, day(time.March, 1), 1),
		diffSnap(2, day(time.March, 15), 2),
		diffSnap(3, day(time.June, 1), 3),
	}
	got, err := ResolveCostSnapshot(snaps, "2025-03-15")
	require.NoError(t, err)
	assert.Equal(t, 2, got.Version)

	nearest, err := ResolveCostSnapshot(snaps, "2025-03-18")
	require.NoError(t, err)
	assert.Equal(t, 2, nearest.Version)

	byVersion, err := ResolveCostSnapshot(snaps, "v3")
	require.NoError(t, err)
	assert.Equal(t, 3, byVersion.Version)
	latest, err := ResolveCostSnapshot(snaps, "")
	require.NoError(t, err)
	assert.Equal(t, 3, latest.Version)
	_, err = ResolveCostSnapshot(snaps, "v9")
	require.ErrorContains(t, err, "snapshot version 9 not found")
	_, err = ResolveCostSnapshot(nil, "v1")
	require.ErrorContains(t, err, "no cost history snapshots")
}

func TestDiff_MixedCurrency(t *testing.T) {
	t.Parallel()
	from := diffSnap(1, day(time.March, 15), 10)
	to := diffSnap(2, day(time.June, 1), 10)
	to.Currency = "EUR"
	_, err := DiffSnapshots(from, to, 0)
	require.ErrorIs(t, err, ErrMixedCurrencies)

	to.Currency = ""
	got, err := DiffSnapshots(from, to, 0)
	require.NoError(t, err)
	assert.Equal(t, "USD", got.Currency)
}

func TestDiff_SortAndRender(t *testing.T) {
	t.Parallel()
	from := diffSnap(5, day(time.June, 1), 1, res("", "aws:ec2:Instance", 1))
	to := diffSnap(2, day(time.March, 1), 70,
		res("urn:pulumi:dev::app::aws:ec2:Instance::b", "aws:ec2:Instance", 10),
		res("urn:pulumi:dev::app::aws:ec2:Instance::a", "aws:ec2:Instance", 10),
		res("urn:z", "aws:ec2:Instance", 50),
	)
	got, err := DiffSnapshots(from, to, 0)
	require.NoError(t, err)
	require.Len(t, got.Added, 3)
	assert.Equal(t, "urn:z", got.Added[0].URN)
	assert.Equal(t, "urn:pulumi:dev::app::aws:ec2:Instance::a", got.Added[1].URN)
	assert.Equal(t, "urn:pulumi:dev::app::aws:ec2:Instance::b", got.Added[2].URN)
	require.Len(t, got.Removed, 1)

	text := RenderCostDiff("dev", got)
	assert.Contains(t, text, "Stack: dev")
	assert.Contains(t, text, "Cost Delta: +$69/mo ($1 → $70)")
	assert.Contains(t, text, "Period: Jun 01, 2025 → Mar 01, 2025 (92 days, 3 versions)")
	assert.Contains(t, text, "aws:ec2:Instance  a  +$10/mo  (sku, us-east-1)")
	assert.Contains(t, text, "aws:ec2:Instance  b  +$10/mo  (sku, us-east-1)")
	assert.Contains(t, text, "aws:ec2:Instance  urn:z  +$50/mo  (sku, us-east-1)")
	assert.Contains(t, text, "aws:ec2:Instance  (unknown)  -$1/mo  (sku, us-east-1)")
	assert.Contains(t, text, "Changed Resources ($0/mo):\n  (none detected)")
	assert.Contains(t, text, "Unchanged Resources (0):\n  $0/mo across 0 resources")
}

func TestDiff_DateTieAndSelectors(t *testing.T) {
	t.Parallel()
	equidistant := []CostSnapshot{
		diffSnap(2, day(time.March, 16), 2),
		diffSnap(1, day(time.March, 14), 1),
	}
	got, err := ResolveCostSnapshot(equidistant, "2025-03-15")
	require.NoError(t, err)
	assert.Equal(t, 1, got.Version)

	sameTime := []CostSnapshot{
		diffSnap(2, day(time.March, 15), 2),
		diffSnap(1, day(time.March, 15), 1),
	}
	got, err = ResolveCostSnapshot(sameTime, "2025-03-15")
	require.NoError(t, err)
	assert.Equal(t, 1, got.Version)

	bare, err := ResolveCostSnapshot(sameTime, "1")
	require.NoError(t, err)
	assert.Equal(t, 1, bare.Version)

	_, err = ResolveCostSnapshot(sameTime, "nope")
	require.ErrorContains(t, err, "invalid snapshot selector")
	_, err = ResolveCostSnapshot(sameTime, "v")
	require.ErrorContains(t, err, "invalid snapshot selector")
}

func diffSnap(version int, at time.Time, total float64, resources ...CostResource) CostSnapshot {
	if resources == nil {
		resources = []CostResource{}
	}
	return CostSnapshot{
		Timestamp:     at,
		Version:       version,
		TotalMonthly:  total,
		Currency:      "USD",
		ResourceCount: len(resources),
		ByProvider:    map[string]float64{"aws": total},
		ByType:        map[string]float64{},
		Resources:     resources,
	}
}

func res(urn, typ string, monthly float64) CostResource {
	return CostResource{
		URN: urn, Type: typ, Provider: "aws", MonthlyCost: monthly, SKU: "sku", Region: "us-east-1",
	}
}

func day(month time.Month, d int) time.Time {
	return time.Date(2025, month, d, 0, 0, 0, 0, time.UTC)
}
