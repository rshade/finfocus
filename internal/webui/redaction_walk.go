package webui

import (
	"encoding"
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

const maxRedactDepth = 64

// dataKeyedMap is implemented by engine.DataKeyed. A plain map field uses the
// struct tag datakey:"true" instead. Both marks make the redactor treat the
// map's keys as domain data rather than property names.
type dataKeyedMap interface {
	KeepDataKeys()
}

//nolint:gochecknoglobals // immutable reflection types
var (
	dataKeyedType     = reflect.TypeFor[dataKeyedMap]()
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalType   = reflect.TypeFor[encoding.TextMarshaler]()
	stringType        = reflect.TypeFor[string]()
)

type fieldRef struct {
	index   []int
	keyed   bool
	blocked bool
}

//nolint:gochecknoglobals // per-type reflection caches
var (
	fieldCache sync.Map
	holdCache  sync.Map
)

// redactNode redacts one decoded-JSON node in a single pass. v is the Go value
// that produced the node; it is only used to learn which maps are data-keyed
// and may be invalid, in which case every map is treated as a property map.
func redactNode(v reflect.Value, node any, keyed bool, depth int) any {
	if depth > maxRedactDepth {
		v, keyed = reflect.Value{}, false
	}
	v = derefValue(v)
	if v.IsValid() {
		keyed = keyed || v.Type().Implements(dataKeyedType)
		if keyed && (v.Kind() != reflect.Map || hasOwnEncoding(v.Type())) {
			v, keyed = reflect.Value{}, false
		}
		if v.IsValid() && !keyed && !mayHoldDataKeys(v.Type()) {
			v = reflect.Value{}
		}
	}
	switch typed := node.(type) {
	case map[string]any:
		if keyed && v.IsValid() {
			return redactKeyedObject(v, typed, depth)
		}
		return redactObject(v, typed, depth)
	case []any:
		return redactSlice(v, typed, nil, depth)
	default:
		return node
	}
}

func redactObject(v reflect.Value, object map[string]any, depth int) any {
	if sensitiveRecord(object) || history.IsPulumiSecret(object) {
		return nil
	}
	fields := structFields(v)
	out := make(map[string]any, len(object))
	for key, item := range object {
		if engine.IsHiddenPropertyKey(key) || history.IsPulumiSecret(item) {
			continue
		}
		child, childKeyed := childOf(v, fields, key)
		if list, isList := item.([]any); isList && key == "deltas" {
			out[key] = redactSlice(child, list, secretDeltaMask(list, object["resource"]), depth+1)
			continue
		}
		out[key] = redactNode(child, item, childKeyed, depth+1)
	}
	return out
}

// redactKeyedObject keeps every key of a data-keyed map. Values still drop
// Pulumi secret envelopes and recurse, and a map that is itself an envelope is
// dropped whole.
func redactKeyedObject(v reflect.Value, object map[string]any, depth int) any {
	if history.IsPulumiSecret(object) {
		return nil
	}
	out := make(map[string]any, len(object))
	for key, item := range object {
		if history.IsPulumiSecret(item) {
			continue
		}
		child, _ := childOf(v, nil, key)
		out[key] = redactNode(child, item, false, depth+1)
	}
	return out
}

func redactSlice(v reflect.Value, items []any, drop []bool, depth int) []any {
	v = derefValue(v)
	indexed := v.IsValid() && (v.Kind() == reflect.Slice || v.Kind() == reflect.Array)
	out := make([]any, 0, len(items))
	for i, item := range items {
		if droppedFromArray(item) || (i < len(drop) && drop[i]) {
			continue
		}
		var child reflect.Value
		if indexed && i < v.Len() {
			child = v.Index(i)
		}
		out = append(out, redactNode(child, item, false, depth+1))
	}
	return out
}

// childOf finds the Go value behind a JSON key. It returns an invalid value
// when the key has no known source, which makes the child a property map.
func childOf(v reflect.Value, fields map[string]fieldRef, key string) (reflect.Value, bool) {
	if !v.IsValid() {
		return reflect.Value{}, false
	}
	switch v.Kind() { //nolint:exhaustive // only structs and maps hold named children
	case reflect.Struct:
		ref, ok := fields[key]
		if !ok {
			return reflect.Value{}, false
		}
		field, err := v.FieldByIndexErr(ref.index)
		if err != nil {
			return reflect.Value{}, false
		}
		return field, ref.keyed
	case reflect.Map:
		keyType := v.Type().Key()
		if keyType.Kind() != reflect.String {
			return reflect.Value{}, false
		}
		name := reflect.ValueOf(key)
		if keyType != stringType {
			name = name.Convert(keyType)
		}
		return v.MapIndex(name), false
	default:
		return reflect.Value{}, false
	}
}

func structFields(v reflect.Value) map[string]fieldRef {
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil
	}
	t := v.Type()
	if cached, ok := fieldCache.Load(t); ok {
		fields, _ := cached.(map[string]fieldRef)
		return fields
	}
	fields := buildFields(t)
	fieldCache.Store(t, fields)
	return fields
}

type fieldLevel struct {
	typ   reflect.Type
	index []int
}

type fieldCandidate struct {
	ref    fieldRef
	tagged bool
}

// fieldTable collects JSON names the way encoding/json resolves them: a
// shallower name wins, and among names at one depth a single explicitly tagged
// field wins; otherwise the name belongs to nobody.
type fieldTable struct {
	fields     map[string]fieldRef
	candidates map[string][]fieldCandidate
}

func (t *fieldTable) add(name string, candidate fieldCandidate) {
	t.candidates[name] = append(t.candidates[name], candidate)
}

func (t *fieldTable) endLevel() {
	for name, candidates := range t.candidates {
		if _, shallower := t.fields[name]; shallower {
			continue
		}
		if len(candidates) > 1 {
			candidates = slices.DeleteFunc(candidates, func(c fieldCandidate) bool { return !c.tagged })
		}
		if len(candidates) == 1 {
			t.fields[name] = candidates[0].ref
		}
		t.blockedNames(name)
	}
	t.candidates = map[string][]fieldCandidate{}
}

// blockedNames keeps a deeper field from taking a name this depth already claimed.
func (t *fieldTable) blockedNames(name string) {
	if _, ok := t.fields[name]; !ok {
		t.fields[name] = fieldRef{index: nil, blocked: true}
	}
}

// buildFields maps each JSON name of a struct to its Go field, promoting
// embedded structs the way encoding/json does.
func buildFields(root reflect.Type) map[string]fieldRef {
	table := &fieldTable{fields: map[string]fieldRef{}, candidates: map[string][]fieldCandidate{}}
	seen := map[reflect.Type]bool{}
	for level := []fieldLevel{{typ: root}}; len(level) > 0; {
		var next []fieldLevel
		for _, current := range level {
			if seen[current.typ] {
				continue
			}
			seen[current.typ] = true
			for field := range current.typ.Fields() {
				name, embedded, skip := jsonFieldName(field)
				if skip {
					continue
				}
				index := append(slices.Clone(current.index), field.Index[0])
				if embedded != nil {
					next = append(next, fieldLevel{typ: embedded, index: index})
					continue
				}
				table.add(name, fieldCandidate{
					ref:    fieldRef{index: index, keyed: field.Tag.Get("datakey") == "true"},
					tagged: jsonTagName(field) != "",
				})
			}
		}
		table.endLevel()
		level = next
	}
	maps.DeleteFunc(table.fields, func(_ string, ref fieldRef) bool { return ref.blocked })
	return table.fields
}

func jsonTagName(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	return name
}

// jsonFieldName returns the JSON key of a field, or the struct type of an
// embedded field whose members are promoted into the parent object.
func jsonFieldName(field reflect.StructField) (string, reflect.Type, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", nil, true
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" && field.Anonymous {
		embedded := field.Type
		if embedded.Kind() == reflect.Pointer {
			embedded = embedded.Elem()
		}
		if embedded.Kind() == reflect.Struct {
			return "", embedded, false
		}
	}
	if !field.IsExported() {
		return "", nil, true
	}
	if name == "" {
		name = field.Name
	}
	return name, nil, false
}

// mayHoldDataKeys reports whether a value of type t can contain a data-keyed
// map. Types that cannot skip the Go-value walk and use the plain decoded
// rules. A type with its own JSON encoding is opaque: its output shape does
// not follow its fields, so it is never matched against them.
func mayHoldDataKeys(t reflect.Type) bool {
	if cached, ok := holdCache.Load(t); ok {
		holds, _ := cached.(bool)
		return holds
	}
	holds := holdsDataKeys(t, map[reflect.Type]bool{})
	holdCache.Store(t, holds)
	return holds
}

func holdsDataKeys(t reflect.Type, visiting map[reflect.Type]bool) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if hasOwnEncoding(t) || visiting[t] {
		return false
	}
	if t.Implements(dataKeyedType) {
		return true
	}
	visiting[t] = true
	switch t.Kind() { //nolint:exhaustive // scalar kinds cannot hold a map
	case reflect.Interface:
		return true
	case reflect.Map, reflect.Slice, reflect.Array:
		return holdsDataKeys(t.Elem(), visiting)
	case reflect.Struct:
		for field := range t.Fields() {
			if field.Tag.Get("datakey") == "true" || holdsDataKeys(field.Type, visiting) {
				return true
			}
		}
	}
	return false
}

func hasOwnEncoding(t reflect.Type) bool {
	pointer := reflect.PointerTo(t)
	return pointer.Implements(jsonMarshalerType) || pointer.Implements(textMarshalType)
}

func derefValue(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}
