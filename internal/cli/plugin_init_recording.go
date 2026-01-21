package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	sessionIDBytes          = 16
	recordingCommandTimeout = 2 * time.Minute
)

type RecordingSession struct {
	ID          string            `json:"id"`
	OutputDir   string            `json:"output_dir"`
	FixtureDir  string            `json:"fixture_dir"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
	Status      string            `json:"status"`
	Errors      []string          `json:"errors,omitempty"`
	Recorded    []RecordedRequest `json:"recorded,omitempty"`
}

type RecordedRequest struct {
	Type      string    `json:"type"`
	Path      string    `json:"path"`
	Timestamp time.Time `json:"timestamp"`
}

type RecorderWorkflow struct {
	logger  zerolog.Logger
	session *RecordingSession
}

// NewRecorderWorkflow creates a RecorderWorkflow configured with the provided logger and output directory.
// It initializes an internal RecordingSession with a generated session ID, the given output directory,
// the current start time, and a status of "initialized".
// logger is used for workflow logging.
// outputDir is the base directory where recorded request files and session data will be placed.
// It returns a pointer to the constructed RecorderWorkflow.
func NewRecorderWorkflow(logger zerolog.Logger, outputDir string) *RecorderWorkflow {
	return &RecorderWorkflow{
		logger: logger,
		session: &RecordingSession{
			ID:        generateSessionID(),
			OutputDir: outputDir,
			StartedAt: time.Now(),
			Status:    "initialized",
		},
	}
}

// generateSessionID returns a pseudo-unique session identifier.
// It generates a cryptographically random 16-byte value and encodes it as a hexadecimal string.
// If secure random generation fails, it falls back to a timestamp-based numeric ID.
func generateSessionID() string {
	b := make([]byte, sessionIDBytes)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(b)
}

func (w *RecorderWorkflow) PrepareOutputDirectory() error {
	testdataDir := filepath.Join(w.session.OutputDir, "testdata", "recorded_requests")
	if err := os.MkdirAll(testdataDir, 0700); err != nil {
		return fmt.Errorf("creating recording output directory: %w", err)
	}
	w.session.OutputDir = testdataDir
	w.logger.Info().Str("dir", testdataDir).Msg("prepared recording output directory")
	return nil
}

func (w *RecorderWorkflow) SetFixtureDirectory(path string) {
	w.session.FixtureDir = path
}

func (w *RecorderWorkflow) RunWithRecorder(ctx context.Context, planPath, statePath string, recordFixtures bool) error {
	if !recordFixtures {
		w.logger.Debug().Msg("fixture recording disabled")
		return nil
	}

	w.session.Status = "recording"
	w.logger.Info().Msg("starting recorded workflow")

	env := append(os.Environ(),
		fmt.Sprintf("FINFOCUS_RECORDER_OUTPUT_DIR=%s", w.session.OutputDir),
		"FINFOCUS_RECORDER_MOCK_RESPONSE=true",
	)

	recordedTypes := []struct {
		name string
		cmd  string
		args []string
	}{
		{"GetProjectedCost", "finfocus", []string{"cost", "projected", "--pulumi-json", planPath}},
		// Fixed date "2025-01-01" ensures deterministic fixture generation for tests
		{"GetActualCost", "finfocus", []string{"cost", "actual", "--pulumi-state", statePath, "--from", "2025-01-01"}},
		{"GetRecommendations", "finfocus", []string{"cost", "recommendations", "--pulumi-json", planPath}},
	}

	for _, rt := range recordedTypes {
		// Check for context cancellation before processing next recording
		if err := ctx.Err(); err != nil {
			return err
		}

		w.logger.Info().Str("type", rt.name).Msg("recording request type")

		timeoutCtx, cancel := context.WithTimeout(ctx, recordingCommandTimeout)

		//nolint:gosec // Command and arguments are hardcoded, not from user input
		cmd := exec.CommandContext(timeoutCtx, rt.cmd, rt.args...)
		cmd.Env = env
		cmd.Dir = w.session.OutputDir

		output, recErr := cmd.CombinedOutput()
		cancel() // Cancel immediately after command completes to avoid accumulating timers

		if recErr != nil {
			errMsg := fmt.Sprintf("recording %s: %v - %s", rt.name, recErr, string(output))
			w.session.Errors = append(w.session.Errors, errMsg)
			w.logger.Warn().Str("type", rt.name).Err(recErr).Msg("recording failed")
			continue
		}

		reqPath := w.findRecordedRequest(rt.name)
		if reqPath != "" {
			w.session.Recorded = append(w.session.Recorded, RecordedRequest{
				Type:      rt.name,
				Path:      reqPath,
				Timestamp: time.Now(),
			})
			w.logger.Info().Str("type", rt.name).Str("path", reqPath).Msg("recorded request captured")
		}
	}

	w.session.Status = "completed"
	w.session.CompletedAt = time.Now()
	w.logger.Info().
		Int("recorded_count", len(w.session.Recorded)).
		Int("error_count", len(w.session.Errors)).
		Dur("duration", w.session.CompletedAt.Sub(w.session.StartedAt)).
		Msg("recording workflow completed")

	return nil
}

func (w *RecorderWorkflow) findRecordedRequest(requestType string) string {
	entries, err := os.ReadDir(w.session.OutputDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, requestType) && strings.HasSuffix(name, ".json") {
			return filepath.Join(w.session.OutputDir, name)
		}
	}
	return ""
}

func (w *RecorderWorkflow) Session() *RecordingSession {
	return w.session
}

func (w *RecorderWorkflow) CopyRecordedRequestsToTestdata(targetDir string) error {
	testdataDir := filepath.Join(targetDir, "testdata", "recorded_requests")
	if err := os.MkdirAll(testdataDir, 0700); err != nil {
		return fmt.Errorf("creating testdata directory: %w", err)
	}

	for _, req := range w.session.Recorded {
		content, err := os.ReadFile(req.Path)
		if err != nil {
			return fmt.Errorf("reading recorded request %s: %w", req.Path, err)
		}

		targetPath := filepath.Join(testdataDir, filepath.Base(req.Path))
		if writeErr := os.WriteFile(targetPath, content, 0600); writeErr != nil {
			return fmt.Errorf("writing recorded request %s: %w", targetPath, writeErr)
		}
		w.logger.Debug().Str("from", req.Path).Str("to", targetPath).Msg("copied recorded request")
	}

	w.logger.Info().
		Int("count", len(w.session.Recorded)).
		Str("dir", testdataDir).
		Msg("copied recorded requests to testdata")

	return nil
}

func (w *RecorderWorkflow) ValidateRecordings() error {
	if len(w.session.Recorded) == 0 {
		if len(w.session.Errors) > 0 {
			return fmt.Errorf("recording failed with errors: %s", strings.Join(w.session.Errors, "; "))
		}
		return errors.New("no requests were recorded")
	}

	expectedTypes := map[string]struct{}{
		"GetProjectedCost":   {},
		"GetActualCost":      {},
		"GetRecommendations": {},
	}

	for _, req := range w.session.Recorded {
		delete(expectedTypes, req.Type)
	}

	if len(expectedTypes) > 0 {
		missing := make([]string, 0, len(expectedTypes))
		for t := range expectedTypes {
			missing = append(missing, t)
		}
		return fmt.Errorf("missing recordings for: %s", strings.Join(missing, ", "))
	}

	return nil
}
