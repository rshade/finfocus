package cloudhistory

import "fmt"

// RequiredError explains why a required import cannot produce a complete report.
type RequiredError struct {
	Reason     string
	Fix        string
	Validation bool
}

// Error provides the unavailable reason and an actionable remedy.
func (e *RequiredError) Error() string {
	return fmt.Sprintf("Pulumi Cloud history unavailable (%s): %s", e.Reason, e.Fix)
}

func requiredFailure(reason string) *RequiredError {
	e := &RequiredError{Reason: reason}
	switch reason {
	case "non_cloud_backend":
		e.Fix = "Cloud history requires a Pulumi Cloud backend"
		e.Validation = true
	case "insecure_backend":
		e.Fix = "Pulumi Cloud backend must use HTTPS"
	case "history_disabled":
		e.Fix = "enable cost.history.enabled before using cost.history.pulumi_cloud.enabled"
		e.Validation = true
	case historyLockedReason:
		e.Fix = "history.db is locked; stop the other FinFocus process and retry"
	case "not_logged_in", "token_rejected":
		e.Fix = "run pulumi login or set PULUMI_BACKEND_URL and PULUMI_ACCESS_TOKEN for the intended backend"
	case "no_permission":
		e.Fix = "use a Pulumi token with read access to this stack"
	case "unknown_stack":
		e.Fix = "select a Pulumi stack and organization, or pass --stack"
	case "unreadable_updates":
		e.Fix = "one or more updates could not be read or stored; check access and local history storage, then retry"
	default:
		e.Fix = "check the Pulumi Cloud backend connection and local history storage, then retry"
	}
	return e
}
