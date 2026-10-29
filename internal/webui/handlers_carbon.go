package webui

import (
	"errors"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus/internal/carbonreport"
)

func (s *Server) registerCarbon(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/carbon/actual/query", s.handleCarbonQuery)
}

func (s *Server) handleCarbonQuery(w http.ResponseWriter, r *http.Request) {
	var selectors carbonreport.Selectors
	if !s.decodeJSON(w, r, &selectors) {
		return
	}
	req, err := selectors.Request()
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "carbon_invalid_query",
			"The historical carbon query is invalid. Use complete UTC months and a supported method and scope.")
		return
	}
	if s.session == nil || s.session.opts.HistoricalCarbon == nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "historical carbon reporting unavailable")
		return
	}
	ctx, done, err := s.session.beginQuery(r.Context())
	if err != nil {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "historical carbon session unavailable")
		return
	}
	defer done()
	report, err := s.session.opts.HistoricalCarbon(ctx, req)
	if err != nil || report == nil {
		httpStatus, code, message := carbonQueryError(err)
		s.writeError(w, httpStatus, code, message)
		return
	}
	s.WriteJSON(w, http.StatusOK, report)
}

func carbonQueryError(err error) (int, string, string) {
	if errors.Is(err, carbonreport.ErrInvalidResponse) {
		return http.StatusBadGateway, "carbon_invalid_data", "The historical carbon provider returned invalid emissions data."
	}
	// FromError follows wrapped gRPC statuses without publishing their diagnostics.
	//nolint:exhaustive // Unrecognized statuses use the fixed internal fallback.
	switch status.Code(err) {
	case codes.PermissionDenied:
		return http.StatusForbidden, "carbon_permission_denied", "AWS denied historical carbon access. Grant the AWS Sustainability read permission."
	case codes.Unauthenticated:
		return http.StatusUnauthorized, "carbon_authentication_required", "AWS authentication failed for historical carbon reporting. Check the provider credentials."
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests, "carbon_throttled", "AWS historical carbon reporting is throttled. Try again later."
	case codes.FailedPrecondition, codes.Unimplemented:
		return http.StatusServiceUnavailable, "carbon_disabled", "Historical carbon reporting is disabled or unsupported. Install aws-ce and enable FINFOCUS_AWS_CARBON_ENABLED=true or plugins.aws-ce.carbon_enabled=true, then restart."
	case codes.Internal, codes.DataLoss:
		return http.StatusBadGateway, "carbon_provider_error", "The historical carbon provider could not produce a valid report. Try again later."
	case codes.Unavailable:
		return http.StatusBadGateway, "carbon_provider_unavailable", "The historical carbon provider is unavailable. Try again later."
	case codes.InvalidArgument:
		return http.StatusBadRequest, "carbon_invalid_query", "The historical carbon provider rejected the query. Check the selected dates and filters."
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout, "carbon_timeout", "The historical carbon query timed out. Try again later."
	default:
		return http.StatusInternalServerError, "carbon_internal_error", "Historical carbon reporting failed. Try again later."
	}
}
