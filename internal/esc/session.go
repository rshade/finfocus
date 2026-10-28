package esc

// Session is the resolved backend, credential, and stack organization.
// Token must never be logged or persisted by callers.
type Session struct {
	Backend string
	Token   string
	Org     string
}

// DiscoverSession exposes the shared Pulumi login and organization discovery.
func DiscoverSession(getenv func(string) string, projectBackend, stackName string) (Session, *Failure) {
	auth, fail := discover(getenv, projectBackend)
	if fail != nil {
		return Session{}, fail
	}
	org, fail := auth.orgFor(Ref{}, true, stackName, getenv)
	if fail != nil {
		return Session{}, fail
	}
	return Session{Backend: auth.BackendURL, Token: auth.Token, Org: org}, nil
}
