package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rshade/finfocus/internal/tui"
)

// PromptResult contains the result of a user prompt interaction.
type PromptResult struct {
	// Accepted is true if the user accepted the prompt (typed "y" or "Y")
	Accepted bool
	// TimedOut is true if the prompt timed out waiting for input (reserved for future use)
	TimedOut bool
	// Cancelled is true if the user explicitly cancelled (e.g., Ctrl+C)
	Cancelled bool
}

// ConfirmFallback prompts the user to confirm fallback to an alternative version.
// It returns immediately with Accepted=false in non-interactive (non-TTY) environments.
//
// Parameters:
//   - writer: where to write the prompt message (typically cmd.OutOrStdout())
//   - reader: where to read user input from (typically os.Stdin)
//   - pluginName: name of the plugin being installed
//   - requestedVersion: the version originally requested
//   - fallbackVersion: the version being offered as fallback
//   - platform: the platform string (e.g., "linux/amd64")
//
// The prompt defaults to "No" (abort) when the user presses Enter without input.
// Valid inputs: "y", "Y", "yes", "Yes", "YES" for acceptance; anything else declines.
func ConfirmFallback(
	writer io.Writer,
	reader io.Reader,
	pluginName string,
	requestedVersion string,
	fallbackVersion string,
	platform string,
) PromptResult {
	// In non-TTY environments, return immediately without prompting
	if !tui.IsTTY() {
		return PromptResult{Accepted: false}
	}

	// Display warning and prompt
	fmt.Fprintf(writer, "\nWarning: No compatible assets found for %s@%s (%s).\n",
		pluginName, requestedVersion, platform)
	fmt.Fprintf(writer, "? Would you like to install the latest stable version (%s) instead? [y/N] ",
		fallbackVersion)

	// Read user input
	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		// EOF or error - treat as cancelled
		if scanner.Err() != nil {
			return PromptResult{Cancelled: true}
		}
		// EOF without error - treat as decline (user pressed Ctrl+D)
		return PromptResult{Accepted: false}
	}

	input := strings.TrimSpace(scanner.Text())

	// Empty input defaults to "No" (abort)
	if input == "" {
		return PromptResult{Accepted: false}
	}

	// Check for acceptance
	switch strings.ToLower(input) {
	case "y", "yes":
		return PromptResult{Accepted: true}
	default:
		return PromptResult{Accepted: false}
	}
}

// ConfirmFallbackWithStdin is a convenience wrapper that uses os.Stdin as the reader.
func ConfirmFallbackWithStdin(
	writer io.Writer,
	pluginName string,
	requestedVersion string,
	fallbackVersion string,
	platform string,
) PromptResult {
	return ConfirmFallback(writer, os.Stdin, pluginName, requestedVersion, fallbackVersion, platform)
}
