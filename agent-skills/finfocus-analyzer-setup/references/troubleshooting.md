# Analyzer Troubleshooting

## "could not start policy pack"

**Cause:** `pulumi-analyzer-policy-finfocus` not found on PATH.

```bash
# Fix: add policy pack dir to PATH
export PATH="$HOME/.finfocus/analyzer:$PATH"

# Verify
which pulumi-analyzer-policy-finfocus
```

Pulumi searches PATH to find the binary — having it only in the policy pack
directory is not sufficient unless that directory is on PATH.

## No cost diagnostics appear

**Causes and checks:**

1. `PulumiPolicy.yaml` missing or malformed:

   ```bash
   cat ~/.finfocus/analyzer/PulumiPolicy.yaml
   # Must contain: runtime: finfocus
   ```

2. Wrong `--policy-pack` path:

   ```bash
   pulumi preview --policy-pack ~/.finfocus/analyzer
   ```

3. No cost plugins installed:

   ```bash
   finfocus plugin list
   ```

4. Debug with verbose logging:

   ```bash
   FINFOCUS_LOG_LEVEL=debug pulumi preview --policy-pack ~/.finfocus/analyzer
   ```

## Version mismatch after upgrade

**Cause:** Installed analyzer binary points to old finfocus version.

```bash
finfocus analyzer install --force
```

## gRPC smoke test fails

**Causes:**

- Port binding failure (another process using the port)
- Plugin loading error during `analyzer serve`
- Binary corruption

**Fix:**

```bash
# Manual smoke test — should print a port number and wait
finfocus analyzer serve

# Reinstall
finfocus analyzer install --force
```

## Plugin isolation in analyzer mode

Control which plugins load when running as an analyzer:

```yaml
# ~/.finfocus/config.yaml
analyzer:
  plugins:
    aws-public:
      enabled: true
    some-other-plugin:
      enabled: false
```

Or use `FINFOCUS_HOME` for full isolation:

```bash
mkdir -p ~/.finfocus/demo/plugins/aws-public/v0.1.5
ln -sf ~/.finfocus/plugins/aws-public/v0.1.5/finfocus-plugin-aws-public \
    ~/.finfocus/demo/plugins/aws-public/v0.1.5/finfocus-plugin-aws-public

FINFOCUS_HOME=~/.finfocus/demo pulumi preview --policy-pack ~/.finfocus/analyzer
```

**Note:** Directory-level symlinks are not supported (#750). Use file-level
symlinks only.

## Analyzer config tuning

```yaml
# ~/.finfocus/config.yaml
analyzer:
  timeout:
    per_resource: 5s       # Per-resource cost lookup timeout
    total: 60s             # Overall analysis timeout
    warn_threshold: 30s    # Log warning if analysis exceeds this
  max_monthly_cost: 500.00 # Optional cost threshold
  enforcement: advisory    # Always advisory (never blocks)
```

**Environment variable overrides:**

| Variable | Purpose |
|----------|---------|
| `FINFOCUS_MAX_MONTHLY_COST` | Cost threshold override |
| `FINFOCUS_ENFORCEMENT` | Enforcement mode (advisory only — never blocks) |
| `FINFOCUS_HOME` | Override base directory (affects plugin loading) |
| `FINFOCUS_LOG_LEVEL` | Analyzer logging: debug, info, warn, error |
| `FINFOCUS_ANALYZER_MODE` | Set automatically by `analyzer serve` |

## Directory resolution

**Pulumi plugin dir:** `--target-dir` > `$PULUMI_HOME/plugins/` >
`~/.pulumi/plugins/`

**Policy pack dir:** `$FINFOCUS_HOME/analyzer/` > `~/.finfocus/analyzer/`

## Important: `analyzers:` in Pulumi.yaml

Adding `analyzers:` to `Pulumi.yaml` does **not** work for YAML-runtime
projects. Use the `--policy-pack` flag or `PULUMI_POLICY_PACK_PATH` env var.

## Handshake protocol (advanced)

When Pulumi invokes the analyzer:

1. Pulumi executes `pulumi-analyzer-policy-finfocus`
2. Binary starts gRPC server on random TCP port
3. Prints ONLY the port number to stdout (single line)
4. Pulumi connects to `127.0.0.1:PORT` via gRPC
5. All logging goes to stderr exclusively
