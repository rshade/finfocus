package webui

import (
	"context"
	"errors"
	"net/http"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

const (
	clusterStatusOK          = "ok"
	clusterStatusUnavailable = "unavailable"
)

// ClusterQuery is the dashboard's `cost cluster` request. Each field takes the
// syntax of the flag of the same name, and an omitted one takes the CLI
// default. Policy is allocation policy HuJSON text: the server never opens a
// path the page names. With no text, the discovered policy applies.
type ClusterQuery struct {
	Context     string   `json:"context,omitempty"`
	Namespace   string   `json:"namespace,omitempty"`
	Selector    []string `json:"selector,omitempty"`
	GroupBy     string   `json:"groupBy,omitempty"`
	From        string   `json:"from,omitempty"`
	To          string   `json:"to,omitempty"`
	UsageSource string   `json:"usageSource,omitempty"`
	Allocator   string   `json:"allocator,omitempty"`
	Policy      string   `json:"policy,omitempty"`
}

// ClusterPolicyQuery is the dashboard's `cost cluster --show-policy` request.
type ClusterPolicyQuery struct {
	Allocator string `json:"allocator,omitempty"`
	Policy    string `json:"policy,omitempty"`
}

// ClusterResponse is the `cost cluster --output json` document plus display
// strings, so the page computes and formats nothing itself.
type ClusterResponse struct {
	engine.ClusterReport

	Status  string                         `json:"status"`
	Display viewmodel.ClusterReportDisplay `json:"display"`
}

// ClusterPolicyResponse is the `--show-policy` document plus display strings.
type ClusterPolicyResponse struct {
	engine.ClusterPolicy

	Status  string               `json:"status"`
	Display ClusterPolicyDisplay `json:"display"`
}

// ClusterPolicyDisplay is the policy source and its short digest.
type ClusterPolicyDisplay struct {
	Source string `json:"source"`
	Digest string `json:"digest"`
}

// ClusterUnavailable answers a query no installed plugin can serve. It is a
// status the page explains, not a server failure.
type ClusterUnavailable struct {
	Status     string `json:"status"`
	Code       string `json:"code"`
	Capability string `json:"capability"`
	Error      string `json:"error"`
}

func (s *Server) registerCluster(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/cluster/query", s.handleClusterQuery)
	mux.HandleFunc("POST /api/cluster/policy", s.handleClusterPolicy)
}

func (s *Server) handleClusterQuery(w http.ResponseWriter, r *http.Request) {
	var query ClusterQuery
	if !s.decodeJSON(w, r, &query) {
		return
	}
	var call func(context.Context, ClusterQuery) (engine.ClusterReport, error)
	if s.session != nil {
		call = s.session.opts.Cluster
	}
	runClusterCall(s, w, r, "cluster_query", call, query, func(report engine.ClusterReport) any {
		return ClusterResponse{
			Status: clusterStatusOK, ClusterReport: report, Display: viewmodel.BuildClusterReportDisplay(report),
		}
	})
}

func (s *Server) handleClusterPolicy(w http.ResponseWriter, r *http.Request) {
	var query ClusterPolicyQuery
	if !s.decodeJSON(w, r, &query) {
		return
	}
	var call func(context.Context, ClusterPolicyQuery) (engine.ClusterPolicy, error)
	if s.session != nil {
		call = s.session.opts.ClusterPolicy
	}
	runClusterCall(s, w, r, "cluster_policy", call, query, func(policy engine.ClusterPolicy) any {
		return ClusterPolicyResponse{
			Status: clusterStatusOK, ClusterPolicy: policy,
			Display: ClusterPolicyDisplay{Source: policy.Source, Digest: viewmodel.ShortDigest(policy.Digest)},
		}
	})
}

// runClusterCall runs a cluster callback in a session query and writes its
// answer. The query has no deadline of its own: every cluster plugin call
// carries one, sized to the request as on the CLI, and the request and the
// session still cancel it.
func runClusterCall[Q, R any](
	s *Server, w http.ResponseWriter, r *http.Request, operation string,
	call func(context.Context, Q) (R, error), query Q, respond func(R) any,
) {
	if call == nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "cluster data is loading")
		return
	}
	ctx, done, err := s.session.beginQueryWithin(r.Context(), noQueryDeadline)
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "cluster data is loading")
		return
	}
	defer done()
	result, err := call(ctx, query)
	if err != nil {
		s.writeClusterError(w, operation, err)
		return
	}
	s.WriteJSON(w, http.StatusOK, respond(result))
}

// writeClusterError maps a cluster failure: a missing plugin capability is an
// "unavailable" status, the caller's input is 400 with the CLI's message, and
// anything else is a generic 502 whose cause stays in the log.
func (s *Server) writeClusterError(w http.ResponseWriter, operation string, err error) {
	if missing, ok := errors.AsType[*engine.ClusterPluginMissingError](err); ok {
		s.WriteJSON(w, http.StatusOK, ClusterUnavailable{
			Status: clusterStatusUnavailable, Code: "cluster_plugin_missing",
			Capability: missing.Capability, Error: missing.Error(),
		})
		return
	}
	if input, ok := errors.AsType[*engine.ClusterInputError](err); ok {
		s.writeError(w, http.StatusBadRequest, "invalid_cluster_query", input.Error())
		return
	}
	s.logger.Warn().Err(err).Str("component", "webui").Str("operation", operation).Msg("cluster query failed")
	s.writeError(w, http.StatusBadGateway, "cluster_unavailable", "cluster data unavailable")
}
