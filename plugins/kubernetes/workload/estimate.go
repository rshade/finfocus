package workload

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	// HoursPerMonth is the canonical month core and the other plugins use.
	HoursPerMonth = 730
	// MaxReasonLength is the decline-reason length core keeps in a note.
	MaxReasonLength = 160
	// ReasonWrongProvider declines a resource from another provider.
	ReasonWrongProvider = "kubernetes plugin prices only kubernetes:* resources"

	provider       = "kubernetes"
	liveClusterTip = "; for a live cluster use finfocus cost cluster"
)

type countRule int

const (
	countFromField countRule = iota
	countFromNodeHint
)

// kind describes where a workload type keeps its pod spec and pod count.
type kind struct {
	podSpec   string
	count     countRule
	countPath string
	usesHours bool
}

const templatePodSpec = "spec.template.spec"

// kindEntry pairs a priced workload type with how to read it.
type kindEntry struct {
	resourceType string
	kind         kind
}

// kindTable lists every workload type this plugin prices, in the order the
// decline reason names them. kindFor and ReasonUnsupportedKind both read it,
// so adding a kind updates the message.
func kindTable() []kindEntry {
	replicated := kind{podSpec: templatePodSpec, countPath: "spec.replicas"}
	return []kindEntry{
		{"kubernetes:apps/v1:Deployment", replicated},
		{"kubernetes:apps/v1:StatefulSet", replicated},
		{"kubernetes:apps/v1:DaemonSet", kind{podSpec: templatePodSpec, count: countFromNodeHint}},
		{"kubernetes:batch/v1:Job", kind{podSpec: templatePodSpec, countPath: "spec.parallelism", usesHours: true}},
		{"kubernetes:batch/v1:CronJob", kind{
			podSpec:   "spec.jobTemplate.spec.template.spec",
			countPath: "spec.jobTemplate.spec.parallelism",
			usesHours: true,
		}},
	}
}

// kindFor returns how to read a workload type, and false for every type this
// plugin does not price.
func kindFor(resourceType string) (kind, bool) {
	for _, entry := range kindTable() {
		if entry.resourceType == resourceType {
			return entry.kind, true
		}
	}
	return kind{}, false
}

// ReasonUnsupportedKind declines a kubernetes type this plugin does not
// price, naming the kinds it does. The list comes from kindTable, so it
// cannot drift from kindFor.
func ReasonUnsupportedKind() string {
	entries := kindTable()
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.resourceType[strings.LastIndex(entry.resourceType, ":")+1:]
	}
	return "kubernetes plugin prices " + joinAnd(names) + " only"
}

// joinAnd joins names with commas and an Oxford "and": "a, b, and c".
func joinAnd(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}

// Result is either a price (Reason empty) or a decline with an actionable
// Reason. Supports and GetProjectedCost share it so their answers agree.
type Result struct {
	PodHourly float64
	Monthly   float64
	Hours     float64
	Note      string
	Reason    string
}

// Priced reports whether the workload was priced.
func (r Result) Priced() bool {
	return r.Reason == ""
}

type podCount struct {
	pods   int
	source string
}

// Estimate prices a declared workload as pods × per-pod hourly request cost ×
// hours, with rates and hints from cfg. It declines, in order: a resource from
// another provider; a kubernetes type it does not price; no attributes; a
// Pulumi unknown count or quantity; an invalid count or quantity; no requests
// declared; a missing or invalid rate; a missing or invalid hint for the kind.
func Estimate(desc *pbc.ResourceDescriptor, cfg Config) Result {
	if desc.GetProvider() != provider {
		return declined(ReasonWrongProvider)
	}
	k, ok := kindFor(desc.GetResourceType())
	if !ok {
		return declined(ReasonUnsupportedKind())
	}
	attrs := desc.GetAttributes()
	if attrs == nil {
		return declined("core sent no inputs for this workload; upgrade finfocus to price it")
	}

	count, countErr := readPodCount(attrs, k, cfg)
	requests, specErr := ReadPodSpec(attrs, k.podSpec)
	if err := firstReadError(countErr, specErr); err != nil {
		return declined(err.Error())
	}
	if !requests.CPUDeclared && !requests.MemoryDeclared {
		return declined("the workload declares no resource requests; set resource requests (cpu, memory) to price it")
	}
	if reason := rateReason(cfg); reason != "" {
		return declined(reason)
	}
	if reason := hintReason(k, cfg); reason != "" {
		return declined(reason)
	}

	hours := float64(HoursPerMonth)
	if k.usesHours {
		hours = cfg.JobHoursPerMonth.Value
	}
	podHourly := requests.CPUCores*cfg.CPUHourlyRate.Value + requests.MemoryGiB*cfg.MemoryGiBHourlyRate.Value
	return Result{
		PodHourly: podHourly,
		Monthly:   float64(count.pods) * podHourly * hours,
		Hours:     hours,
		Note:      pricedNote(k, count, requests, cfg, podHourly, hours),
	}
}

func declined(reason string) Result {
	if utf8.RuneCountInString(reason) > MaxReasonLength {
		runes := []rune(reason)
		reason = string(runes[:MaxReasonLength-1]) + "…"
	}
	return Result{Reason: reason}
}

// firstReadError prefers an unknown value over an invalid one: an unknown
// explains the plan, while an invalid value may only be a preview artifact.
func firstReadError(errs ...error) error {
	for _, err := range errs {
		if _, ok := errors.AsType[*UnknownValueError](err); ok {
			return err
		}
	}
	return errors.Join(errs...)
}

func readPodCount(attrs *structpb.Struct, k kind, cfg Config) (podCount, error) {
	if k.count == countFromNodeHint {
		return podCount{pods: cfg.DaemonSetNodeCount.Value, source: "from " + EnvDaemonSetNodeCount}, nil
	}
	value, ok := pluginsdk.AttributeValue(attrs, k.countPath)
	if !ok || isNull(value) {
		return podCount{pods: 1, source: "default; " + k.countPath + " not set"}, nil
	}
	if isUnknown(value) {
		return podCount{}, &UnknownValueError{Path: k.countPath}
	}
	number, isNumber := value.GetKind().(*structpb.Value_NumberValue)
	if !isNumber || !wholeCount(number.NumberValue) {
		return podCount{}, fmt.Errorf("%s=%s is not a whole number of pods", k.countPath, shortValue(value))
	}
	return podCount{pods: int(number.NumberValue), source: k.countPath}, nil
}

func isNull(value *structpb.Value) bool {
	_, null := value.GetKind().(*structpb.Value_NullValue)
	return null
}

func wholeCount(n float64) bool {
	return n >= 0 && n <= math.MaxInt32 && n == math.Trunc(n)
}

func shortValue(value *structpb.Value) string {
	const maxLen = 24
	text := value.GetStringValue()
	if number, ok := value.GetKind().(*structpb.Value_NumberValue); ok {
		text = strconv.FormatFloat(number.NumberValue, 'f', -1, 64)
	}
	if utf8.RuneCountInString(text) > maxLen {
		text = string([]rune(text)[:maxLen]) + "…"
	}
	return strconv.Quote(text)
}

func rateReason(cfg Config) string {
	cpu, mem := cfg.CPUHourlyRate, cfg.MemoryGiBHourlyRate
	switch {
	case !cpu.Set && !mem.Set:
		return "no rate set: set " + EnvCPUHourlyRate + " and ..._MEMORY_GIB_HOURLY_RATE (USD)" + liveClusterTip
	case !cpu.Set:
		return "set " + EnvCPUHourlyRate + " (USD per vCPU-hour) to price workloads" + liveClusterTip
	case !mem.Set:
		return "set " + EnvMemoryGiBHourlyRate + " (USD per GiB-hour) to price workloads" + liveClusterTip
	case cpu.Err != nil:
		return cpu.Err.Error()
	case mem.Err != nil:
		return mem.Err.Error()
	default:
		return ""
	}
}

func hintReason(k kind, cfg Config) string {
	switch {
	case k.count == countFromNodeHint && !cfg.DaemonSetNodeCount.Set:
		return "set " + EnvDaemonSetNodeCount + " to the number of nodes to price a DaemonSet"
	case k.count == countFromNodeHint && cfg.DaemonSetNodeCount.Err != nil:
		return cfg.DaemonSetNodeCount.Err.Error()
	case k.usesHours && !cfg.JobHoursPerMonth.Set:
		return "set " + EnvJobHoursPerMonth + " to the hours it runs each month to price a Job or CronJob"
	case k.usesHours && cfg.JobHoursPerMonth.Err != nil:
		return cfg.JobHoursPerMonth.Err.Error()
	default:
		return ""
	}
}

func pricedNote(k kind, count podCount, requests PodRequests, cfg Config, podHourly, hours float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Estimated from declared requests × configured rates: %s (%s) × $%s/pod-hour",
		pods(count.pods), count.source, number(podHourly))
	fmt.Fprintf(&b, " (%s vCPU × %s + %s GiB × %s)",
		number(requests.CPUCores), number(cfg.CPUHourlyRate.Value),
		number(requests.MemoryGiB), number(cfg.MemoryGiBHourlyRate.Value))
	fmt.Fprintf(&b, " × %s h", number(hours))
	if k.usesHours {
		b.WriteString(" (from " + EnvJobHoursPerMonth + ")")
	}
	if count.pods == 0 {
		b.WriteString(" (scaled to zero)")
	}
	if !requests.CPUDeclared {
		b.WriteString(" (cpu not declared, counted as 0)")
	}
	if !requests.MemoryDeclared {
		b.WriteString(" (memory not declared, counted as 0)")
	}
	if requests.UsedLimits {
		b.WriteString(" (limits used where requests are absent)")
	}
	b.WriteString(".")
	b.WriteString(" Rates from " + EnvCPUHourlyRate + " and " + EnvMemoryGiBHourlyRate +
		" are plugin configuration, not a real node price.")
	return b.String()
}

func pods(n int) string {
	if n == 1 {
		return "1 pod"
	}
	return strconv.Itoa(n) + " pods"
}

// number formats a value to at most six decimal places without trailing zeros.
func number(v float64) string {
	const scale = 1e6
	return strconv.FormatFloat(math.Round(v*scale)/scale, 'f', -1, 64)
}
