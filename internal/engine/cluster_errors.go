package engine

import "fmt"

// ClusterInputError marks a cluster allocation failure the caller can fix by
// changing the request: a flag or query value, a policy, or a plugin choice.
// The CLI exits 2 on it and the dashboard answers 400. The message is the
// wrapped error's, so both surfaces show the same text.
type ClusterInputError struct {
	Err error
}

func (e *ClusterInputError) Error() string { return e.Err.Error() }

func (e *ClusterInputError) Unwrap() error { return e.Err }

// ClusterPluginMissingError reports that no installed plugin declares a
// capability a cluster allocation needs. The CLI exits 1 on it; the dashboard
// answers that allocation is unavailable and names the capability.
type ClusterPluginMissingError struct {
	Capability string
}

func (e *ClusterPluginMissingError) Error() string {
	return fmt.Sprintf("no installed plugin provides %s; install one with: finfocus plugin install kubernetes",
		e.Capability)
}
