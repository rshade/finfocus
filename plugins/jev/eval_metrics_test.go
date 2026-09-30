package jev

import (
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// auc is the probability that a random positive outranks a random negative,
// counting ties as half.
func auc(scores []float64, positive []bool) float64 {
	type sp struct {
		s float64
		p bool
	}
	xs := make([]sp, len(scores))
	for i := range scores {
		xs[i] = sp{scores[i], positive[i]}
	}
	sort.Slice(xs, func(a, b int) bool { return xs[a].s < xs[b].s })
	var rankSum, np, nn float64
	for i := 0; i < len(xs); {
		j := i
		for j+1 < len(xs) && xs[j+1].s == xs[i].s {
			j++
		}
		avg := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			if xs[k].p {
				rankSum += avg
				np++
			} else {
				nn++
			}
		}
		i = j + 1
	}
	if np == 0 || nn == 0 {
		return math.NaN()
	}
	return (rankSum - np*(np+1)/2) / (np * nn)
}

// brier is the mean squared error between probabilities and 0/1 outcomes; 0.25
// is what always answering 0.5 scores.
func brier(scores []float64, positive []bool) float64 {
	var sum float64
	for i, s := range scores {
		y := 0.0
		if positive[i] {
			y = 1
		}
		sum += (s - y) * (s - y)
	}
	return sum / float64(len(scores))
}

func ranks(xs []float64) []float64 {
	idx := make([]int, len(xs))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return xs[idx[a]] < xs[idx[b]] })
	out := make([]float64, len(xs))
	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && xs[idx[j+1]] == xs[idx[i]] {
			j++
		}
		for k := i; k <= j; k++ {
			out[idx[k]] = float64(i+j)/2 + 1
		}
		i = j + 1
	}
	return out
}

func spearman(a, b []float64) float64 {
	ra, rb := ranks(a), ranks(b)
	var ma, mb float64
	for i := range ra {
		ma += ra[i]
		mb += rb[i]
	}
	n := float64(len(ra))
	ma /= n
	mb /= n
	var num, da, db float64
	for i := range ra {
		num += (ra[i] - ma) * (rb[i] - mb)
		da += (ra[i] - ma) * (ra[i] - ma)
		db += (rb[i] - mb) * (rb[i] - mb)
	}
	if da == 0 || db == 0 {
		return math.NaN()
	}
	return num / math.Sqrt(da*db)
}

func TestMetrics(t *testing.T) {
	t.Parallel()

	assert.InDelta(t, 1.0, auc([]float64{0.1, 0.2, 0.8, 0.9}, []bool{false, false, true, true}), 1e-9)
	assert.InDelta(t, 0.0, auc([]float64{0.9, 0.8, 0.2, 0.1}, []bool{false, false, true, true}), 1e-9)
	assert.InDelta(t, 0.5, auc([]float64{0.5, 0.5, 0.5, 0.5}, []bool{false, true, false, true}), 1e-9)
	assert.True(t, math.IsNaN(auc([]float64{0.1, 0.2}, []bool{true, true})))

	assert.InDelta(t, 0.25, brier([]float64{0.5, 0.5}, []bool{true, false}), 1e-9)
	assert.InDelta(t, 0.0, brier([]float64{1, 0}, []bool{true, false}), 1e-9)

	assert.InDelta(t, 1.0, spearman([]float64{1, 2, 3}, []float64{10, 20, 30}), 1e-9)
	assert.InDelta(t, -1.0, spearman([]float64{1, 2, 3}, []float64{30, 20, 10}), 1e-9)
}
