package pluginhost

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc"
)

const (
	// stderrTailBytes is how much recent plugin stderr is kept for an early-exit report.
	stderrTailBytes = 4096
	// maxStderrExcerptLines and maxStderrExcerptBytes bound the excerpt placed
	// in the launch error, which reaches terminals, `plugin list`, and the web UI.
	maxStderrExcerptLines = 3
	maxStderrExcerptBytes = 300
	// minMaskedSecretLen keeps short values such as "true" from being masked
	// throughout the excerpt.
	minMaskedSecretLen = 6

	redactedText     = "[redacted]"
	excerptSeparator = " | "
	excerptEllipsis  = "..."
)

var (
	urlUserinfo = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^/\s@]+@`)
	ansiEscape  = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
)

// pluginProcess owns the single cmd.Wait call of a launched plugin, so the bind
// wait can notice an early exit while the cleanup paths still join the process.
type pluginProcess struct {
	cmd     *exec.Cmd
	exited  chan struct{}
	waitErr error
}

func watchProcess(cmd *exec.Cmd) *pluginProcess {
	proc := &pluginProcess{cmd: cmd, exited: make(chan struct{})}
	go func() {
		proc.waitErr = cmd.Wait()
		close(proc.exited)
	}()
	return proc
}

// stop kills the plugin's process group and waits for the process. The group
// is signaled even after the main process exited, because a child it left in
// the group can still hold the plugin's port or stdio.
func (pp *pluginProcess) stop() {
	if pp == nil || pp.cmd == nil || pp.cmd.Process == nil {
		return
	}
	_ = killProcessGroup(pp.cmd)
	<-pp.exited
	closeLogTap(pp.cmd)
}

// exitStatus describes how the process ended. Call it only after exited is closed.
func (pp *pluginProcess) exitStatus() string {
	if pp.cmd.ProcessState != nil {
		return pp.cmd.ProcessState.String()
	}
	if pp.waitErr != nil {
		return pp.waitErr.Error()
	}
	return "exited"
}

// pluginExitError reports a plugin process that exited before it bound its port.
type pluginExitError struct {
	status string
	stderr string
	// portCollision is set when the plugin's own stderr reports that its port
	// was taken, the one early exit that a new port can fix.
	portCollision bool
}

func (e *pluginExitError) Error() string {
	msg := "plugin exited before binding to port (" + e.status + ")"
	if e.stderr == "" {
		return msg + " with no error output"
	}
	return msg + ": " + e.stderr
}

func newPluginExitError(proc *pluginProcess, tail *stderrTail, secrets []string) *pluginExitError {
	data, truncated := tail.snapshot()
	return &pluginExitError{
		status:        proc.exitStatus(),
		stderr:        stderrExcerpt(data, truncated, secrets),
		portCollision: bytes.Contains(data, []byte("address already in use")),
	}
}

// stderrTail keeps the last stderrTailBytes of plugin stderr. Writes never fail,
// so it cannot stop exec's stderr copy and block the plugin on a full pipe.
type stderrTail struct {
	mu        sync.Mutex
	buf       []byte
	truncated bool
}

func (t *stderrTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - stderrTailBytes; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
		t.truncated = true
	}
	return len(p), nil
}

func (t *stderrTail) snapshot() ([]byte, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]byte(nil), t.buf...), t.truncated
}

// stderrExcerpt turns the end of a plugin's stderr into a short single-line
// reason for a launch error. That error reaches places the log file does not
// (terminal, `plugin list` Notes, the dashboard), so the excerpt is reduced
// before it leaves: a structured log line contributes only its message and
// error fields, values of credential-named variables in the plugin's
// environment are masked, URL userinfo and control characters are removed,
// and the result is cut to maxStderrExcerptBytes.
func stderrExcerpt(data []byte, truncated bool, secrets []string) string {
	text := maskSecrets(string(data), secrets)
	lines := strings.Split(text, "\n")
	if truncated && len(lines) > 0 {
		lines = lines[1:]
	}
	var rendered []string
	for _, line := range lines {
		if out := renderStderrLine(line); out != "" {
			rendered = append(rendered, out)
		}
	}
	if len(rendered) > maxStderrExcerptLines {
		rendered = rendered[len(rendered)-maxStderrExcerptLines:]
	}
	excerpt := maskSecrets(strings.Join(rendered, excerptSeparator), secrets)
	excerpt = urlUserinfo.ReplaceAllString(excerpt, "${1}"+redactedText+"@")
	return keepEnd(excerpt, maxStderrExcerptBytes)
}

func renderStderrLine(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "{") {
		var fields map[string]any
		if json.Unmarshal([]byte(line), &fields) == nil {
			line = joinLogFields(fields)
		}
	}
	return strings.TrimSpace(stripControl(line))
}

func joinLogFields(fields map[string]any) string {
	message, _ := fields["message"].(string)
	errText, _ := fields["error"].(string)
	switch {
	case message != "" && errText != "":
		return message + ": " + errText
	case message != "":
		return message
	default:
		return errText
	}
}

func stripControl(s string) string {
	s = ansiEscape.ReplaceAllString(s, "")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func maskSecrets(s string, secrets []string) string {
	for _, secret := range secrets {
		if len(secret) >= minMaskedSecretLen {
			s = strings.ReplaceAll(s, secret, redactedText)
		}
	}
	return s
}

// keepEnd cuts s to at most limit bytes, keeping the end, where the reason for
// an exit usually is, and never splitting a UTF-8 sequence.
func keepEnd(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	start := len(s) - (limit - len(excerptEllipsis))
	for start < len(s) && !utf8.RuneStart(s[start]) {
		start++
	}
	return excerptEllipsis + s[start:]
}

// credentialValues returns the values of credential-named variables in a
// plugin environment, plus every variable delivered from ESC. Those are the
// secrets core knows the plugin holds and could echo in an error.
// Longer values come first so a secret that contains another is masked whole.
func credentialValues(environ []string, fromESC map[string]esc.Ref) []string {
	var values []string
	for _, entry := range environ {
		key, value, found := strings.Cut(entry, "=")
		if !found || value == "" {
			continue
		}
		if _, isESC := fromESC[key]; isESC || config.IsCredentialName(key) {
			values = append(values, value)
		}
	}
	slices.SortFunc(values, func(a, b string) int { return len(b) - len(a) })
	return values
}
