package webui

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

const maxDataKeyDepth = 64

// dataKeyedMap is implemented by engine.DataKeyed. A plain map field uses the
// struct tag datakey:"true" instead. Both marks keep keys through redaction.
type dataKeyedMap interface {
	KeepDataKeys()
}

// preserveDataKeys copies data-keyed maps from original back onto a value that
// RedactValue has already walked. JSON decoding erases the marker, so the
// original Go value is what tells a domain map from a property map.
func preserveDataKeys(original, redacted any) any {
	return preserveKeyed(reflect.ValueOf(original), redacted, 0)
}

func preserveKeyed(value reflect.Value, redacted any, depth int) any {
	if depth > maxDataKeyDepth {
		return redacted
	}
	value = derefValue(value)
	if !value.IsValid() {
		return redacted
	}
	if value.Kind() == reflect.Map && isDataKeyedValue(value) {
		return redactKeepingKeys(value, redacted, depth)
	}
	if value.Kind() == reflect.Struct {
		return preserveStruct(value, redacted, depth)
	}
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		return preserveSlice(value, redacted, depth)
	}
	if value.Kind() == reflect.Map {
		return preserveMap(value, redacted, depth)
	}
	return redacted
}

func preserveStruct(value reflect.Value, redacted any, depth int) any {
	object, ok := redacted.(map[string]any)
	if !ok {
		return redacted
	}
	structType := value.Type()
	for i := range structType.NumField() {
		field := structType.Field(i)
		name, promoted, skip := jsonFieldName(field)
		if skip {
			continue
		}
		fieldValue := derefValue(value.Field(i))
		if !fieldValue.IsValid() {
			continue
		}
		if promoted {
			preserveKeyed(fieldValue, object, depth+1)
			continue
		}
		if field.Tag.Get("datakey") == "true" || isDataKeyedValue(fieldValue) {
			restoreDataKeyedField(object, name, fieldValue, depth+1)
			continue
		}
		if child, exists := object[name]; exists {
			object[name] = preserveKeyed(fieldValue, child, depth+1)
		}
	}
	return object
}

func restoreDataKeyedField(object map[string]any, name string, fieldValue reflect.Value, depth int) {
	_, exists := object[name]
	if !exists && (dataKeyLen(fieldValue) == 0 || engine.IsHiddenPropertyKey(name)) {
		return
	}
	restored := redactKeepingKeys(fieldValue, object[name], depth)
	if restored != nil || exists {
		object[name] = restored
	}
}

func preserveSlice(value reflect.Value, redacted any, depth int) any {
	items, ok := redacted.([]any)
	if !ok || !canHoldDataKeys(value.Type().Elem()) {
		return redacted
	}
	indexes := surviving(value, len(items))
	if indexes == nil {
		return redacted
	}
	for i := range items {
		items[i] = preserveKeyed(value.Index(indexes[i]), items[i], depth+1)
	}
	return items
}

// canHoldDataKeys reports whether an element type can contain a data-keyed
// map. Scalar elements such as a []float64 cannot, so they skip the per-element
// JSON round trip in surviving.
func canHoldDataKeys(t reflect.Type) bool {
	switch t.Kind() { //nolint:exhaustive // only scalar kinds are excluded
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return false
	default:
		return true
	}
}

// surviving maps each redacted element back to its original index by replaying
// the element drop rule of redactArray. It returns nil when the counts disagree.
func surviving(value reflect.Value, kept int) []int {
	indexes := make([]int, 0, kept)
	for i := range value.Len() {
		encoded, err := json.Marshal(value.Index(i).Interface())
		if err != nil {
			return nil
		}
		var decoded any
		if json.Unmarshal(encoded, &decoded) != nil {
			return nil
		}
		if !droppedFromArray(decoded) {
			indexes = append(indexes, i)
		}
	}
	if len(indexes) != kept {
		return nil
	}
	return indexes
}

func preserveMap(value reflect.Value, redacted any, depth int) any {
	object, ok := redacted.(map[string]any)
	if !ok || object == nil {
		return redacted
	}
	for _, key := range value.MapKeys() {
		if key.Kind() != reflect.String {
			continue
		}
		name := key.String()
		child, exists := object[name]
		if !exists {
			continue
		}
		object[name] = preserveKeyed(value.MapIndex(key), child, depth+1)
	}
	return object
}

// redactKeepingKeys drops Pulumi secret values and still returns every other key.
// Nested data-keyed values are preserved from the original Go value.
func redactKeepingKeys(value reflect.Value, fallback any, depth int) any {
	value = derefValue(value)
	if !value.IsValid() || value.Kind() != reflect.Map {
		return fallback
	}
	if value.IsNil() {
		return nil
	}
	encoded, err := json.Marshal(value.Interface())
	if err != nil {
		return fallback
	}
	var decoded map[string]any
	if unmarshalErr := json.Unmarshal(encoded, &decoded); unmarshalErr != nil {
		return fallback
	}
	if history.IsPulumiSecret(decoded) {
		return nil
	}
	out := make(map[string]any, len(decoded))
	for _, key := range value.MapKeys() {
		if key.Kind() != reflect.String {
			continue
		}
		name := key.String()
		item, present := decoded[name]
		if !present || history.IsPulumiSecret(item) {
			continue
		}
		out[name] = preserveKeyed(value.MapIndex(key), RedactValue(item), depth+1)
	}
	return out
}

func isDataKeyedValue(value reflect.Value) bool {
	if !value.IsValid() || !value.CanInterface() {
		return false
	}
	_, ok := reflect.TypeAssert[dataKeyedMap](value)
	return ok
}

func dataKeyLen(value reflect.Value) int {
	value = derefValue(value)
	if !value.IsValid() || value.Kind() != reflect.Map {
		return 0
	}
	return value.Len()
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

func jsonFieldName(field reflect.StructField) (string, bool, bool) {
	if field.PkgPath != "" {
		return "", false, true
	}
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "-" {
		return "", false, true
	}
	if name == "" {
		if field.Anonymous {
			return "", true, false
		}
		return field.Name, false, false
	}
	return name, false, false
}
