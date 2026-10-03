# Upgrade to finfocus-spec v0.6.2: Supports reaches the host

Before v0.6.2, a plugin that did not implement `Supports` made the host see
an error. From v0.6.2 the SDK answers for it with `Supported: false` and the
reason `pluginsdk.DefaultSupportsNotImplementedReason`.

finfocus core treats that reason as "unknown" and still routes to the
plugin. Other hosts may take `false` at face value and skip the plugin.

## Manual

1. **Implement `pluginsdk.SupportsProvider`** if the plugin does not
   already:

   ```go
   func (p *Plugin) Supports(ctx context.Context, req *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
       r := req.GetResource()
       if r.GetProvider() != "aws" || !p.knowsType(r.GetResourceType()) {
           return &pbc.SupportsResponse{Supported: false, Reason: "unsupported resource type"}, nil
       }
       return &pbc.SupportsResponse{Supported: true}, nil
   }
   ```

   Base the answer on what the pricing code can actually price. Do not
   change the pricing code to make the answer match.

2. **Check hand-set capabilities.** If `GetPluginInfo` uses
   `pluginsdk.WithCapabilities`, the SDK now compares the list with the
   capabilities it infers from the implemented interfaces. It fills an empty
   list and logs a drift warning when they differ. List every implemented
   service, or drop the option and use the inferred set.

## Verify

```bash
finfocus plugin list --output json
```

The plugin's `capabilities` in the list output should match the services it
implements. A missing `notes` field confirms the plugin started.
