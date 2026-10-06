package viewmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

func TestFilterOverviewRows(t *testing.T) {
	t.Parallel()

	all := []engine.OverviewRowResult{
		{URN: "urn:pulumi:prod::app::aws:ec2/instance:Instance::web", Type: "aws:ec2:Instance"},
		{URN: "urn:pulumi:prod::app::aws:s3/bucket:Bucket::data", Type: "aws:s3:Bucket"},
	}

	tests := []struct {
		name       string
		filterText string
		wantURNs   []string
	}{
		{name: "empty filter returns all rows", filterText: "", wantURNs: []string{all[0].URN, all[1].URN}},
		{name: "matches URN case-insensitively", filterText: "WEB", wantURNs: []string{all[0].URN}},
		{name: "matches type case-insensitively", filterText: "bucket", wantURNs: []string{all[1].URN}},
		{name: "no match returns empty non-nil slice", filterText: "gcp", wantURNs: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FilterOverviewRows(all, tt.filterText)
			assert.NotNil(t, got)
			urns := make([]string, len(got))
			for i, r := range got {
				urns[i] = r.URN
			}
			assert.Equal(t, tt.wantURNs, urns)
		})
	}
}

func TestFilterOverviewRowsEmptyFilterCopies(t *testing.T) {
	t.Parallel()

	all := []engine.OverviewRowResult{{URN: "urn:a"}}
	got := FilterOverviewRows(all, "")
	// Mutating the result must not reorder/alias the source slice.
	got[0].URN = "urn:mutated"
	assert.Equal(t, "urn:a", all[0].URN)
}

func TestFilterCostResults(t *testing.T) {
	t.Parallel()

	all := []engine.CostResult{
		{ResourceID: "web-server", ResourceType: "aws:ec2:Instance"},
		{ResourceID: "data-bucket", ResourceType: "aws:s3:Bucket"},
	}

	t.Run("empty filter returns input slice", func(t *testing.T) {
		t.Parallel()

		got := FilterCostResults(all, "")
		assert.Equal(t, all, got)
	})

	t.Run("matches resource ID case-insensitively", func(t *testing.T) {
		t.Parallel()

		got := FilterCostResults(all, "WEB")
		assert.Len(t, got, 1)
		assert.Equal(t, "web-server", got[0].ResourceID)
	})

	t.Run("matches resource type case-insensitively", func(t *testing.T) {
		t.Parallel()

		got := FilterCostResults(all, "BUCKET")
		assert.Len(t, got, 1)
		assert.Equal(t, "data-bucket", got[0].ResourceID)
	})

	t.Run("no match returns nil slice", func(t *testing.T) {
		t.Parallel()

		got := FilterCostResults(all, "gcp")
		assert.Nil(t, got)
	})
}

func TestFilterRecommendations(t *testing.T) {
	t.Parallel()

	all := []engine.Recommendation{
		{ResourceID: "web-server", Type: "Right-sizing", Description: "Downsize instance"},
		{ResourceID: "old-disk", Type: "Delete Unused", Description: "Remove unattached volume"},
	}

	t.Run("empty filter returns input slice", func(t *testing.T) {
		t.Parallel()

		got := FilterRecommendations(all, "")
		assert.Equal(t, all, got)
	})

	tests := []struct {
		name       string
		filterText string
		wantIDs    []string
	}{
		{name: "matches resource ID", filterText: "WEB", wantIDs: []string{"web-server"}},
		{name: "matches action type", filterText: "delete", wantIDs: []string{"old-disk"}},
		{name: "matches description", filterText: "DOWNSIZE", wantIDs: []string{"web-server"}},
		{name: "no match returns nil", filterText: "gcp", wantIDs: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FilterRecommendations(all, tt.filterText)
			if tt.wantIDs == nil {
				assert.Nil(t, got)
				return
			}
			ids := make([]string, len(got))
			for i, r := range got {
				ids[i] = r.ResourceID
			}
			assert.Equal(t, tt.wantIDs, ids)
		})
	}
}
