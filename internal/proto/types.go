package proto

// FieldMappingStatus represents the support status of a resource field in the plugin.
type FieldMappingStatus string

const (
	// StatusSupported indicates the field is fully supported and mapped.
	StatusSupported FieldMappingStatus = "SUPPORTED"
	// StatusUnsupported indicates the field is known but not supported for cost estimation.
	StatusUnsupported FieldMappingStatus = "UNSUPPORTED"
	// StatusConditional indicates the field is supported only under certain conditions.
	StatusConditional FieldMappingStatus = "CONDITIONAL"
	// StatusDynamic indicates the field's support is determined at runtime.
	StatusDynamic FieldMappingStatus = "DYNAMIC"
)

// String returns the string representation of the FieldMappingStatus.
// It implements the fmt.Stringer interface for human-readable output in logs and debug.
func (s FieldMappingStatus) String() string {
	return string(s)
}

// PluginMetadata contains information about a plugin's version and capabilities.
type PluginMetadata struct {
	Name               string            `json:"name"                         yaml:"name"`
	Version            string            `json:"version"                      yaml:"version"`
	SpecVersion        string            `json:"specVersion"                  yaml:"specVersion"`
	SupportedProviders []string          `json:"supportedProviders,omitempty" yaml:"supportedProviders,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"           yaml:"metadata,omitempty"`
	Capabilities       []string          `json:"capabilities,omitempty"       yaml:"capabilities,omitempty"`
}

// FieldMapping describes how a Pulumi resource property maps to pricing inputs.
type FieldMapping struct {
	FieldName    string             `json:"fieldName"              yaml:"fieldName"`
	Status       FieldMappingStatus `json:"status"                 yaml:"status"`
	Condition    string             `json:"condition,omitempty"    yaml:"condition,omitempty"`
	ExpectedType string             `json:"expectedType,omitempty" yaml:"expectedType,omitempty"`
}
