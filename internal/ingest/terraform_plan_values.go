package ingest

import "strings"

// unionSensitive merges positional marker trees before either value tree is redacted.
// A true marker hides its entire subtree, regardless of the other marker's shape.
func unionSensitive(before, after any) any {
	if before == true || after == true {
		return true
	}
	switch left := before.(type) {
	case map[string]any:
		right, _ := after.(map[string]any)
		merged := make(map[string]any, len(left)+len(right))
		for key, child := range left {
			merged[key] = unionSensitive(child, right[key])
		}
		for key, child := range right {
			if _, exists := left[key]; !exists {
				merged[key] = child
			}
		}
		return merged
	case []any:
		right, _ := after.([]any)
		merged := make([]any, max(len(left), len(right)))
		for i := range merged {
			var beforeChild, afterChild any
			if i < len(left) {
				beforeChild = left[i]
			}
			if i < len(right) {
				afterChild = right[i]
			}
			merged[i] = unionSensitive(beforeChild, afterChild)
		}
		return merged
	default:
		return after
	}
}

func stripSensitive(value, marker any) any {
	next, drop := stripSensitiveNode(value, marker)
	if drop {
		return nil
	}
	return next
}

//nolint:gocognit // the marker tree mirrors Terraform's before_sensitive and after_sensitive JSON
func stripSensitiveNode(value, marker any) (any, bool) {
	switch typed := marker.(type) {
	case bool:
		if typed {
			return nil, true
		}
		return value, false
	case map[string]any:
		object, ok := value.(map[string]any)
		if !ok {
			return value, false
		}
		out := make(map[string]any, len(object))
		for key, child := range object {
			next, drop := stripSensitiveNode(child, typed[key])
			if drop {
				continue
			}
			out[key] = next
		}
		return out, false
	case []any:
		list, ok := value.([]any)
		if !ok {
			return value, false
		}
		out := make([]any, 0, len(list))
		for i, child := range list {
			var mark any
			if i < len(typed) {
				mark = typed[i]
			}
			next, drop := stripSensitiveNode(child, mark)
			if drop {
				continue
			}
			out = append(out, next)
		}
		return out, false
	default:
		return value, false
	}
}

func collectSensitivePaths(prefix []string, marker any, out map[string]struct{}) {
	switch typed := marker.(type) {
	case bool:
		if typed {
			out[pathFrom(prefix)] = struct{}{}
		}
	case map[string]any:
		for key, child := range typed {
			collectSensitivePaths(append(prefix, key), child, out)
		}
	case []any:
		for _, child := range typed {
			collectSensitivePaths(prefix, child, out)
		}
	}
}

func unknownPaths(prefix []string, node any) []string {
	var paths []string
	collectUnknown(prefix, node, &paths)
	return uniquePaths(paths)
}

func collectUnknown(prefix []string, node any, paths *[]string) {
	switch typed := node.(type) {
	case bool:
		if typed && len(prefix) > 0 {
			*paths = append(*paths, pathFrom(prefix))
		}
	case map[string]any:
		for key, child := range typed {
			collectUnknown(append(prefix, key), child, paths)
		}
	case []any:
		var children []string
		for _, child := range typed {
			if child == true && len(prefix) > 0 {
				children = append(children, pathFrom(prefix))
				continue
			}
			walkUnknownElement(prefix, child, &children)
		}
		// The SDK cannot traverse lists in an unknown property path. Mark
		// the enclosing list unknown and omit its value conservatively.
		if len(children) > 0 && len(prefix) > 0 {
			*paths = append(*paths, pathFrom(prefix))
		}
	}
}

func walkUnknownElement(prefix []string, child any, paths *[]string) {
	switch typed := child.(type) {
	case map[string]any:
		for key, value := range typed {
			collectUnknown(append(prefix, key), value, paths)
		}
	case []any:
		collectUnknown(prefix, typed, paths)
	}
}

//nolint:gocognit // after_unknown mirrors the planned value tree, including lists
func applyUnknown(value, unknown any) (any, bool) {
	switch typed := unknown.(type) {
	case bool:
		if typed {
			return nil, true
		}
		return value, false
	case map[string]any:
		object, ok := value.(map[string]any)
		if !ok {
			return value, false
		}
		out := make(map[string]any, len(object))
		for key, child := range object {
			next, drop := applyUnknown(child, typed[key])
			if drop {
				continue
			}
			out[key] = next
		}
		return out, false
	case []any:
		list, ok := value.([]any)
		if !ok {
			return value, false
		}
		if len(unknownPaths([]string{"list"}, typed)) > 0 {
			return nil, true
		}
		out := make([]any, len(list))
		for i, child := range list {
			var mark any
			if i < len(typed) {
				mark = typed[i]
			}
			next, drop := applyUnknown(child, mark)
			if drop {
				return nil, true
			}
			out[i] = next
		}
		return out, false
	default:
		return value, false
	}
}

func withoutPaths(paths []string, drop map[string]struct{}) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if sensitivePath(path, drop) {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func sensitivePath(path string, sensitive map[string]struct{}) bool {
	for prefix := range sensitive {
		if prefix == "" || path == prefix || strings.HasPrefix(path, prefix+".") {
			return true
		}
	}
	return false
}

func uniquePaths(paths []string) []string {
	return withoutPaths(paths, nil)
}

func pathFrom(prefix []string) string {
	parts := make([]string, len(prefix))
	for i, part := range prefix {
		parts[i] = SnakeToCamel(part)
	}
	return strings.Join(parts, ".")
}
