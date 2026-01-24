package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// NewPluginInspectCmd creates the plugin inspect command.
func NewPluginInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <plugin-name> <resource-type>",
		Short: "Inspect a plugin's capabilities and field mappings",
		Long: `Inspect a plugin to discover how it maps Pulumi resource properties to pricing inputs.
This command performs a dry-run against the plugin to retrieve field mappings for a specific resource type.`,
		Example: `  # Inspect field mappings for AWS EC2 Instance
  finfocus plugin inspect aws-public aws:ec2/instance:Instance

  # Inspect specific version
  finfocus plugin inspect aws-public aws:ec2/instance:Instance --version v0.1.0

  # Output as JSON
  finfocus plugin inspect aws-public aws:ec2/instance:Instance --json`,
		Args: cobra.ExactArgs(2), //nolint:mnd // Command requires exactly 2 arguments
		RunE: runPluginInspect,
	}

	cmd.Flags().Bool("json", false, "Output in JSON format")
	cmd.Flags().String("version", "", "Specify plugin version to inspect")

	return cmd
}

func runPluginInspect(cmd *cobra.Command, args []string) error {
	pluginName := args[0]
	resourceType := args[1]
	jsonOutput, err := cmd.Flags().GetBool("json")
	if err != nil {
		return fmt.Errorf("failed to get json flag: %w", err)
	}
	version, err := cmd.Flags().GetString("version")
	if err != nil {
		return fmt.Errorf("failed to get version flag: %w", err)
	}
	ctx := cmd.Context()

	cfg := config.New()
	path, err := findPluginPath(cfg, pluginName, version)
	if err != nil {
		return fmt.Errorf("plugin not found: %w", err)
	}

	// 2. Launch plugin
	launcher := pluginhost.NewProcessLauncher()
	client, err := pluginhost.NewClient(ctx, launcher, path)
	if err != nil {
		return fmt.Errorf("failed to launch plugin: %w", err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			logging.FromContext(ctx).Debug().
				Ctx(ctx).
				Str("component", "plugin-client").
				Str("operation", "close").
				Err(closeErr).
				Msg("failed to close plugin client")
		}
	}()

	// 3. Call DryRun
	const dryRunTimeout = 10 * time.Second
	dryRunCtx, cancel := context.WithTimeout(ctx, dryRunTimeout)
	defer cancel()

	req := &pbc.DryRunRequest{
		Resource: &pbc.ResourceDescriptor{
			ResourceType: resourceType,
		},
	}

	resp, err := client.API.DryRun(dryRunCtx, req)
	if err != nil {
		if pluginhost.IsUnimplementedError(err) {
			return fmt.Errorf("plugin '%s' does not support inspection (capability discovery not implemented)",
				pluginName)
		}
		return fmt.Errorf("dry-run failed: %w", err)
	}

	// 4. Render output
	if jsonOutput {
		return renderJSON(resp, cmd.OutOrStdout())
	}
	return renderTable(resp, cmd.OutOrStdout())
}

// findPluginPath locates the plugin binary.
func findPluginPath(cfg *config.Config, name, version string) (string, error) {
	pluginDir := filepath.Join(cfg.PluginDir, name)

	if version == "" {
		v, vErr := findLatestVersion(pluginDir, name)
		if vErr != nil {
			return "", vErr
		}
		version = v
	}

	// Try standard binary names
	candidates := []string{
		"finfocus-plugin-" + name,
		"pulumicost-plugin-" + name, // Legacy
		name,                        // Fallback
	}

	for _, binName := range candidates {
		binPath := filepath.Join(pluginDir, version, binName)
		if _, statErr := os.Stat(binPath); statErr == nil {
			return binPath, nil
		}
		// Windows extension check
		if _, statErrExe := os.Stat(binPath + ".exe"); statErrExe == nil {
			return binPath + ".exe", nil
		}
	}

	return "", fmt.Errorf("plugin binary not found for %s version %s", name, version)
}

func findLatestVersion(pluginDir, name string) (string, error) {
	entries, dirErr := os.ReadDir(pluginDir)
	if dirErr != nil {
		return "", fmt.Errorf("plugin '%s' not installed (checked %s)", name, pluginDir)
	}
	var latest string
	var latestVer *semver.Version
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "v") {
			verStr := strings.TrimPrefix(e.Name(), "v")
			ver, err := semver.NewVersion(verStr)
			if err != nil {
				continue // Skip entries that fail to parse as semver
			}
			if latestVer == nil || ver.Compare(latestVer) > 0 {
				latestVer = ver
				latest = e.Name()
			}
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no valid semver versions found for plugin '%s'", name)
	}
	return latest, nil
}

func renderJSON(resp *pbc.DryRunResponse, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func renderTable(resp *pbc.DryRunResponse, w io.Writer) error {
	const (
		colWidthField  = 20
		colWidthStatus = 10
	)

	// Build format string using column widths for consistency
	rowFormat := fmt.Sprintf("%%-%ds %%-%ds %%-%ds\n", colWidthField, colWidthStatus, colWidthStatus)

	if _, err := fmt.Fprintf(w, "Field Mappings:\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, rowFormat, "FIELD", "STATUS", "CONDITION"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s %s %s\n",
		strings.Repeat("-", colWidthField),
		strings.Repeat("-", colWidthStatus),
		strings.Repeat("-", colWidthStatus)); err != nil { // CONDITION column has variable width
		return err
	}

	for _, m := range resp.GetFieldMappings() {
		status := "UNKNOWN"
		switch m.GetSupportStatus() {
		case pbc.FieldSupportStatus_FIELD_SUPPORT_STATUS_SUPPORTED:
			status = "SUPPORTED"
		case pbc.FieldSupportStatus_FIELD_SUPPORT_STATUS_UNSUPPORTED:
			status = "UNSUPPORTED"
		case pbc.FieldSupportStatus_FIELD_SUPPORT_STATUS_CONDITIONAL:
			status = "CONDITIONAL"
		case pbc.FieldSupportStatus_FIELD_SUPPORT_STATUS_DYNAMIC:
			status = "DYNAMIC"
		case pbc.FieldSupportStatus_FIELD_SUPPORT_STATUS_UNSPECIFIED:
			status = "UNKNOWN"
		}

		if _, err := fmt.Fprintf(w, rowFormat, m.GetFieldName(), status, m.GetConditionDescription()); err != nil {
			return err
		}
	}
	return nil
}
