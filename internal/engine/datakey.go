package engine

// DataKeyed is a map whose keys are domain data, such as a resource URN, a
// breakdown line name, or a service label, rather than a property name.
// Presentation redaction keeps every key and still removes Pulumi secret values.
//
// An existing plain map field opts in with the struct tag datakey:"true".
// Either mark is enough, so a new payload does not need a redactor edit.
type DataKeyed[V any] map[string]V

// KeepDataKeys marks the map so presentation redaction can tell it from a
// property map without a payload-specific type switch.
func (DataKeyed[V]) KeepDataKeys() {}
