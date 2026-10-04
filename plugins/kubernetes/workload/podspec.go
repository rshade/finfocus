package workload

import (
	"fmt"
	"strconv"

	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/plugins/kubernetes/usage"
)

// PulumiUnknown is the value Pulumi writes for an input not known at preview.
const PulumiUnknown = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"

// PodRequests is one pod's effective request read from a declared pod spec.
type PodRequests struct {
	CPUCores  float64
	MemoryGiB float64
	// CPUDeclared and MemoryDeclared report whether any container, init
	// container, or the pod itself declared a request or limit for each.
	CPUDeclared    bool
	MemoryDeclared bool
	// UsedLimits reports that a request was absent and its limit stood in.
	UsedLimits bool
}

// UnknownValueError reports a Pulumi unknown where a count or quantity is needed.
type UnknownValueError struct {
	Path string
}

func (e *UnknownValueError) Error() string {
	return e.Path + " is not known until deployment"
}

// InvalidQuantityError reports a value that is not a Kubernetes quantity.
type InvalidQuantityError struct {
	Path  string
	Value string
}

func (e *InvalidQuantityError) Error() string {
	return fmt.Sprintf("%s=%q is not a valid quantity", e.Path, e.Value)
}

// ReadPodSpec reads the pod spec at path within attrs and returns its
// effective request, using the rule cost cluster applies to live pods
// (usage.EffectiveRequests). It reads only containers, init containers, their
// resources and restartPolicy, and pod-level resources, so a value elsewhere
// in the spec cannot fail the read. A plan's requests are not defaulted by the
// API server, so a container's limit stands in for an absent request first.
func ReadPodSpec(attrs *structpb.Struct, path string) (PodRequests, error) {
	var out PodRequests
	var spec corev1.PodSpec
	var err error
	if spec.Containers, err = readContainers(attrs, path+".containers", &out); err != nil {
		return PodRequests{}, err
	}
	if spec.InitContainers, err = readContainers(attrs, path+".initContainers", &out); err != nil {
		return PodRequests{}, err
	}
	pod, set, err := readRequirements(attrs, path+".resources", &out)
	if err != nil {
		return PodRequests{}, err
	}
	if set {
		spec.Resources = &pod
	}
	out.CPUCores, out.MemoryGiB = usage.EffectiveRequests(spec)
	return out, nil
}

func readContainers(attrs *structpb.Struct, path string, out *PodRequests) ([]corev1.Container, error) {
	value, ok := pluginsdk.AttributeValue(attrs, path)
	if !ok {
		return nil, nil
	}
	if isUnknown(value) {
		return nil, &UnknownValueError{Path: path}
	}
	items := value.GetListValue().GetValues()
	containers := make([]corev1.Container, 0, len(items))
	for i := range items {
		itemPath := path + "." + strconv.Itoa(i)
		requirements, _, err := readRequirements(attrs, itemPath+".resources", out)
		if err != nil {
			return nil, err
		}
		container := corev1.Container{Resources: requirements}
		policy, found := pluginsdk.AttributeValue(attrs, itemPath+".restartPolicy")
		if found && policy.GetStringValue() == string(corev1.ContainerRestartPolicyAlways) {
			always := corev1.ContainerRestartPolicyAlways
			container.RestartPolicy = &always
		}
		containers = append(containers, container)
	}
	return containers, nil
}

// readRequirements reads cpu and memory requests at path, with a limit
// standing in for an absent request. It reports whether anything was set.
func readRequirements(
	attrs *structpb.Struct, path string, out *PodRequests,
) (corev1.ResourceRequirements, bool, error) {
	for _, container := range []string{path, path + ".requests", path + ".limits"} {
		if value, ok := pluginsdk.AttributeValue(attrs, container); ok && isUnknown(value) {
			return corev1.ResourceRequirements{}, false, &UnknownValueError{Path: container}
		}
	}
	requirements := corev1.ResourceRequirements{Requests: corev1.ResourceList{}}
	set := false
	for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
		request, hasRequest, err := readQuantity(attrs, path+".requests."+string(name))
		if err != nil {
			return corev1.ResourceRequirements{}, false, err
		}
		limit, hasLimit, err := readQuantity(attrs, path+".limits."+string(name))
		if err != nil {
			return corev1.ResourceRequirements{}, false, err
		}
		switch {
		case hasRequest:
			requirements.Requests[name] = request
		case hasLimit:
			requirements.Requests[name] = limit
			out.UsedLimits = true
		default:
			continue
		}
		set = true
		if name == corev1.ResourceCPU {
			out.CPUDeclared = true
		} else {
			out.MemoryDeclared = true
		}
	}
	return requirements, set, nil
}

func readQuantity(attrs *structpb.Struct, path string) (resource.Quantity, bool, error) {
	value, ok := pluginsdk.AttributeValue(attrs, path)
	if !ok {
		return resource.Quantity{}, false, nil
	}
	var text string
	switch kind := value.GetKind().(type) {
	case *structpb.Value_NullValue:
		return resource.Quantity{}, false, nil
	case *structpb.Value_StringValue:
		if kind.StringValue == PulumiUnknown {
			return resource.Quantity{}, false, &UnknownValueError{Path: path}
		}
		text = kind.StringValue
	case *structpb.Value_NumberValue:
		text = strconv.FormatFloat(kind.NumberValue, 'f', -1, 64)
	default:
		return resource.Quantity{}, false, &InvalidQuantityError{Path: path, Value: value.String()}
	}
	quantity, err := resource.ParseQuantity(text)
	if err != nil || quantity.Sign() < 0 {
		return resource.Quantity{}, false, &InvalidQuantityError{Path: path, Value: text}
	}
	return quantity, true, nil
}

func isUnknown(value *structpb.Value) bool {
	return value.GetStringValue() == PulumiUnknown
}
