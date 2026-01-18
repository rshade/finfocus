package pluginhost

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/proto"
	"google.golang.org/grpc"
)

// ErrPluginIncompatible is returned when a plugin's spec version is incompatible
// with the core spec version and strict compatibility mode is enabled.
var ErrPluginIncompatible = errors.New("plugin spec version incompatible with core")

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

// SkipVersionCheckKey is the context key for skipping version validation.
const SkipVersionCheckKey contextKey = "skip_version_check"

// Client wraps a gRPC connection to a plugin and provides the cost source API.
type Client struct {
	Name     string
	Metadata *proto.PluginMetadata
	Conn     *grpc.ClientConn
	API      proto.CostSourceClient
	Close    func() error
}

// Launcher is an interface for different plugin launching strategies (TCP or stdio).
type Launcher interface {
	Start(ctx context.Context, path string, args ...string) (*grpc.ClientConn, func() error, error)
}

// NewClient creates and initializes a plugin Client by launching the plugin binary and establishing a gRPC connection.
//
// NewClient starts the plugin using the provided Launcher, obtains the plugin's legacy name, and prepares a CostSource gRPC client.
// It attempts to fetch plugin metadata with a 5 second timeout and stores that metadata on the returned Client when available.
// If the plugin does not implement GetPluginInfo or the info call fails, NewClient returns the Client with Metadata left nil and a nil error.
// After obtaining plugin info, NewClient validates spec version compatibility; in strict compatibility mode a mismatch causes the plugin to be closed and an error to be returned.
// If obtaining the legacy name fails, NewClient ensures the plugin process is closed before returning an error.
//
// Parameters:
//  - ctx: context for launching and RPC calls; may carry SkipVersionCheckKey to skip version validation.
//  - launcher: launcher used to start the plugin process and produce a gRPC connection and close function.
//  - binPath: filesystem path to the plugin binary to start.
//
// Returns:
//  - *Client: initialized client with Name, Conn, API, Close, and optionally Metadata (nil if GetPluginInfo failed or was unimplemented).
//  - error: any error encountered while launching the plugin, retrieving its name, or enforcing strict version compatibility. Closing errors are wrapped when the client close fails while handling an error.
func NewClient(ctx context.Context, launcher Launcher, binPath string) (*Client, error) {
	conn, closeFn, err := launcher.Start(ctx, binPath)
	if err != nil {
		return nil, err
	}

	api := proto.NewCostSourceClient(conn)

	// Get plugin name (legacy method, fast)
	nameResp, err := api.Name(ctx, &proto.Empty{})
	if err != nil {
		if closeErr := closeFn(); closeErr != nil {
			return nil, fmt.Errorf("getting plugin name: %w (close error: %w)", err, closeErr)
		}
		return nil, fmt.Errorf("getting plugin name: %w", err)
	}

	client := &Client{
		Name:  nameResp.GetName(),
		Conn:  conn,
		API:   api,
		Close: closeFn,
	}

	// Fetch plugin info with timeout
	const infoTimeout = 5 * time.Second
	infoCtx, cancel := context.WithTimeout(ctx, infoTimeout)
	defer cancel()

	infoResp, err := api.GetPluginInfo(infoCtx, &proto.Empty{})
	if err != nil {
		handleGetPluginInfoError(ctx, client.Name, err)
		return client, nil
	}

	// Store metadata
	client.Metadata = &proto.PluginMetadata{
		Name:               infoResp.GetName(),
		Version:            infoResp.GetVersion(),
		SpecVersion:        infoResp.GetSpecVersion(),
		SupportedProviders: infoResp.GetProviders(),
		Metadata:           infoResp.GetMetadata(),
	}

	// Check version compatibility (may return error in strict mode)
	if compatErr := checkVersionCompatibility(ctx, client.Name, infoResp.GetSpecVersion()); compatErr != nil {
		if closeErr := closeFn(); closeErr != nil {
			return nil, fmt.Errorf("%w (close error: %w)", compatErr, closeErr)
		}
		return nil, compatErr
	}

	return client, nil
}

// handleGetPluginInfoError logs plugin info retrieval issues.
// If the error indicates the GetPluginInfo RPC is unimplemented, it logs a debug message noting a legacy plugin.
// For any other error (timeout, network, etc.) it logs a warning that includes the error and the plugin name.
func handleGetPluginInfoError(ctx context.Context, pluginName string, err error) {
	log := logging.FromContext(ctx)
	if IsUnimplementedError(err) {
		log.Debug().Str("plugin", pluginName).Msg("Plugin does not support GetPluginInfo (legacy)")
		return
	}
	// Log warning for other errors (timeout, etc) but continue
	log.Warn().Err(err).Str("plugin", pluginName).Msg("Failed to get plugin info")
}

// checkVersionCompatibility validates plugin spec version compatibility.
// In permissive mode (default), it logs a warning on mismatch.
// checkVersionCompatibility validates a plugin's spec version against the core spec
// version and enforces strict compatibility when configured.
//
// It reads SkipVersionCheckKey from ctx; if set to true the check is skipped.
// pluginName is used only for logging and error messages. pluginSpecVersion is the
// spec version reported by the plugin.
//
// Returns nil if the check is skipped, the versions are compatible, or the plugin's
// spec version cannot be parsed (parse errors are logged but not blocking).
// Returns an error wrapping ErrPluginIncompatible when a major-version mismatch is
// detected and strict plugin compatibility is enabled via config.GetStrictPluginCompatibility().
func checkVersionCompatibility(ctx context.Context, pluginName, pluginSpecVersion string) error {
	v, ok := ctx.Value(SkipVersionCheckKey).(bool)
	skipCheck := ok && v
	if skipCheck {
		return nil
	}

	log := logging.FromContext(ctx)
	result, verErr := CompareSpecVersions(pluginsdk.SpecVersion, pluginSpecVersion)
	if verErr != nil {
		log.Warn().Err(verErr).Str("plugin", pluginName).Msg("Failed to parse plugin spec version")
		return nil // Parse errors are not blocking
	}

	if result == MajorMismatch {
		log.Warn().
			Str("plugin", pluginName).
			Str("core_spec", pluginsdk.SpecVersion).
			Str("plugin_spec", pluginSpecVersion).
			Msg("Plugin spec version mismatch: this may cause instability")

		// In strict mode, return an error to block plugin initialization
		if config.GetStrictPluginCompatibility() {
			return fmt.Errorf("%w: plugin %s has spec version %s, core requires compatible with %s",
				ErrPluginIncompatible, pluginName, pluginSpecVersion, pluginsdk.SpecVersion)
		}
	}

	return nil
}