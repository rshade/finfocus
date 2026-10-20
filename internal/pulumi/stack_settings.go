package pulumi

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// StackEnvForm is how a stack file describes its ESC environment.
type StackEnvForm int

const (
	// StackEnvAbsent means the stack file has no environment key.
	StackEnvAbsent StackEnvForm = iota
	// StackEnvImports is a list of environment references.
	StackEnvImports
	// StackEnvUnsupported is any other environment shape.
	StackEnvUnsupported
)

// StackEnvironment is the stack file's environment value.
type StackEnvironment struct {
	Form    StackEnvForm
	Imports []string
}

// StackSettingsNameCandidates returns stack settings names in lookup order.
// A fully qualified reference (org/project/stack) tries the short name first.
func StackSettingsNameCandidates(stackName string) []string {
	trimmed := strings.TrimSpace(stackName)
	if trimmed == "" {
		return nil
	}

	candidates := []string{trimmed}
	if idx := strings.LastIndexAny(trimmed, `/\`); idx >= 0 && idx < len(trimmed)-1 {
		candidates = append([]string{trimmed[idx+1:]}, candidates...)
	}

	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

// ReadStackSettingsFile reads Pulumi.<stack>.yaml, then Pulumi.<stack>.yml,
// for each candidate name.
func ReadStackSettingsFile(projectDir, stackName string) ([]byte, error) {
	candidates := StackSettingsNameCandidates(stackName)
	if len(candidates) == 0 {
		return nil, errors.New("stack name is required")
	}

	var lastErr error
	for _, candidate := range candidates {
		for _, ext := range []string{".yaml", ".yml"} {
			path := filepath.Join(projectDir, "Pulumi."+candidate+ext)
			data, err := os.ReadFile(path)
			if err == nil {
				return data, nil
			}
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return nil, lastErr
}

// ParseStackEnvironment reads the environment key from a stack settings file.
func ParseStackEnvironment(data []byte) (StackEnvironment, error) {
	node, err := environmentNode(data)
	if errors.Is(err, errNoEnvironment) {
		return StackEnvironment{Form: StackEnvAbsent}, nil
	}
	if err != nil {
		return StackEnvironment{}, err
	}
	switch node.Kind {
	case yaml.SequenceNode:
		return sequenceEnvironment(node), nil
	case yaml.MappingNode:
		return mappingEnvironment(node), nil
	case yaml.ScalarNode, yaml.AliasNode, yaml.DocumentNode:
		return StackEnvironment{Form: StackEnvUnsupported}, nil
	default:
		return StackEnvironment{Form: StackEnvUnsupported}, nil
	}
}

func sequenceEnvironment(node *yaml.Node) StackEnvironment {
	imports, ok := stringSequence(node)
	if !ok {
		return StackEnvironment{Form: StackEnvUnsupported}
	}
	return StackEnvironment{Form: StackEnvImports, Imports: imports}
}

func mappingEnvironment(node *yaml.Node) StackEnvironment {
	if len(node.Content) != 2 || node.Content[0].Value != "imports" {
		return StackEnvironment{Form: StackEnvUnsupported}
	}
	imports, ok := stringSequence(node.Content[1])
	if !ok {
		return StackEnvironment{Form: StackEnvUnsupported}
	}
	return StackEnvironment{Form: StackEnvImports, Imports: imports}
}

func stringSequence(node *yaml.Node) ([]string, bool) {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil, false
	}
	out := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag == "!!null" {
			return nil, false
		}
		out = append(out, item.Value)
	}
	return out, true
}

var errNoEnvironment = errors.New("stack file has no environment")

func environmentNode(data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, errNoEnvironment
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil, errNoEnvironment
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "environment" {
			return mapping.Content[i+1], nil
		}
	}
	return nil, errNoEnvironment
}
