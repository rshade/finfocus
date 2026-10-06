// Package promql builds the instant queries for one historical usage window.
// Evaluation time is the window end. The integral is sum_over_time over a
// fixed 60s step, so a counter reset keeps both sides and a sample that does
// not exist adds nothing.
package promql

import (
	"strconv"
	"strings"
	"time"
)

const (
	rateWindow = "5m"
	substep    = "60s"
)

// Query is one instant query and the time it is evaluated at.
type Query struct {
	Expr string
	Time time.Time
}

// Selectors are the PromQL matchers taken from the stats request.
// Namespace limits workload series. Cluster limits every series when one
// cluster was selected. Other label selectors are applied in Go.
type Selectors struct {
	Namespace string
	Cluster   string
}

// DurationSeconds is the window length in whole seconds.
func DurationSeconds(from, to time.Time) int {
	return int(to.Sub(from) / time.Second)
}

// CPUUsage is container CPU in core-hours, summed by namespace, pod, and node.
// rate() keeps both sides of a counter reset. increase() over the whole
// window is not used.
func CPUUsage(from, to time.Time, sel Selectors) Query {
	dur := durationLiteral(from, to)
	expr := `sum by (namespace, pod, node) (
  sum_over_time(
    rate(container_cpu_usage_seconds_total{` + containerSelector(sel) + `}[` + rateWindow + `])
    [` + dur + `:` + substep + `]
  )
) * 60 / 3600`
	return Query{Expr: expr, Time: to}
}

// MemoryUsage is container working-set memory in GiB-hours, summed like CPU.
// 1073741824 is 1024 cubed.
func MemoryUsage(from, to time.Time, sel Selectors) Query {
	dur := durationLiteral(from, to)
	expr := `sum by (namespace, pod, node) (
  sum_over_time(
    container_memory_working_set_bytes{` + containerSelector(sel) + `}[` + dur + `:` + substep + `]
  )
) * 60 / 1073741824 / 3600`
	return Query{Expr: expr, Time: to}
}

// CPUAllocatable is node allocatable CPU in core-hours. A namespace selector
// is not copied onto node series.
func CPUAllocatable(from, to time.Time, sel Selectors) Query {
	return allocatable("cpu", "3600", from, to, sel)
}

// MemoryAllocatable is node allocatable memory in GiB-hours.
func MemoryAllocatable(from, to time.Time, sel Selectors) Query {
	return allocatable("memory", "1073741824 / 3600", from, to, sel)
}

// SeriesGap is how many 60s steps are missing inside each series' own
// first-to-last span, for one container metric. A value above 1 is a hole.
// A series whose first sample is after the window start is not a hole,
// because the span starts at that sample.
func SeriesGap(metric string, from, to time.Time, sel Selectors) Query {
	dur := durationLiteral(from, to)
	selector := metric + "{" + containerSelector(sel) + "}"
	expr := `max by (namespace, pod, node) (
  (
    (max_over_time(timestamp(` + selector + `)[` + dur + `:` + substep + `]) - min_over_time(timestamp(` + selector + `)[` + dur + `:` + substep + `])) / 60
    + 1
  )
  - count_over_time(` + selector + `[` + dur + `:` + substep + `])
)`
	return Query{Expr: expr, Time: to}
}

// ClusterDiscovery lists the cluster label values on node series in the
// window. Capacity series count as well as node labels, so a store that does
// not keep kube_node_labels still gets a cluster matcher.
func ClusterDiscovery(from, to time.Time) Query {
	dur := durationLiteral(from, to)
	expr := `group by (cluster) (last_over_time({__name__=~"kube_node_labels|kube_node_status_allocatable"}[` +
		dur + `]))`
	return Query{Expr: expr, Time: to}
}

// StoreStart is the Unix time of the earliest node allocatable sample in the
// window. Node series exist for the whole life of a node, so a value well after
// the window start means the store holds no data for the start of the window.
func StoreStart(from, to time.Time, sel Selectors) Query {
	dur := durationLiteral(from, to)
	selector := joinMatchers([]string{`resource="cpu"`, labelEqual("cluster", sel.Cluster)})
	expr := `min(min_over_time(timestamp(kube_node_status_allocatable{` + selector + `})[` + dur + `:` + substep + `]))`
	return Query{Expr: expr, Time: to}
}

// LastOverTime reads the newest sample of a kube-state-metrics series in the
// window. Node metrics drop a namespace selector. Pod metrics keep it.
func LastOverTime(metric string, from, to time.Time, sel Selectors) Query {
	dur := durationLiteral(from, to)
	matcher := podSelector(sel)
	if !podScoped(metric) {
		matcher = nodeSelector(sel)
	}
	if matcher == "" {
		return Query{Expr: "last_over_time(" + metric + "[" + dur + "])", Time: to}
	}
	return Query{Expr: "last_over_time(" + metric + "{" + matcher + "}[" + dur + "])", Time: to}
}

// allocatable takes max by node at each step before integrating. Replicated
// kube-state-metrics exports one series per replica for the same node, and a
// plain sum would count that node's capacity once per replica.
func allocatable(resource, scale string, from, to time.Time, sel Selectors) Query {
	dur := durationLiteral(from, to)
	selector := joinMatchers([]string{`resource="` + resource + `"`, labelEqual("cluster", sel.Cluster)})
	expr := `sum by (node) (
  sum_over_time(
    (max by (node) (kube_node_status_allocatable{` + selector + `}))[` + dur + `:` + substep + `]
  )
) * 60 / ` + scale
	return Query{Expr: expr, Time: to}
}

func durationLiteral(from, to time.Time) string {
	return strconv.Itoa(DurationSeconds(from, to)) + "s"
}

func containerSelector(sel Selectors) string {
	return joinMatchers([]string{
		`container!=""`,
		`container!="POD"`,
		labelEqual("namespace", sel.Namespace),
		labelEqual("cluster", sel.Cluster),
	})
}

func podSelector(sel Selectors) string {
	return joinMatchers([]string{
		labelEqual("namespace", sel.Namespace),
		labelEqual("cluster", sel.Cluster),
	})
}

func nodeSelector(sel Selectors) string {
	return labelEqual("cluster", sel.Cluster)
}

func podScoped(metric string) bool {
	return !strings.HasPrefix(metric, "kube_node_")
}

func labelEqual(key, value string) string {
	if value == "" {
		return ""
	}
	return key + "=" + promQuote(value)
}

func joinMatchers(parts []string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, ",")
}

func promQuote(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(value)
	return `"` + escaped + `"`
}
