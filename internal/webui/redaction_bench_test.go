package webui

import (
	"fmt"
	"testing"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func benchOverviewSnapshot(rows int) OverviewSnapshot {
	source := make([]engine.OverviewRow, rows)
	for i := range source {
		source[i] = engine.OverviewRow{
			URN:    fmt.Sprintf("urn:pulumi:dev::demo::aws:ec2/instance:Instance::web-%d", i),
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
			Properties: map[string]any{
				"instanceType": "t3.large",
				"password":     "hunter2",
				"tags":         map[string]any{"Name": "web", "token": "abc"},
				"ebsBlockDevices": []any{
					map[string]any{"volumeSize": 100.0, "volumeType": "gp3"},
				},
			},
			ActualCost: &engine.ActualCostData{
				MTDCost:   12.5,
				Currency:  "USD",
				Breakdown: map[string]float64{"compute": 10, "storage": 2.5},
			},
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 40,
				Currency:    "USD",
				Breakdown:   map[string]float64{"compute": 30, "storage": 10},
			},
			PropertyDiffs: []engine.PropertyDiff{{Key: "instanceType", OldValue: "t3.small", NewValue: "t3.large"}},
		}
	}
	return OverviewSnapshot{Rows: engine.ComputeOverviewResult(source, 15).Rows, Ready: true, Stack: "dev"}
}

func benchActualCostPage(rows int) viewmodel.ActualCostPage {
	page := viewmodel.ActualCostPage{Trends: map[string]string{}, Page: 1, TotalPages: 1}
	for i := range rows {
		id := fmt.Sprintf("urn:pulumi:dev::demo::aws:ec2/instance:Instance::web-%d", i)
		page.Rows = append(
			page.Rows,
			viewmodel.CostDisplay{ID: id, Type: "aws:ec2/instance:Instance", Cost: "$12.50", Currency: "USD"},
		)
		page.Trends[id] = "▁▂▃▄▅▆▇█"
	}
	return page
}

func BenchmarkRedactJSONOverviewSnapshot(b *testing.B) {
	snapshot := benchOverviewSnapshot(300)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := RedactJSON(snapshot); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedactJSONActualCostPage(b *testing.B) {
	page := benchActualCostPage(500)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := RedactJSON(page); err != nil {
			b.Fatal(err)
		}
	}
}
