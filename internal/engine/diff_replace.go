// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

// collapseReplacements merges the plan steps of one Pulumi replacement into a
// single update descriptor. A replace is reported as create-replacement,
// replace, and delete-replaced steps that share a URN; priced separately they
// count the old resource twice. The merged descriptor takes its new state from
// the create-replacement (or replace) step and its old state from that step's
// OldProperties, falling back to the delete-replaced step. It keeps the
// position of the first step. A URN that is not part of a replacement keeps all
// of its entries, and a replacement step with no partner is left alone.
func collapseReplacements(resources []ResourceDescriptor) []ResourceDescriptor {
	groups := make(map[string][]int)
	for i, resource := range resources {
		if resource.ID != "" && isReplacementOperation(resource.Operation) {
			groups[resource.ID] = append(groups[resource.ID], i)
		}
	}
	merged := make(map[int]ResourceDescriptor)
	dropped := make(map[int]struct{})
	for _, indexes := range groups {
		if len(indexes) < 2 { //nolint:mnd // a replacement needs a partner step
			continue
		}
		merged[indexes[0]] = mergeReplacement(resources, indexes)
		for _, idx := range indexes[1:] {
			dropped[idx] = struct{}{}
		}
	}
	if len(merged) == 0 {
		return resources
	}
	out := make([]ResourceDescriptor, 0, len(resources)-len(dropped))
	for i, resource := range resources {
		if _, skip := dropped[i]; skip {
			continue
		}
		if replacement, ok := merged[i]; ok {
			resource = replacement
		}
		out = append(out, resource)
	}
	return out
}

func isReplacementOperation(op string) bool {
	switch op {
	case opReplace, opCreateReplacement, opDeleteReplaced:
		return true
	default:
		return false
	}
}

func mergeReplacement(resources []ResourceDescriptor, indexes []int) ResourceDescriptor {
	base := -1
	deleted := -1
	for _, idx := range indexes {
		switch resources[idx].Operation {
		case opDeleteReplaced:
			if deleted < 0 {
				deleted = idx
			}
		case opCreateReplacement:
			base = idx
		case opReplace:
			if base < 0 || resources[base].Operation != opCreateReplacement {
				base = idx
			}
		}
	}
	if base < 0 {
		return resources[indexes[0]]
	}
	out := resources[base]
	out.Operation = DiffOperationUpdate
	if out.OldProperties == nil && deleted >= 0 {
		out.OldProperties = deleteBaseline(resources[deleted])
	}
	return out
}
