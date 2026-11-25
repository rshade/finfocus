package pluginhost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"os"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/logging"
)

// credentialPlan is how one holder passes its plugin's declared credentials:
// the ESC variables that leave the process environment and the set its calls
// carry instead. The zero plan keeps everything in the environment.
type credentialPlan struct {
	// mapping lists the configured name and variable pairs, even when the plan
	// is not usable. It holds no value.
	mapping []string
	// withheld names the declared variables. It is empty when nothing is
	// declared or the holder's values do not fit the contract.
	withheld map[string]bool
	// set holds this holder's values. It is empty when ESC supplies none.
	set pluginsdk.Credentials
}

// resolveCredentialPlan reads the declared map for plugin and this holder's
// values for it. Only ESC values become credentials: a shell or config value
// is the same for every holder and stays in the environment.
func resolveCredentialPlan(ctx context.Context, plugin string, offer esc.Delivery) credentialPlan {
	src, ok := esc.SourceFromContext(ctx)
	if !ok {
		return credentialPlan{}
	}
	declared := src.Credentials(plugin)
	if len(declared) == 0 {
		return credentialPlan{}
	}
	plan := credentialPlan{withheld: make(map[string]bool, len(declared))}
	values := make(map[string]string, len(declared))
	for _, name := range slices.Sorted(maps.Keys(declared)) {
		variable := declared[name]
		plan.mapping = append(plan.mapping, name, variable)
		plan.withheld[variable] = true
		if value, offered := offer.Values[variable]; offered {
			values[name] = value
		}
	}
	if len(values) == 0 {
		return plan
	}
	set, err := pluginsdk.NewCredentials(values)
	if err != nil {
		logging.FromContext(ctx).Warn().
			Str("component", "pluginhost").
			Str("operation", "resolve_credentials").
			Str("plugin", plugin).
			Msg("per-request credentials are not usable; the plugin runs in its own process")
		return credentialPlan{mapping: plan.mapping}
	}
	plan.set = set
	return plan
}

// usable reports whether a plugin that reads per-request credentials may run
// without the declared variables.
func (p credentialPlan) usable() bool {
	return len(p.withheld) > 0
}

// digest adds the credentials a holder's calls carry to the digest of its full
// environment. That digest already covers each value's ESC reference and
// revision; the map decides which of them is sent, and under which name.
func (p credentialPlan) digest(environment string) string {
	if p.set.Len() == 0 {
		return environment
	}
	sum := sha256.New()
	for _, part := range append([]string{environment}, p.mapping...) {
		_, _ = sum.Write([]byte(part))
		_, _ = sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// advertisesCredentials reports whether the plugin reads per-request credentials.
func advertisesCredentials(info *pbc.GetPluginInfoResponse) bool {
	return info.GetMetadata()[pluginsdk.MetadataSupportsPerRequestCredentials] == pluginsdk.ValueTrue
}

// withoutVariables returns offer without the named variables.
func withoutVariables(offer esc.Delivery, names map[string]bool) esc.Delivery {
	values := maps.Clone(offer.Values)
	maps.DeleteFunc(values, func(name, _ string) bool { return names[name] })
	return esc.Delivery{Values: values, Origins: offer.Origins, Fingerprints: offer.Fingerprints}
}

// credentialConn attaches one holder's credentials to every unary call, which
// every plugin service call is. The connection underneath is shared, so the
// set must never be stored on it. It is kept in a closure because fmt would
// print a field: it ignores the set's own String method inside this struct.
type credentialConn struct {
	grpc.ClientConnInterface

	withSet func(context.Context) context.Context
	attach  grpc.UnaryClientInterceptor
}

func newCredentialConn(conn grpc.ClientConnInterface, set pluginsdk.Credentials) credentialConn {
	return credentialConn{
		ClientConnInterface: conn,
		withSet: func(ctx context.Context) context.Context {
			return pluginsdk.WithCredentials(ctx, set)
		},
		attach: pluginsdk.CredentialUnaryClientInterceptor(),
	}
}

// Invoke makes one call that carries the holder's credentials.
func (c credentialConn) Invoke(
	ctx context.Context, method string, args, reply any, opts ...grpc.CallOption,
) error {
	return c.attach(c.withSet(ctx), method, args, reply, nil,
		func(ctx context.Context, method string, req, reply any, _ *grpc.ClientConn, opts ...grpc.CallOption) error {
			return c.ClientConnInterface.Invoke(ctx, method, req, reply, opts...)
		}, opts...)
}

// credentialStats masks the call's credentials in the error that telemetry
// records. Telemetry sees a call end before any interceptor does.
type credentialStats struct {
	stats.Handler
}

// HandleRPC passes the event on, with a masked error when a call ends.
func (s credentialStats) HandleRPC(ctx context.Context, event stats.RPCStats) {
	if end, ok := event.(*stats.End); ok && end.Error != nil {
		masked := *end
		masked.Error = maskCredentials(ctx, end.Error)
		event = &masked
	}
	s.Handler.HandleRPC(ctx, event)
}

// maskCredentials removes the credentials that ctx attached to a call, or
// delivered in the environment of the process being launched, from the error
// the plugin answered with. The status code is kept. A status detail
// that holds a value is dropped, since its encoding cannot be edited safely.
func maskCredentials(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	values := append(attachedValues(ctx), launchValues(ctx)...)
	if len(values) == 0 {
		return err
	}
	// Longest first, so a value that contains another is masked whole.
	slices.SortFunc(values, func(a, b string) int { return len(b) - len(a) })
	original := status.Convert(err).Proto()
	masked := status.Convert(err).Proto()
	masked.Message = maskSecrets(masked.GetMessage(), values)
	masked.Details = slices.DeleteFunc(masked.GetDetails(), func(detail *anypb.Any) bool {
		return slices.ContainsFunc(values, func(value string) bool {
			return bytes.Contains(detail.GetValue(), []byte(value))
		})
	})
	if proto.Equal(original, masked) {
		return err
	}
	return status.ErrorProto(masked)
}

// launchValues returns the secrets in the whole environment of the process that
// ctx launches, chosen as for the excerpt of a plugin that exits early.
func launchValues(ctx context.Context) []string {
	delivered, ok := DeliveredEnvFromContext(ctx)
	if !ok {
		return nil
	}
	return credentialValues(appendDeliveredEnv(ctx, os.Environ()), delivered.Origins)
}

// attachedValues returns the credential values ctx carries.
func attachedValues(ctx context.Context) []string {
	set, err := pluginsdk.ExtractCredentials(ctx)
	if err != nil {
		return nil
	}
	values := make([]string, 0, set.Len())
	for _, name := range set.Names() {
		if value, ok := set.Get(name); ok {
			values = append(values, value)
		}
	}
	return values
}
