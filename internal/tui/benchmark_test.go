package tui_test

import (
	"testing"

	"github.com/rshade/finfocus/internal/tui"
)

// Benchmarks for hot-path rendering functions.
// These functions are called frequently during CLI output rendering.

func BenchmarkFormatMoneyShort(b *testing.B) {
	amounts := []float64{0, 1234.56, -999.99, 1234567.89, 0.01}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, amount := range amounts {
			_ = tui.FormatMoneyShort(amount)
		}
	}
}

func BenchmarkFormatMoney(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = tui.FormatMoney(1234.56, "USD")
	}
}

func BenchmarkFormatPercent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = tui.FormatPercent(85.7)
	}
}

func BenchmarkRenderStatus(b *testing.B) {
	statuses := []string{"ok", "warning", "critical", "unknown"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, status := range statuses {
			_ = tui.RenderStatus(status)
		}
	}
}

func BenchmarkRenderDelta(b *testing.B) {
	deltas := []float64{25.50, -15.75, 0.0, 1234.56}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, delta := range deltas {
			_ = tui.RenderDelta(delta)
		}
	}
}

func BenchmarkRenderPriority(b *testing.B) {
	priorities := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, priority := range priorities {
			_ = tui.RenderPriority(priority)
		}
	}
}

func BenchmarkProgressBarRender(b *testing.B) {
	pb := tui.DefaultProgressBar()
	percents := []float64{0, 25, 50, 75, 100}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pct := range percents {
			_ = pb.Render(pct)
		}
	}
}

func BenchmarkDetectOutputMode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = tui.DetectOutputMode(false, false, false)
	}
}
