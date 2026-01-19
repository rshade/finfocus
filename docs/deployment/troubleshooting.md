---
title: Deployment Troubleshooting
layout: default
---

If you encounter issues deploying FinFocus, please refer to our comprehensive troubleshooting guide.

- [Troubleshooting Guide](../support/troubleshooting.md)

## Common Deployment Issues

### Docker Permission Denied

If you see permission errors when mounting volumes in Docker, ensure the host directory is owned
by the user running the container or use `chmod` to grant access.

Debugging steps:

1. Confirm the host path exists and is writable by the container user.
2. Run `ls -la` on the host path and look for mismatched ownership.
3. Use a dedicated data directory and map it with `-v /path:/data`.

See [Docker Guide](docker.md#troubleshooting) for specific commands.

### Docker Image Pull Failures

Image pulls that fail with authentication or rate limit errors typically indicate missing
registry credentials.

Debugging steps:

1. Ensure you are logged into the container registry (`docker login`).
2. Confirm the image name and tag match your deployment configuration.
3. Retry the pull from the same host to verify network reachability.

### CLI Not Found in PATH

Deployments can fail if the `finfocus` binary is missing or not exported to `PATH`.

Debugging steps:

1. Run `which finfocus` to confirm the binary location.
2. Verify the install step or artifact download ran in the pipeline.
3. Export the binary path (for example, `export PATH=$PATH:/opt/finfocus/bin`).

### Plugin Not Installed or Version Mismatch

Missing plugins or a version mismatch can prevent validation or pricing steps.

Debugging steps:

1. Check the plugin directory at `~/.finfocus/plugins/<name>/<version>/`.
2. Reinstall the plugin with `finfocus plugin init` or your artifact cache.
3. Run `finfocus plugin validate` to verify compatibility.

### Config or Credentials Missing

FinFocus relies on environment variables or `~/.finfocus/config.yaml` for providers.

Debugging steps:

1. Confirm required `FINFOCUS_*` variables are set in the runtime environment.
2. Validate that `~/.finfocus/config.yaml` exists and is readable.
3. Log masked config values in CI to confirm they are injected.

### Network and Proxy Errors

If deployments fail with timeouts or proxy errors, the runtime may not be able to reach
provider APIs.

Debugging steps:

1. Test DNS resolution and outbound access from the host or container.
2. Verify proxy configuration variables match your environment.
3. Re-run with `FINFOCUS_LOG_LEVEL=debug` to capture failed endpoints.

### GitHub Actions (finfocus-action)

If your workflow uses `github.com/rshade/finfocus-action` and fails:

1. Confirm the action version tag matches your workflow pin.
2. Ensure the workflow sets required `FINFOCUS_*` env vars or config secrets.
3. Enable debug logging with `FINFOCUS_LOG_LEVEL=debug` for richer logs.
4. Check that the action inputs match the expected spec in the action README.

### Generic CI/CD Pipeline Failures

If FinFocus fails in CI/CD:

1. Enable debug logging: `FINFOCUS_LOG_LEVEL=debug`.
2. Check that the `finfocus` binary is in the `PATH`.
3. Verify that plugins are correctly installed or cached.
4. Confirm secrets are available to the job and not masked to empty strings.

For more help, see the [Support](../support/support-channels.md) options.
