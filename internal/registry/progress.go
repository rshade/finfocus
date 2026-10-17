package registry

import "strings"

// ProgressPhase names one step of an install, update, or remove.
type ProgressPhase string

const (
	// PhaseResolve is release lookup.
	PhaseResolve ProgressPhase = "resolve"
	// PhaseDownload is archive transfer.
	PhaseDownload ProgressPhase = "download"
	// PhaseVerify is checksum verification.
	PhaseVerify ProgressPhase = "verify"
	// PhaseExtract is archive extraction or version removal.
	PhaseExtract ProgressPhase = "extract"
	// PhaseValidate is the binary check after extraction.
	PhaseValidate ProgressPhase = "validate"
	// PhaseDone is the terminal success sentence.
	PhaseDone ProgressPhase = "done"
)

// ProgressEvent is one progress step. Message is the same sentence the string
// callback receives, except a download byte event, which carries Bytes and Total
// alongside the percent sentence.
type ProgressEvent struct {
	Phase   ProgressPhase
	Message string
	Bytes   int64
	Total   int64
	Warning bool
}

// progressReport delivers a sentence to the string callback and a classified
// event to OnEvent. Download percent lines use text and event separately so
// the byte count is not lost and the event is not emitted twice.
type progressReport struct {
	text  func(string)
	event func(ProgressEvent)
}

func newProgressReport(text func(string), event func(ProgressEvent)) progressReport {
	return progressReport{text: text, event: event}
}

func (p progressReport) msg(message string) {
	if p.text != nil {
		p.text(message)
	}
	if p.event != nil {
		p.event(classifyProgress(message))
	}
}

func classifyProgress(message string) ProgressEvent {
	event := ProgressEvent{Message: message}
	body := message
	if strings.HasPrefix(message, "Warning:") {
		event.Warning = true
		body = strings.TrimSpace(strings.TrimPrefix(message, "Warning:"))
	}
	event.Phase = phaseOf(body)
	return event
}

func phaseOf(message string) ProgressPhase {
	switch {
	case strings.HasPrefix(message, "Successfully"):
		return PhaseDone
	case strings.HasPrefix(message, "Fetching"),
		strings.HasPrefix(message, "Checking"),
		strings.HasPrefix(message, "Would update"):
		return PhaseResolve
	case strings.HasPrefix(message, "Downloading"):
		return PhaseDownload
	case strings.HasPrefix(message, "Checksum"), strings.Contains(strings.ToLower(message), "checksum"):
		return PhaseVerify
	case strings.HasPrefix(message, "Extracting"), strings.HasPrefix(message, "Removing"):
		return PhaseExtract
	case strings.HasPrefix(message, "Validating"), strings.Contains(message, "no executable"):
		return PhaseValidate
	default:
		return PhaseResolve
	}
}
