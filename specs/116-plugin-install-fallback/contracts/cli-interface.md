# CLI Contract: Plugin Install with Fallback

**Date**: 2026-01-18
**Feature**: 116-plugin-install-fallback

## Command Signature

```text
finfocus plugin install <plugin> [flags]
```

## New Flags

Flag | Short | Type | Default | Description
-----|-------|------|---------|------------
`--fallback-to-latest` | - | bool | false | Automatically install latest stable version if requested version lacks assets
`--no-fallback` | - | bool | false | Disable fallback behavior entirely

## Flag Combinations

Interactive | --fallback-to-latest | --no-fallback | Behavior on Missing Assets
------------|----------------------|---------------|---------------------------
Yes | false | false | Prompt user with Y/n (default: No)
Yes | true | false | Auto-fallback, show message
Yes | false | true | Fail immediately
No | false | false | Fail immediately (preserve existing behavior)
No | true | false | Auto-fallback, show message
No | false | true | Fail immediately
- | true | true | ERROR: mutually exclusive flags

## Exit Codes

Code | Condition
-----|----------
0 | Successful installation (including fallback)
1 | Installation failed (including declined fallback)
2 | Invalid arguments (mutually exclusive flags)

## Output Formats

### Interactive Prompt (TTY, no flags)

```text
Warning: No compatible assets found for aws-public@v0.1.3 (linux/amd64).
? Would you like to install the latest stable version (v0.1.2) instead? [y/N]
```

### Successful Installation with Fallback

```text
Warning: No compatible assets found for aws-public@v0.1.3 (linux/amd64).
Installing aws-public@v0.1.2 (fallback from v0.1.3)...
Downloading finfocus-plugin-aws-public_v0.1.2_Linux_amd64.tar.gz (15.2 MB)...
Downloading... 100%
Extracting archive...
Successfully installed aws-public@v0.1.2

✓ Plugin installed successfully
  Name:    aws-public
  Version: v0.1.2 (requested: v0.1.3)
  Path:    /home/user/.finfocus/plugins/aws-public/v0.1.2
```

### Declined Fallback

```text
Warning: No compatible assets found for aws-public@v0.1.3 (linux/amd64).
? Would you like to install the latest stable version (v0.1.2) instead? [y/N] n
Installation aborted.
```

### No Fallback Available

```text
Error: no compatible assets found for aws-public@v0.1.3 or any of 10 stable releases
```

### Mutually Exclusive Flags Error

```text
Error: if any flags in the group [fallback-to-latest no-fallback] are set none of the others can be; [fallback-to-latest no-fallback] were all set
```

## Help Text

```text
finfocus plugin install - Install a plugin from registry or URL

Usage:
  finfocus plugin install <plugin> [flags]

Examples:
  # Install latest version from registry
  finfocus plugin install kubecost

  # Install specific version from registry
  finfocus plugin install kubecost@v1.0.0

  # Install from GitHub URL
  finfocus plugin install github.com/rshade/finfocus-plugin-aws-public

  # Auto-fallback to latest stable if requested version lacks assets (CI mode)
  finfocus plugin install kubecost@v1.0.0 --fallback-to-latest

  # Fail immediately if requested version lacks assets (strict mode)
  finfocus plugin install kubecost@v1.0.0 --no-fallback

Flags:
      --clean               Remove other versions after successful install
      --fallback-to-latest  Automatically install latest stable version if requested version lacks assets
  -f, --force               Reinstall even if version already exists
  -h, --help                help for install
      --no-fallback         Disable fallback behavior entirely (fail if requested version lacks assets)
      --no-save             Don't add plugin to config file
      --plugin-dir string   Custom plugin directory (default: ~/.finfocus/plugins)
```

## Error Handling Contract

Scenario | Error Message | Exit Code
---------|---------------|-----------
Version exists, no assets, no fallback | `no asset found for linux/amd64. Available: []` | 1
No stable releases | `no stable releases found` | 1
All releases lack assets | `no compatible asset found for version X or any of 10 fallback releases` | 1
User declines fallback | `Installation aborted.` | 1
Mutually exclusive flags | Cobra's standard mutual exclusion error | 2
