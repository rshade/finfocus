---
applyTo: 'go.mod'
---

# Go Module Instructions

The go.mod file manages Go module dependencies and version requirements.

## Go Version Management:

- **Current version**: Go 1.25.6
- **Compatibility**: Ensure all dependencies support Go 1.25.6
- **Updates**: Update version consistently across all documentation

## Dependency Management:

- Use semantic versioning for dependencies
- Prefer stable, well-maintained packages
- Minimize dependency footprint
- Review dependencies for security vulnerabilities
- Use `go mod tidy` to clean up unused dependencies

## Module Organization:

- Use proper module path
- Include replace directives for local development
- Maintain clean dependency tree
- Document major dependency changes

## Security Considerations:

- Regularly audit dependencies for vulnerabilities
- Use `govulncheck` for security scanning
- Update dependencies to patch security issues
- Review dependency licenses

## Version Consistency:

- Keep Go version in sync across:
  - go.mod
  - CI/CD workflows
  - Documentation files
  - Docker images
  - Development environment setup

## Build Requirements:

- Ensure dependencies compile with target Go version
- Test builds with different Go versions when needed
- Validate module compatibility
- Check for deprecated dependencies

## Local Development:

- Use `go mod download` to fetch dependencies
- Handle replace directives for local packages
- Maintain consistent development environment
- Document setup requirements
