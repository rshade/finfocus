---
title: Security
layout: default
---

FinFocus is designed to keep cost data and credentials local, with clear boundaries between
core workflows, plugins, and deployment environments. Use this guide to secure deployments for
developers, DevOps engineers, and platform teams.

## Credential Handling

FinFocus does **not** store cloud provider credentials. It relies on the environment configuration
(e.g., `AWS_PROFILE`, `AZURE_CREDENTIALS`) or credentials provided to the Pulumi engine.

- **OIDC in CI/CD**: Prefer short-lived credentials with OIDC.
  - GitHub Actions: use `permissions: { id-token: write, contents: read }` and
    `aws-actions/configure-aws-credentials`.
  - GitLab CI: use `id_tokens` and cloud provider OIDC federation.
- **Environment inheritance**: Plugins run as separate processes and inherit environment variables.
  Limit the environment to only required variables.
- **Hygiene**: Avoid printing secrets in logs, scrub debug output, and scope variables to steps.
- **Plugin isolation**: Treat plugin-specific credentials as scoped to a single provider.
- **Avoid exposure**: Do not pass credentials via CLI arguments or config files committed to repo.

## Data Privacy

FinFocus processes cost data locally or within your CI runner.

- **No SaaS Dependency**: The core engine does not send data externally unless you configure a
  plugin (for example, Vantage) to do so.
- **Local Specs**: Pricing data is read from public sources or local spec files.

## Plugin Security

Plugins are executable binaries that run as separate processes and communicate over gRPC on
`127.0.0.1`.

- **Execution model**: Plugins are trusted code and are not sandboxed.
- **gRPC transport**: Uses insecure credentials but is restricted to localhost.
- **Plugin discovery**: Plugins are discovered under `~/.finfocus/plugins/`.
- **Strict compatibility**: `plugin_host.strict_compatibility` blocks plugin initialization when
  plugin spec versions do not match, preventing unvetted behavior changes.
- **File permissions**: Use `0700` for `~/.finfocus` directories and `0600` for config files.
- **Atomic config writes**: Write config updates to a temp file and rename to avoid partial writes.
- **Source verification**: Only install plugins from trusted sources.
- **Current limitations**: No binary signature verification or checksum validation yet.

## Network Security

- **Localhost-only gRPC**: All core plugin traffic stays on `127.0.0.1`.
- **No outbound requirement**: Core FinFocus functionality does not require external network
  access.
- **Plugin networking**: Plugins may call external APIs (for example, Vantage) when configured.
- **Container policies**: Apply egress allow-lists and network policies for container deployments.

## CI/CD Security Best Practices

- **Secret management**: Store credentials in platform-native secret stores.
- **Least privilege**: Use minimal workflow permissions (`contents: read`, `checks: write`).
- **OIDC adoption**: Prefer short-lived tokens over static credentials.
- **Vulnerability scanning**: Integrate `govulncheck` and Trivy into pipelines.
- **SARIF output**: Publish vulnerability findings to code scanning.
- **Environment scoping**: Pass credentials only to steps that run `finfocus`.

See [CI/CD Integration](cicd-integration.html) for platform examples.

## Container Security

Our Docker images are built using minimal base images (Alpine) and run as non-root users.

- **Multi-stage builds**: Reduce attack surface by copying only the final binary.
- **SBOM generation**: CI generates SPDX-JSON SBOMs with `anchore/sbom-action`.
- **Vulnerability scans**: Trivy scans images and publishes results for verification.
- **Inspecting SBOMs**: Review SBOM artifacts in CI to validate dependencies.
- **Supply chain**: Verify image digests and pin tags in production deployments.

See the [Docker Guide](docker.html) for operational details.

## Reporting Vulnerabilities

If you discover a security vulnerability, do not open a public issue. Use the
[GitHub Security Advisory](https://github.com/rshade/finfocus/security/advisories)
workflow or follow the repository security policy for coordinated disclosure.

## Related Documentation

- [Configuration Guide](configuration.html)
- [CI/CD Integration](cicd-integration.html)
- [Docker Deployment](docker.html)
- [Troubleshooting](troubleshooting.html)
