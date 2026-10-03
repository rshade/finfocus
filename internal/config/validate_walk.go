package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/tailscale/hujson"
)

func configStructType() reflect.Type {
	return reflect.TypeFor[Config]()
}

func walkObject(
	object *hujson.Object,
	path string,
	typ reflect.Type,
	src *configSource,
	result *ValidationResult,
) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Map {
		walkMap(object, path, typ, src, result)
		return
	}
	if typ.Kind() != reflect.Struct {
		return
	}

	fields, open := structLayout(typ)
	known := make([]string, 0, len(fields)+len(legacyBudgetNames()))
	byName := make(map[string]reflect.Type, len(fields))
	for _, field := range fields {
		known = append(known, field.name)
		byName[field.name] = field.typ
	}
	legacy := path == costBudgetsPath
	if legacy {
		known = append(known, legacyBudgetNames()...)
	}
	legacyRaw := map[string]json.RawMessage{}

	for i := range object.Members {
		visitMember(&object.Members[i], path, known, byName, legacy, legacyRaw, open, src, result)
	}
	if len(legacyRaw) > 0 {
		validateLegacyBudget(legacyRaw, src, result)
	}
}

func visitMember(
	member *hujson.ObjectMember,
	path string,
	known []string,
	byName map[string]reflect.Type,
	legacy bool,
	legacyRaw map[string]json.RawMessage,
	open bool,
	src *configSource,
	result *ValidationResult,
) {
	name, ok := literalString(member.Name)
	if !ok || member.Value.Value == nil {
		return
	}
	line := src.lineAt(member.Name.StartOffset)
	child := joinPath(path, name)
	src.lines[child] = line
	kind := member.Value.Value.Kind()

	if legacy && isLegacyBudgetKey(name) {
		noteLegacyField(result, child, name, line, member, legacyRaw)
		if fieldType, exists := scopedFieldType(name); exists && acceptsKind(fieldType, kind) {
			walkValue(&member.Value, child, fieldType, src, result)
		}
		return
	}

	fieldType, knownField := byName[name]
	if !knownField {
		if !open {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Line:       line,
				Path:       child,
				Message:    fmt.Sprintf("%s '%s'", unknownFieldText, name),
				Suggestion: didYouMean(name, known),
			})
		}
		return
	}
	if !acceptsKind(fieldType, kind) {
		hint, example := typeHint(name)
		result.Errors = append(result.Errors, ValidationError{
			Line:    line,
			Path:    child,
			Message: typeMismatch(child, fieldType, kind),
			Hint:    hint,
			Example: example,
		})
		return
	}
	if name == fieldAlerts {
		if array, isArray := member.Value.Value.(*hujson.Array); isArray && len(array.Elements) == 0 {
			result.Errors = append(result.Errors, emptyAlertsError(child, line))
		}
	}
	walkValue(&member.Value, child, fieldType, src, result)
}

func noteLegacyField(
	result *ValidationResult,
	path, name string,
	line int,
	member *hujson.ObjectMember,
	legacyRaw map[string]json.RawMessage,
) {
	result.Warnings = append(result.Warnings, ValidationWarning{
		Line:       line,
		Path:       path,
		Message:    fmt.Sprintf("flat field '%s' is not applied", name),
		Suggestion: "Set " + costBudgetsPath + ".global." + name,
	})
	cloned := member.Value.Clone()
	cloned.Standardize()
	if packed := cloned.Pack(); len(packed) > 0 {
		legacyRaw[name] = append(json.RawMessage(nil), packed...)
	}
	if name == fieldAlerts {
		if array, isArray := member.Value.Value.(*hujson.Array); isArray && len(array.Elements) == 0 {
			result.Errors = append(result.Errors, emptyAlertsError(path, line))
		}
	}
	fieldType, exists := scopedFieldType(name)
	if !exists || member.Value.Value == nil || acceptsKind(fieldType, member.Value.Value.Kind()) {
		return
	}
	hint, example := typeHint(name)
	result.Errors = append(result.Errors, ValidationError{
		Line:    line,
		Path:    path,
		Message: typeMismatch(path, fieldType, member.Value.Value.Kind()),
		Hint:    hint,
		Example: example,
	})
}

func validateLegacyBudget(raw map[string]json.RawMessage, src *configSource, result *ValidationResult) {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return
	}
	var scope ScopedBudget
	if unmarshalErr := json.Unmarshal(encoded, &scope); unmarshalErr != nil {
		return
	}
	if validErr := scope.Validate(""); validErr != nil {
		addSemanticError(result, validErr, src.lines)
	}
}

func emptyAlertsError(path string, line int) ValidationError {
	return ValidationError{
		Line:    line,
		Path:    path,
		Message: "alerts must be non-empty when present",
		Hint:    "Omit alerts, or add at least one threshold.",
		Example: "- threshold: 80\n  type: actual",
	}
}

func walkMap(
	object *hujson.Object,
	path string,
	typ reflect.Type,
	src *configSource,
	result *ValidationResult,
) {
	elem := typ.Elem()
	for i := range object.Members {
		member := &object.Members[i]
		name, ok := literalString(member.Name)
		if !ok || member.Value.Value == nil {
			continue
		}
		child := joinPath(path, name)
		src.lines[child] = src.lineAt(member.Name.StartOffset)
		if !acceptsKind(elem, member.Value.Value.Kind()) {
			hint, example := typeHint(name)
			result.Errors = append(result.Errors, ValidationError{
				Line:    src.lines[child],
				Path:    child,
				Message: typeMismatch(child, elem, member.Value.Value.Kind()),
				Hint:    hint,
				Example: example,
			})
			continue
		}
		walkValue(&member.Value, child, elem, src, result)
	}
}

func walkValue(value *hujson.Value, path string, typ reflect.Type, src *configSource, result *ValidationResult) {
	if value == nil || value.Value == nil {
		return
	}
	switch node := value.Value.(type) {
	case *hujson.Object:
		walkObject(node, path, typ, src, result)
	case *hujson.Array:
		walkArray(node, path, typ, src, result)
	}
}

func walkArray(array *hujson.Array, path string, typ reflect.Type, src *configSource, result *ValidationResult) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Slice && typ.Kind() != reflect.Array {
		return
	}
	elem := typ.Elem()
	for i := range array.Elements {
		element := &array.Elements[i]
		child := path + "[" + strconv.Itoa(i) + "]"
		src.lines[child] = src.lineAt(element.StartOffset)
		if element.Value == nil {
			continue
		}
		if !acceptsKind(elem, element.Value.Kind()) {
			hint, example := typeHint("")
			result.Errors = append(result.Errors, ValidationError{
				Line:    src.lines[child],
				Path:    child,
				Message: typeMismatch(child, elem, element.Value.Kind()),
				Hint:    hint,
				Example: example,
			})
			continue
		}
		walkValue(element, child, elem, src, result)
	}
}

func literalString(value hujson.Value) (string, bool) {
	literal, ok := value.Value.(hujson.Literal)
	if !ok || literal.Kind() != '"' {
		return "", false
	}
	var text string
	if err := json.Unmarshal(literal, &text); err != nil {
		return "", false
	}
	return text, true
}

func joinPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

func legacyBudgetNames() []string {
	return []string{fieldAmount, fieldCurrency, fieldPeriod, fieldAlerts}
}

func isLegacyBudgetKey(name string) bool {
	return slices.Contains(legacyBudgetNames(), name)
}

func scopedFieldType(name string) (reflect.Type, bool) {
	fields, _ := structLayout(reflect.TypeFor[ScopedBudget]())
	for _, field := range fields {
		if field.name == name {
			return field.typ, true
		}
	}
	return nil, false
}

type structField struct {
	name string
	typ  reflect.Type
}

func structLayout(typ reflect.Type) ([]structField, bool) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil, true
	}
	var fields []structField
	open := false
	for field := range typ.Fields() {
		name, inline, skip := jsonFieldName(field)
		if skip {
			continue
		}
		if inline {
			merged, mergedOpen := inlineLayout(field.Type)
			fields = append(fields, merged...)
			open = open || mergedOpen
			continue
		}
		fields = append(fields, structField{name: name, typ: field.Type})
	}
	return fields, open
}

func inlineLayout(typ reflect.Type) ([]structField, bool) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Map || typ.Kind() == reflect.Interface {
		return nil, true
	}
	return structLayout(typ)
}

func jsonFieldName(field reflect.StructField) (string, bool, bool) {
	if field.PkgPath != "" && !field.Anonymous {
		return "", false, true
	}
	tag := field.Tag.Get("json")
	if tag == "" {
		if field.Anonymous {
			return "", true, false
		}
		return field.Name, false, false
	}
	name, opts, _ := strings.Cut(tag, ",")
	if name == "-" {
		return "", false, true
	}
	inline := field.Anonymous || strings.Contains(opts, "inline")
	if inline && name == "" {
		return "", true, false
	}
	if name == "" {
		name = field.Name
	}
	return name, false, false
}

func acceptsKind(typ reflect.Type, kind hujson.Kind) bool {
	if kind == 'n' {
		switch typ.Kind() { //nolint:exhaustive // null is valid only for nillable kinds
		case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface:
			return true
		default:
			return false
		}
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == reflect.TypeFor[Duration]() {
		return kind == '"'
	}
	switch typ.Kind() { //nolint:exhaustive // remaining kinds accept any JSON value
	case reflect.String:
		return kind == '"'
	case reflect.Bool:
		return kind == 't' || kind == 'f'
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return kind == '0'
	case reflect.Slice, reflect.Array:
		return kind == '['
	case reflect.Struct, reflect.Map:
		return kind == '{'
	default:
		return true
	}
}

func expectedTypeName(typ reflect.Type) string {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == reflect.TypeFor[Duration]() {
		return "duration string"
	}
	switch typ.Kind() { //nolint:exhaustive // remaining kinds share the generic name
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Struct, reflect.Map:
		return "object"
	default:
		return "value"
	}
}

func kindName(kind hujson.Kind) string {
	switch kind {
	case '"':
		return "string"
	case '0':
		return "number"
	case 't', 'f':
		return "boolean"
	case '{':
		return "object"
	case '[':
		return "array"
	case 'n':
		return "null"
	default:
		return "value"
	}
}

func typeMismatch(path string, typ reflect.Type, kind hujson.Kind) string {
	return fmt.Sprintf("'%s' must be a %s, got %s", path, expectedTypeName(typ), kindName(kind))
}

func typeHint(name string) (string, string) {
	switch name {
	case fieldAmount:
		return "Use numeric values for budget amounts (for example, amount: 100).", "amount: 100"
	case "exit_on_threshold":
		return "Use true or false.", "exit_on_threshold: true"
	case fieldExitCode:
		return "Use a whole number from 0 through 255.", "exit_code: 2"
	default:
		return "Check that the value matches the field type.", ""
	}
}
