// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package pluginhost

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/proto"
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
	// Conn is the plugin connection. A holder of a pooled process that reads
	// per-request credentials gets one that attaches its own to every call.
	Conn  grpc.ClientConnInterface
	API   proto.CostSourceClient
	Close func() error
	// DeliveredDigest hashes the settings this plugin process received.
	// Empty means nothing was delivered, and cache keys stay unchanged.
	DeliveredDigest string
}

// Launcher is an interface for different plugin launching strategies (TCP or stdio).
type Launcher interface {
	Start(ctx context.Context, path string, args ...string) (*grpc.ClientConn, func() error, error)
}

// NewClient creates a new plugin client by launching the plugin and establishing a gRPC connection.
// When the context carries an install name, settings from the config files and the
// plugin's advertised descriptors are injected before the process starts. The process
// is started once more when GetPluginInfo names variables the first launch did not have.
func NewClient(ctx context.Context, launcher Launcher, binPath string) (*Client, error) {
	client, _, err := launchClient(ctx, launcher, binPath, resolveLaunchInputs(ctx))
	return client, err
}

// launchInputs is what a launch resolves from its context before the process
// starts. sources is the one read of settings inputs that every resolution of
// this launch uses, including ESC values the plugin is allowed only after it
// advertises them. Only the pool sets credentials.
type launchInputs struct {
	installName string
	named       bool
	start       launchState
	sources     settingSources
	credentials credentialPlan
}

// launchState is what one process of a launch receives. channel means the
// declared credentials are left out, so it must read them from each call.
type launchState struct {
	delivered Delivered
	settings  []*pbc.PluginSetting
	channel   bool
	// unproven means a process without the credentials did not announce its
	// port, so this one has them in its environment.
	unproven bool
}

func resolveLaunchInputs(ctx context.Context) launchInputs {
	installName, named := InstallNameFromContext(ctx)
	if !named {
		return launchInputs{}
	}
	in := launchInputs{
		installName: installName,
		named:       true,
		sources:     readSettingSources(ctx, installName),
	}
	in.start = in.state(ctx, nil, false)
	return in
}

// state resolves what a process with these advertised settings receives. With
// channel, the ESC values of the declared credentials are left out.
func (in launchInputs) state(ctx context.Context, settings []*pbc.PluginSetting, channel bool) launchState {
	sources := in.sources
	if channel {
		sources.escOffer = withoutVariables(sources.escOffer, in.credentials.withheld)
	}
	return launchState{
		delivered: sources.resolve(ctx, in.installName, settings),
		settings:  settings,
		channel:   channel,
	}
}

// stateFor resolves what a process that gave this GetPluginInfo answer
// receives. Without an answer the known settings apply, and the credentials
// stay in the environment.
func (in launchInputs) stateFor(
	ctx context.Context, info *pbc.GetPluginInfoResponse, known []*pbc.PluginSetting,
) launchState {
	if info != nil {
		known = info.GetSettings()
	}
	return in.state(ctx, known, in.credentials.usable() && advertisesCredentials(info))
}

// launchOutcome is what a successful launch ended with: what the final process
// received and the GetPluginInfo answer it gave (nil when the call failed or
// is not implemented).
type launchOutcome struct {
	state launchState
	info  *pbc.GetPluginInfoResponse
}

func launchClient(
	ctx context.Context, launcher Launcher, binPath string, in launchInputs,
) (*Client, launchOutcome, error) {
	started, state, err := connectState(ctx, launcher, binPath, in, in.start)
	if err != nil {
		return nil, launchOutcome{}, err
	}
	if in.named {
		started, state, err = applyAdvertisedSettings(ctx, launcher, binPath, in, started, state)
		if err != nil {
			return nil, launchOutcome{}, err
		}
	}
	outcome := launchOutcome{state: state, info: started.info}

	client := &Client{
		Name:            started.name,
		Conn:            started.conn,
		API:             started.api,
		Close:           started.idempotentClose(),
		DeliveredDigest: state.delivered.Digest,
	}
	if started.info == nil {
		return client, outcome, nil
	}

	client.Metadata = pluginMetadata(started.info)
	if compatErr := checkVersionCompatibility(ctx, client.Name, started.info.GetSpecVersion()); compatErr != nil {
		if closeErr := client.Close(); closeErr != nil {
			return nil, launchOutcome{}, fmt.Errorf("%w (close error: %w)", compatErr, closeErr)
		}
		return nil, launchOutcome{}, compatErr
	}
	return client, outcome, nil
}

// connectState starts a process that receives state. A process left without
// its credentials will read them from calls, so it must announce its port
// before the host connects: that proves the host talks to the process it
// launched. One that does not is replaced by a process that has them, and the
// returned state says so.
func connectState(
	ctx context.Context, launcher Launcher, binPath string, in launchInputs, state launchState,
) (pluginConnection, launchState, error) {
	if state.channel {
		started, err := connectPlugin(
			withAnnouncedPortRequired(WithDeliveredEnv(ctx, state.delivered)), launcher, binPath)
		if !errors.Is(err, errPortNotAnnounced) {
			return started, state, err
		}
		logging.FromContext(ctx).Warn().
			Str("component", "pluginhost").
			Str("operation", "start_plugin").
			Str("plugin", in.installName).
			Msg("plugin did not announce its port; per-request credentials are off for this process")
		state = in.state(ctx, state.settings, false)
		state.unproven = true
	}
	if in.named {
		ctx = WithDeliveredEnv(ctx, state.delivered)
	}
	started, err := connectPlugin(ctx, launcher, binPath)
	return started, state, err
}

// applyAdvertisedSettings restarts the process once when GetPluginInfo names
// variables the first launch did not receive, or says that credentials can
// come with each call. A second difference is kept, except that a process left
// without its credentials must read them from calls: if it does not say so, it
// is replaced by one that has them. It reuses the settings sources read before
// the launch, so the environment it delivers is the one a pool keyed the
// process by.
func applyAdvertisedSettings(
	ctx context.Context,
	launcher Launcher,
	binPath string,
	in launchInputs,
	started pluginConnection,
	state launchState,
) (pluginConnection, launchState, error) {
	if state.unproven || started.info == nil && !state.channel {
		return started, state, nil
	}
	next := in.stateFor(ctx, started.info, state.settings)
	if sameDelivery(state.delivered, next.delivered) && (state.channel || !next.channel) {
		return started, next, nil
	}
	started, next, err := restartPlugin(ctx, launcher, binPath, in, started, next)
	if err != nil {
		return pluginConnection{}, launchState{}, err
	}
	if !next.channel {
		if started.info != nil {
			unproven := next.unproven
			next = in.state(ctx, started.info.GetSettings(), false)
			next.unproven = unproven
		}
		return started, next, nil
	}
	settled := in.stateFor(ctx, started.info, next.settings)
	if settled.channel || sameDelivery(next.delivered, settled.delivered) {
		return started, settled, nil
	}
	return restartPlugin(ctx, launcher, binPath, in, started, settled)
}

// restartPlugin replaces a running process by one that receives next.
func restartPlugin(
	ctx context.Context,
	launcher Launcher,
	binPath string,
	in launchInputs,
	running pluginConnection,
	next launchState,
) (pluginConnection, launchState, error) {
	if closeErr := running.closeFn(); closeErr != nil {
		logging.FromContext(ctx).Warn().
			Str("component", "pluginhost").
			Str("operation", "restart_plugin").
			Str("plugin", in.installName).
			Err(closeErr).
			Msg("closing plugin before applying advertised settings")
	}
	logging.FromContext(ctx).Debug().
		Str("component", "pluginhost").
		Str("operation", "restart_plugin").
		Str("plugin", in.installName).
		Int("count", len(next.delivered.Pairs)).
		Msg("restarting plugin to apply advertised settings")
	return connectState(ctx, launcher, binPath, in, next)
}

func sameDelivery(left, right Delivered) bool {
	return sameStringSet(left.Pairs, right.Pairs) &&
		sameStringSet(overrideKeys(left.Overrides), overrideKeys(right.Overrides))
}

func overrideKeys(overrides map[string]bool) []string {
	keys := make([]string, 0, len(overrides))
	for name := range overrides {
		keys = append(keys, name)
	}
	return keys
}

type pluginConnection struct {
	conn    *grpc.ClientConn
	closeFn func() error
	api     proto.CostSourceClient
	name    string
	info    *pbc.GetPluginInfoResponse
}

func (c pluginConnection) idempotentClose() func() error {
	var closeOnce sync.Once
	var onceCloseErr error
	return func() error {
		closeOnce.Do(func() {
			onceCloseErr = c.closeFn()
		})
		return onceCloseErr
	}
}

func connectPlugin(
	ctx context.Context,
	launcher Launcher,
	binPath string,
) (pluginConnection, error) {
	conn, closeFn, err := launcher.Start(ctx, binPath)
	if err != nil {
		return pluginConnection{}, err
	}
	api := proto.NewCostSourceClient(conn)
	nameResp, err := api.Name(ctx, &proto.Empty{})
	if err != nil {
		if closeErr := closeFn(); closeErr != nil {
			return pluginConnection{}, fmt.Errorf("getting plugin name: %w (close error: %w)", err, closeErr)
		}
		return pluginConnection{}, fmt.Errorf("getting plugin name: %w", err)
	}
	started := pluginConnection{
		conn:    conn,
		closeFn: closeFn,
		api:     api,
		name:    nameResp.GetName(),
	}
	const infoTimeout = 5 * time.Second
	infoCtx, cancel := context.WithTimeout(ctx, infoTimeout)
	defer cancel()
	infoResp, err := api.GetPluginInfo(infoCtx, &proto.Empty{})
	if err != nil {
		handleGetPluginInfoError(ctx, started.name, err)
		return started, nil
	}
	started.info = infoResp
	return started, nil
}

func pluginMetadata(info *pbc.GetPluginInfoResponse) *proto.PluginMetadata {
	return &proto.PluginMetadata{
		Name:               info.GetName(),
		Version:            info.GetVersion(),
		SpecVersion:        info.GetSpecVersion(),
		SupportedProviders: info.GetProviders(),
		Metadata:           info.GetMetadata(),
		Capabilities:       ConvertCapabilities(info.GetCapabilities()),

		CapabilitiesDeclared: len(info.GetCapabilities()) > 0,
	}
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
		// In strict mode, parse errors also block plugin initialization
		if config.GetStrictPluginCompatibility() {
			return fmt.Errorf(
				"%w: failed to parse spec version %q for plugin %s: %s",
				ErrPluginIncompatible, pluginSpecVersion, pluginName, verErr.Error(),
			)
		}
		return nil // Parse errors are not blocking in permissive mode
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

// CapabilityRecommendationScoring is the normalized capability name of a scorer plugin.
const CapabilityRecommendationScoring = "recommendation_scoring"

// Capability names for services core gates on before an RPC.
const (
	// CapabilityUsageStats is the normalized capability name of a usage-source plugin.
	CapabilityUsageStats = "usage_stats"
	// CapabilityAllocation is the normalized capability name of an allocator plugin.
	CapabilityAllocation = "allocation"
	// CapabilityBudgets is the normalized capability name of a plugin that serves GetBudgets.
	CapabilityBudgets = "budgets"
)

// ConvertCapabilities converts proto PluginCapability enums to string slice.
// Returns capability names in lowercase format: "projected_costs", "actual_costs", etc.
func ConvertCapabilities(caps []pbc.PluginCapability) []string {
	if len(caps) == 0 {
		return nil
	}

	result := make([]string, 0, len(caps))
	for _, capability := range caps {
		switch capability {
		case pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS:
			result = append(result, "projected_costs")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS:
			result = append(result, "actual_costs")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_RECOMMENDATIONS:
			result = append(result, "recommendations")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_DRY_RUN:
			result = append(result, "dry_run")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_BUDGETS:
			result = append(result, CapabilityBudgets)
		case pbc.PluginCapability_PLUGIN_CAPABILITY_HISTORICAL_CARBON:
			result = append(result, CapabilityHistoricalCarbon)
		case pbc.PluginCapability_PLUGIN_CAPABILITY_CARBON:
			result = append(result, "carbon")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_ENERGY:
			result = append(result, "energy")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_WATER:
			result = append(result, "water")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_PRICING_SPEC:
			result = append(result, "pricing_spec")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_ESTIMATE_COST:
			result = append(result, "estimate_cost")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_DISMISS_RECOMMENDATIONS:
			result = append(result, "dismiss_recommendations")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_BATCH_COST:
			result = append(result, "batch_cost")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_RESOLVE_RESOURCE_TYPES:
			result = append(result, "resolve_resource_types")
		case pbc.PluginCapability_PLUGIN_CAPABILITY_USAGE_STATS:
			result = append(result, CapabilityUsageStats)
		case pbc.PluginCapability_PLUGIN_CAPABILITY_ALLOCATION:
			result = append(result, CapabilityAllocation)
		case pbc.PluginCapability_PLUGIN_CAPABILITY_RECOMMENDATION_SCORING:
			result = append(result, CapabilityRecommendationScoring)
		case pbc.PluginCapability_PLUGIN_CAPABILITY_UNSPECIFIED,
			pbc.PluginCapability_PLUGIN_CAPABILITY_CONTRACT_COMMITMENTS,
			pbc.PluginCapability_PLUGIN_CAPABILITY_INVOICE_DATA,
			pbc.PluginCapability_PLUGIN_CAPABILITY_COST_PARSE:
			// Not a capability core acts on
			continue
		default:
			// Unknown capability - skip
			continue
		}
	}

	return result
}

// HasCapability checks whether the plugin client advertises a given capability string.
// Returns false if the client has no metadata.
func (c *Client) HasCapability(capability string) bool {
	if c.Metadata == nil {
		return false
	}
	return slices.Contains(c.Metadata.Capabilities, capability)
}
