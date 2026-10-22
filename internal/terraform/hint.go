package terraform

import (
	"errors"
	"fmt"
	"strings"
)

// HintKind classifies a failed OpenTofu plan.
type HintKind string

const (
	// HintBinaryMissing means tofu is not on PATH.
	HintBinaryMissing HintKind = "binary"
	// HintNotInitialized means the directory has not been initialized.
	HintNotInitialized HintKind = "init"
	// HintCredentials means OpenTofu could not read credentials or the backend.
	HintCredentials HintKind = "credentials"
	// HintPlanFailed means the plan failed for another reason.
	HintPlanFailed HintKind = "failed"
)

const (
	hintBinary = "OpenTofu plan needs the tofu command on PATH. " +
		"Install OpenTofu, then run this command again."
	hintInit = "This directory is not initialized. Run tofu init here, " +
		"then run this command again. FinFocus does not run init."
	hintCredentials = "OpenTofu could not read credentials or the backend. " +
		"Fix the access OpenTofu uses for this directory and run this command again. " +
		"FinFocus does not log in or store credentials."
	hintFailed = "OpenTofu plan failed."
)

// HintError is an operational plan failure. It is not a user-input error.
type HintError struct {
	Kind HintKind
	Text string
	Err  error
}

func (e *HintError) Error() string {
	if e == nil {
		return hintFailed
	}
	return e.Text
}

func (e *HintError) Unwrap() error { return e.Err }

func binaryMissing() error {
	return &HintError{Kind: HintBinaryMissing, Text: hintBinary}
}

func classifyStderr(stderr string) error {
	kind, text := classifyHint(stderr)
	return &HintError{Kind: kind, Text: text, Err: errors.New(text)}
}

func classifyHint(stderr string) (HintKind, string) {
	if asksForInit(stderr) {
		return HintNotInitialized, hintInit
	}
	if hasCredentialPhrase(stderr) {
		return HintCredentials, hintCredentials
	}
	return HintPlanFailed, hintFailed + suffixLines(stderr)
}

func asksForInit(stderr string) bool {
	for _, phrase := range []string{
		"terraform init",
		"tofu init",
		"initialization required",
		"Plugin reinitialization required",
	} {
		if strings.Contains(stderr, phrase) {
			return true
		}
	}
	return false
}

func hasCredentialPhrase(stderr string) bool {
	for _, phrase := range credentialPhraseList() {
		if strings.Contains(stderr, phrase) {
			return true
		}
	}
	return false
}

func credentialPhraseList() []string {
	return []string{
		"No valid credential sources found",
		"failed to refresh cached credentials",
		"ExpiredToken",
		"InvalidClientTokenId",
		"Error building ARM Config",
		"az login",
		"unable to build authorizer",
		"could not find default credentials",
		"google: could not find default credentials",
		"error configuring the backend",
		"Error loading state",
		"Failed to get existing workspaces",
		"Error inspecting states",
	}
}

func suffixLines(stderr string) string {
	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return ""
	}
	const stderrTailLines = 20
	if len(lines) > stderrTailLines {
		lines = lines[len(lines)-stderrTailLines:]
	}
	return "\n" + strings.Join(lines, "\n")
}

func oldBinaryHint() error {
	return &HintError{
		Kind: HintPlanFailed,
		Text: fmt.Sprintf("%s OpenTofu %s or newer is required.", hintFailed, minTofu),
	}
}

// PlanFailure wraps err as the plan-failed hint. The hint names a 32 MiB
// limit when the plan document is too large.
func PlanFailure(err error) error {
	text := hintFailed
	if err != nil && strings.Contains(err.Error(), "32 MiB") {
		text = hintFailed + " The plan input exceeds 32 MiB."
	}
	return &HintError{Kind: HintPlanFailed, Text: text, Err: err}
}
