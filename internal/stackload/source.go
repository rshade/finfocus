package stackload

// Source identifies explicit files or a resolved Pulumi project to load.
type Source struct {
	// ProjectDir is the resolved Pulumi project directory.
	ProjectDir string
	// Stack selects the Pulumi stack; empty uses the project default.
	Stack string
	// StatePath selects an explicit stack export instead of a subprocess.
	StatePath string
	// PlanPath selects an explicit preview instead of a subprocess.
	PlanPath string
	// Passphrase is scoped to subprocesses; nil preserves the inherited value.
	Passphrase *string
	// SkipPreview requests only deployed state.
	SkipPreview bool
}

// Status describes whether an input was loaded, without backend details.
type Status struct {
	// Loaded reports whether the requested input was read and parsed.
	Loaded bool
	// Reason is one of the safe Reason categories, never backend error text.
	Reason string
}

// Safe reason categories for input availability.
const (
	// ReasonNone means the requested input loaded successfully.
	ReasonNone = "none"
	// ReasonNotRequested means no source was requested for this input.
	ReasonNotRequested = "not_requested"
	// ReasonBackendUnreachable means the backend could not be reached or authenticated.
	ReasonBackendUnreachable = "backend_unreachable"
	// ReasonPassphraseRequired means state decryption requires a valid passphrase.
	ReasonPassphraseRequired = "passphrase_required"
	// ReasonReadFailed means the input could not be read or parsed.
	ReasonReadFailed = "read_failed"
)
