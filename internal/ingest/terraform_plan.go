package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/resourcetype"
)

const terraformPlanFormat = "terraform-plan"
const terraformModeManaged = "managed"

// TerraformPlanMaxBytes is the advertised raw input limit for a plan document.
const TerraformPlanMaxBytes int64 = 32 * 1024 * 1024

// PlanFormatError is a plan document whose format_version major is not 1.
type PlanFormatError struct {
	Version string
}

func (e *PlanFormatError) Error() string {
	return fmt.Sprintf("terraform plan format_version %q is not major version 1", e.Version)
}

// TerraformPlanParser implements pluginsdk.CostParseProvider for format
// terraform-plan. Parse stores the operation map and before-values beside
// the response. Those are not fields of ParseResponse.
type TerraformPlanParser struct {
	mu         sync.Mutex
	operations map[string]string
	before     map[string]map[string]any
	skipped    map[string]int
}

// TerraformPlanLimits advertises content input and a 32 MiB ceiling.
func TerraformPlanLimits() *pluginsdk.ParseLimits {
	return &pluginsdk.ParseLimits{
		Formats:       []string{terraformPlanFormat},
		InputModes:    []string{"content"},
		MaxInputBytes: TerraformPlanMaxBytes,
	}
}

// TerraformPlanRequest builds the one-file content request the parser accepts.
func TerraformPlanRequest(content []byte) *pbc.ParseRequest {
	return &pbc.ParseRequest{
		Format: terraformPlanFormat,
		Input: &pbc.ParseRequest_Files{Files: &pbc.ParseFileSet{Files: []*pbc.ParseFile{{
			Path:    "plan.json",
			Content: content,
		}}}},
	}
}

// Operations returns the companion map from the last Parse call.
func (p *TerraformPlanParser) Operations() map[string]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return cloneStringMap(p.operations)
}

// Before returns old property maps from the last Parse call, keyed by source id.
func (p *TerraformPlanParser) Before() map[string]map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return cloneAnyMap(p.before)
}

// SkippedProviders returns how many managed resources each utility provider skipped.
func (p *TerraformPlanParser) SkippedProviders() map[string]int {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[string]int, len(p.skipped))
	maps.Copy(out, p.skipped)
	return out
}

// Parse validates the request, reads resource_changes, and returns a response
// that passes pluginsdk.ValidateParseResponse.
func (p *TerraformPlanParser) Parse(ctx context.Context, req *pbc.ParseRequest) (*pbc.ParseResponse, error) {
	if err := pluginsdk.ValidateParseRequest(req, TerraformPlanLimits()); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content, err := planFileContent(req)
	if err != nil {
		return nil, err
	}
	response, ops, before, skipped, err := parsePlanDocument(content)
	if err != nil {
		return nil, err
	}
	if validateErr := pluginsdk.ValidateParseResponse(req, response); validateErr != nil {
		return nil, fmt.Errorf("terraform plan parse response: %w", validateErr)
	}
	p.mu.Lock()
	p.operations = ops
	p.before = before
	p.skipped = skipped
	p.mu.Unlock()
	return response, nil
}

func planFileContent(req *pbc.ParseRequest) ([]byte, error) {
	files := req.GetFiles().GetFiles()
	if len(files) != 1 || files[0].GetPath() != "plan.json" {
		return nil, errors.New("terraform plan request must contain plan.json")
	}
	return files[0].GetContent(), nil
}

type tfPlanDocument struct {
	FormatVersion   string           `json:"format_version"`
	ResourceChanges []tfPlanResource `json:"resource_changes"`
}

type tfPlanResource struct {
	Address       string       `json:"address"`
	ModuleAddress string       `json:"module_address"`
	Mode          string       `json:"mode"`
	Type          string       `json:"type"`
	ProviderName  string       `json:"provider_name"`
	Change        tfPlanChange `json:"change"`
}

type tfPlanChange struct {
	Actions         []string        `json:"actions"`
	Before          json.RawMessage `json:"before"`
	After           json.RawMessage `json:"after"`
	AfterUnknown    json.RawMessage `json:"after_unknown"`
	BeforeSensitive json.RawMessage `json:"before_sensitive"`
	AfterSensitive  json.RawMessage `json:"after_sensitive"`
}

func parsePlanDocument(content []byte) (
	*pbc.ParseResponse,
	map[string]string,
	map[string]map[string]any,
	map[string]int,
	error,
) {
	var doc tfPlanDocument
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("decoding terraform plan JSON: %w", err)
	}
	if err := requirePlanMajor(doc.FormatVersion); err != nil {
		return nil, nil, nil, nil, err
	}
	response := &pbc.ParseResponse{}
	ops := make(map[string]string)
	before := make(map[string]map[string]any)
	skipped := make(map[string]int)
	for _, resource := range doc.ResourceChanges {
		warning, parsed, err := parsePlanResource(resource)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if warning != nil {
			response.Warnings = append(response.Warnings, warning)
			if provider, ok := skippedProviderName(resource); ok {
				skipped[provider]++
			}
			continue
		}
		response.Resources = append(response.Resources, parsed.resource)
		ops[parsed.resource.GetSourceId()] = parsed.operation
		if parsed.old != nil {
			before[parsed.resource.GetSourceId()] = parsed.old
		}
	}
	return response, ops, before, skipped, nil
}

func requirePlanMajor(version string) error {
	major, _, found := strings.Cut(version, ".")
	if !found {
		major = version
	}
	number, err := strconv.Atoi(major)
	if err != nil || number != 1 {
		return &PlanFormatError{Version: version}
	}
	return nil
}

type parsedPlanResource struct {
	resource  *pbc.ParsedResource
	operation string
	old       map[string]any
}

func parsePlanResource(resource tfPlanResource) (*pbc.ParseWarning, *parsedPlanResource, error) {
	if resource.Mode != terraformModeManaged {
		return skipWarning(resource, infoSeverity(), "skipped data source"), nil, nil
	}
	operation, ok := planOperation(resource.Change.Actions)
	if !ok {
		return skipWarning(resource, warningSeverity(), "skipped unrecognized action"), nil, nil
	}
	provider, skip := planCloudProvider(resource.Type)
	if skip {
		return skipWarning(resource, infoSeverity(), "skipped utility provider"), nil, nil
	}
	parsed, err := plannedResource(resource, provider, operation)
	if err != nil {
		return nil, nil, err
	}
	return nil, parsed, nil
}

func skippedProviderName(resource tfPlanResource) (string, bool) {
	if resource.Mode != terraformModeManaged {
		return "", false
	}
	if _, ok := planOperation(resource.Change.Actions); !ok {
		return "", false
	}
	provider, skip := planCloudProvider(resource.Type)
	return provider, skip
}

func skipWarning(resource tfPlanResource, severity pbc.ParseWarningSeverity, message string) *pbc.ParseWarning {
	return &pbc.ParseWarning{
		Severity: severity,
		Code:     "resource_skipped",
		Message:  message,
		SourceId: resource.Address,
	}
}

func infoSeverity() pbc.ParseWarningSeverity {
	return pbc.ParseWarningSeverity_PARSE_WARNING_SEVERITY_INFO
}

func warningSeverity() pbc.ParseWarningSeverity {
	return pbc.ParseWarningSeverity_PARSE_WARNING_SEVERITY_WARNING
}

func planOperation(actions []string) (string, bool) {
	switch strings.Join(actions, ",") {
	case "no-op":
		return engine.DiffOperationSame, true
	case "create":
		return engine.DiffOperationCreate, true
	case engine.DiffOperationUpdate:
		return engine.DiffOperationUpdate, true
	case "delete":
		return engine.DiffOperationDelete, true
	case "delete,create", "create,delete":
		return "replace", true
	default:
		return "", false
	}
}

func utilityProvider(provider string) bool {
	switch provider {
	case "random", "null", "helm", "time", "tls", "local", "archive", "external", "http":
		return true
	default:
		return false
	}
}

func planCloudProvider(resourceType string) (string, bool) {
	provider := resourcetype.ExtractProvider(resourceType)
	if utilityProvider(provider) {
		return provider, true
	}
	switch provider {
	case "aws", "azure", "gcp", "kubernetes":
		return provider, false
	default:
		return "custom", false
	}
}

func plannedResource(resource tfPlanResource, provider, operation string) (*parsedPlanResource, error) {
	after, before, unknown, afterSensitive, beforeSensitive, err := decodePlanChange(resource.Change)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", resource.Address, err)
	}
	sensitive := unionSensitive(beforeSensitive, afterSensitive)
	before = stripSensitive(before, sensitive)
	unknown = stripSensitive(unknown, sensitive)
	sensitivePaths := map[string]struct{}{}
	collectSensitivePaths(nil, sensitive, sensitivePaths)
	paths := withoutPaths(unknownPaths(nil, unknown), sensitivePaths)
	// Resolve unknowns against the original list positions, before redaction can
	// remove an element and shift the remaining sensitivity markers.
	if stripped, drop := applyUnknown(after, unknown); drop {
		after = nil
	} else {
		after = stripped
	}
	after = stripSensitive(after, sensitive)
	attrs, err := planAttributes(resource, after)
	if err != nil {
		return nil, err
	}
	var old map[string]any
	if _, ok := before.(map[string]any); ok {
		old = planProperties(resource, before)
	}
	return &parsedPlanResource{
		operation: operation,
		old:       old,
		resource: &pbc.ParsedResource{
			SourceId: resource.Address,
			Resource: &pbc.ResourceDescriptor{
				Provider:     provider,
				ResourceType: resource.Type,
				Attributes:   attrs,
			},
			UnknownProperties: paths,
		},
	}, nil
}

func decodePlanChange(change tfPlanChange) (any, any, any, any, any, error) {
	after, err := decodeRaw(change.After)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	before, err := decodeRaw(change.Before)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	unknown, err := decodeRaw(change.AfterUnknown)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	afterSensitive, err := decodeRaw(change.AfterSensitive)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	beforeSensitive, err := decodeRaw(change.BeforeSensitive)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return after, before, unknown, afterSensitive, beforeSensitive, nil
}

func decodeRaw(raw json.RawMessage) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil //nolint:nilnil // JSON null is a value, not a decode failure
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func planAttributes(resource tfPlanResource, after any) (*structpb.Struct, error) {
	props := planProperties(resource, after)
	if len(props) == 0 {
		return &structpb.Struct{}, nil
	}
	attrs, err := structpb.NewStruct(props)
	if err != nil {
		return nil, fmt.Errorf("encoding attributes for %s: %w", resource.Address, err)
	}
	return attrs, nil
}

func planProperties(resource tfPlanResource, value any) map[string]any {
	props := map[string]any{}
	if object, isObject := value.(map[string]any); isObject {
		props = convertKeysRecursive(object)
		if arn, isString := object["arn"].(string); isString && arn != "" {
			props[PropertyTerraformARN] = arn
		}
		if id, isString := object["id"].(string); isString && id != "" {
			props[PropertyTerraformCloudID] = id
		}
	}
	props[PropertyTerraformID] = resource.Address
	if resource.ModuleAddress != "" {
		props[PropertyTerraformModule] = resource.ModuleAddress
	}
	if resource.ProviderName != "" {
		props[PropertyTerraformProvider] = resource.ProviderName
	}
	return props
}

// AdaptTerraformPlan copies a validated parse result onto engine descriptors.
// unknown_properties is copied to terraform:unknownProperties only on that value.
func AdaptTerraformPlan(
	response *pbc.ParseResponse,
	operations map[string]string,
	before map[string]map[string]any,
) []engine.ResourceDescriptor {
	if response == nil {
		return nil
	}
	out := make([]engine.ResourceDescriptor, 0, len(response.GetResources()))
	for _, resource := range response.GetResources() {
		props := structToMap(resource.GetResource().GetAttributes())
		if paths := resource.GetUnknownProperties(); len(paths) > 0 {
			copied := make([]any, len(paths))
			for i, path := range paths {
				copied[i] = path
			}
			props[PropertyTerraformUnknown] = copied
		}
		id := resource.GetSourceId()
		out = append(out, engine.ResourceDescriptor{
			Type:          resource.GetResource().GetResourceType(),
			ID:            id,
			Provider:      resource.GetResource().GetProvider(),
			Properties:    props,
			Operation:     operations[id],
			OldProperties: before[id],
		})
	}
	return out
}

// PropertyTerraformUnknown is the engine-only copy of unknown_properties.
const PropertyTerraformUnknown = "terraform:unknownProperties"

func structToMap(attrs *structpb.Struct) map[string]any {
	if attrs == nil {
		return map[string]any{}
	}
	return attrs.AsMap()
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}

func cloneAnyMap(in map[string]map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(in))
	for key, value := range in {
		copied := make(map[string]any, len(value))
		maps.Copy(copied, value)
		out[key] = copied
	}
	return out
}
