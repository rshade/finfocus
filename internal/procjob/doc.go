// Package procjob ties every process finfocus starts to finfocus itself on
// Windows, so none outlives it, however it ends.
//
// Unix needs no equivalent here: plugins join their own process group and
// finfocus's session (internal/pluginhost), and the caller owns the session.
package procjob
