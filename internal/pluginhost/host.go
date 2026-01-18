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

// NewClient creates a new plugin client by launching the plugin and establishing a gRPC connection.
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
// In strict mode, it returns an error on mismatch.
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
