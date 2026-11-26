package pluginhost

import "slices"

// MatchKind says which of a plugin's names a user-supplied name matched.
// Lower values take precedence when two plugins match the same name.
type MatchKind int

const (
	// MatchNone means the name matched no plugin.
	MatchNone MatchKind = iota
	// MatchInstall means the name is the plugin's install name.
	MatchInstall
	// MatchAlias means the name is a former name from the plugin's catalog entry.
	MatchAlias
	// MatchReported means the name is what the plugin reports from its Name RPC.
	MatchReported
)

// String returns the match kind as used in logs and messages.
func (k MatchKind) String() string {
	switch k {
	case MatchInstall:
		return "install"
	case MatchAlias:
		return "alias"
	case MatchReported:
		return "reported"
	case MatchNone:
		return "none"
	default:
		return "unknown"
	}
}

// matchKind returns the highest-precedence name of c that equals name.
func (c *Client) matchKind(name string) MatchKind {
	switch {
	case c == nil || name == "":
		return MatchNone
	case c.InstallName != "" && name == c.InstallName:
		return MatchInstall
	case slices.Contains(c.Aliases, name):
		return MatchAlias
	case name == c.Name:
		return MatchReported
	default:
		return MatchNone
	}
}

// MatchesName reports whether a user-supplied plugin name (a routing entry,
// --adapter, a pricing-mode plugin) refers to this plugin: its install name,
// one of its catalog aliases, or its reported name. An empty name matches
// nothing. Every comparison of a user-supplied name with a plugin goes
// through this rule or ResolveName.
func (c *Client) MatchesName(name string) bool {
	return c.matchKind(name) != MatchNone
}

// ResolveName returns the plugin a user-supplied name refers to and how it
// matched. Install names take precedence over aliases, and aliases over
// reported names; within one kind the first client wins. ambiguous is true
// when a different client also matched the name through a lower-precedence
// kind, so callers can warn that the name is read two ways.
func ResolveName(clients []*Client, name string) (*Client, MatchKind, bool) {
	var first [MatchReported + 1]*Client
	for _, c := range clients {
		kind := c.matchKind(name)
		if kind != MatchNone && first[kind] == nil {
			first[kind] = c
		}
	}

	for kind := MatchInstall; kind <= MatchReported; kind++ {
		chosen := first[kind]
		if chosen == nil {
			continue
		}
		for lower := kind + 1; lower <= MatchReported; lower++ {
			if first[lower] != nil && first[lower] != chosen {
				return chosen, kind, true
			}
		}
		return chosen, kind, false
	}
	return nil, MatchNone, false
}
