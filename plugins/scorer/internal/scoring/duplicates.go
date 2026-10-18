package scoring

import (
	"fmt"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// pair is two recommendations, by request index, that share a resource
// identity and might describe the same change.
type pair struct {
	a, b int
}

// duplicateBlocks groups the indexes of recommendations that share a
// resource.id, keeping blocks of two or more in request order. A block longer
// than maxBlock keeps its first maxBlock members; the rest are not compared.
// Recommendations without a resource id are never grouped.
func duplicateBlocks(recs []*pbc.Recommendation, maxBlock int) [][]int {
	byID := make(map[string][]int)
	var order []string
	for i, rec := range recs {
		id := rec.GetResource().GetId()
		if id == "" {
			continue
		}
		if _, seen := byID[id]; !seen {
			order = append(order, id)
		}
		byID[id] = append(byID[id], i)
	}
	var blocks [][]int
	for _, id := range order {
		members := byID[id]
		if len(members) < 2 { //nolint:mnd // A block needs two members to hold a pair.
			continue
		}
		blocks = append(blocks, members[:min(len(members), maxBlock)])
	}
	return blocks
}

func blockPairs(blocks [][]int) []pair {
	var pairs []pair
	for _, block := range blocks {
		for i := range block {
			for j := i + 1; j < len(block); j++ {
				pairs = append(pairs, pair{a: block[i], b: block[j]})
			}
		}
	}
	return pairs
}

// duplicateGroups merges every pair judged the same into connected components
// and names each component with more than one member. It returns the group id
// by request index.
func duplicateGroups(n int, pairs []pair, same []bool) map[int]string {
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	find := func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	for i, p := range pairs {
		if !same[i] {
			continue
		}
		ra, rb := find(p.a), find(p.b)
		if ra != rb {
			parent[max(ra, rb)] = min(ra, rb)
		}
	}
	size := make(map[int]int)
	for i := range n {
		size[find(i)]++
	}
	names := make(map[int]string)
	groups := make(map[int]string)
	for i := range n {
		root := find(i)
		if size[root] < 2 { //nolint:mnd // A group needs two members.
			continue
		}
		name, ok := names[root]
		if !ok {
			name = fmt.Sprintf("dup-%d", len(names)+1)
			names[root] = name
		}
		groups[i] = name
	}
	return groups
}
