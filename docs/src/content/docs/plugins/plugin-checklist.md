---
layout: default
title: Plugin Development Checklist
description: Complete checklist for plugin implementation and deployment
---

This checklist ensures your FinFocus plugin is complete, tested, and ready
for production deployment. Use this as a guide during development and before
releasing your plugin.

## Table of Contents

1. [Core Implementation](#core-implementation)
2. [Testing Requirements](#testing-requirements)
3. [Documentation](#documentation)
4. [Deployment Verification](#deployment-verification)
5. [Quality Assurance](#quality-assurance)

---

## Core Implementation

### Required Interfaces

- [ ] Implement `Plugin` interface from `pkg/pluginsdk`
  - [ ] `Name() string` returns unique plugin identifier
  - [ ] `GetProjectedCost()` calculates cost estimates
  - [ ] `GetActualCost()` retrieves historical costs (or returns NoDataError)

- [ ] Use `BasePlugin` for common functionality
  - [ ] Plugin name is lowercase alphanumeric with hyphens
  - [ ] ResourceMatcher configured with supported providers
  - [ ] ResourceMatcher configured with supported resource types

### Resource Support

- [ ] Provider support declared
  - [ ] All supported cloud providers added via `AddProvider()`
  - [ ] Provider names are standard (aws, azure, gcp, kubernetes)

- [ ] Resource type support declared
  - [ ] All supported resource types added via `AddResourceType()`
  - [ ] Resource types follow format: `provider:service:type`
  - [ ] Examples: `aws:ec2:Instance`, `azure:compute:VirtualMachine`

- [ ] Resource matching logic implemented
  - [ ] Check provider and resource type before processing
  - [ ] Return `NotSupportedError()` for unsupported resources
  - [ ] Handle nil or empty resource descriptors gracefully

### Cost Calculation

- [ ] Projected cost calculation
  - [ ] Extract resource properties from tags
  - [ ] Apply pricing logic based on resource configuration
  - [ ] Handle regional pricing variations
  - [ ] Support different billing modes (on-demand, reserved, spot)
  - [ ] Return costs in consistent currency (USD recommended)
  - [ ] Use `CostCalculator.CreateProjectedCostResponse()`

- [ ] Actual cost retrieval (if supported)
  - [ ] Query cost management API
  - [ ] Handle time range parameters
  - [ ] Filter by resource tags/metadata
  - [ ] Return time-series cost data
  - [ ] Use `CostCalculator.CreateActualCostResponse()`

### Error Handling

- [ ] Request validation
  - [ ] Check for nil requests
  - [ ] Check for nil resource descriptors
  - [ ] Validate required fields

- [ ] Error responses
  - [ ] Use `NotSupportedError()` for unsupported resources
  - [ ] Use `NoDataError()` when cost data unavailable
  - [ ] Return descriptive error messages
  - [ ] Include context in error messages

- [ ] Context handling
  - [ ] Respect context cancellation
  - [ ] Propagate context to API calls
  - [ ] Handle timeout scenarios

### Configuration

- [ ] Configuration loading
  - [ ] Read from `~/.finfocus/config.yaml`
  - [ ] Extract plugin-specific section
  - [ ] Validate required fields

- [ ] Credential management
  - [ ] Support API keys
  - [ ] Support OAuth2 tokens (if required)
  - [ ] Handle missing or invalid credentials gracefully

- [ ] Configuration schema documented
  - [ ] Required fields listed
  - [ ] Optional fields listed
  - [ ] Example configuration provided

### Server Setup

- [ ] gRPC server configuration
  - [ ] Use `pluginsdk.Serve()` for server lifecycle
  - [ ] Configure port (0 for auto-select recommended)
  - [ ] Print `PORT=<number>` to stdout

- [ ] Graceful shutdown
  - [ ] Handle SIGINT and SIGTERM signals
  - [ ] Cancel context on signal
  - [ ] Clean up resources before exit

---

## Testing Requirements

### Unit Tests

- [ ] Core functionality tests
  - [ ] Test `Name()` returns correct identifier
  - [ ] Test `GetProjectedCost()` with valid resources
  - [ ] Test `GetProjectedCost()` with unsupported resources
  - [ ] Test `GetActualCost()` if implemented

- [ ] Resource matching tests
  - [ ] Test supported providers
  - [ ] Test unsupported providers
  - [ ] Test supported resource types
  - [ ] Test unsupported resource types

- [ ] Cost calculation tests
  - [ ] Test pricing logic for each resource type
  - [ ] Test regional pricing variations
  - [ ] Test billing mode discounts
  - [ ] Test tag extraction and defaults

- [ ] Error handling tests
  - [ ] Test nil request handling
  - [ ] Test nil resource handling
  - [ ] Test unsupported resource error
  - [ ] Test API error scenarios

- [ ] Configuration tests
  - [ ] Test config loading
  - [ ] Test missing config handling
  - [ ] Test invalid credentials handling

### Integration Tests

- [ ] Plugin lifecycle tests
  - [ ] Test plugin starts successfully
  - [ ] Test plugin responds to gRPC calls
  - [ ] Test graceful shutdown

- [ ] API integration tests
  - [ ] Test API client connectivity
  - [ ] Test API authentication
  - [ ] Test rate limiting
  - [ ] Test retry logic

- [ ] End-to-end tests
  - [ ] Test with FinFocus CLI
  - [ ] Test cost calculation pipeline
  - [ ] Test output formatting

### Test Coverage

- [ ] Minimum coverage requirements
  - [ ] Overall coverage >= 80%
  - [ ] Core functions coverage >= 90%
  - [ ] Error paths tested

- [ ] Test documentation
  - [ ] Test cases documented
  - [ ] Test data fixtures included
  - [ ] Integration test setup documented

---

## Documentation

### Code Documentation

- [ ] Package documentation
  - [ ] Package comment at top of files
  - [ ] Describes plugin purpose and functionality

- [ ] Function documentation
  - [ ] Exported functions have doc comments
  - [ ] Parameters described
  - [ ] Return values described
  - [ ] Examples included for complex functions

- [ ] Type documentation
  - [ ] Exported types documented
  - [ ] Field descriptions provided
  - [ ] Usage examples included

### User Documentation

- [ ] README.md created
  - [ ] Plugin description
  - [ ] Supported cloud providers
  - [ ] Supported resource types
  - [ ] Installation instructions
  - [ ] Configuration guide
  - [ ] Usage examples

- [ ] Configuration guide
  - [ ] Example config.yaml
  - [ ] Required fields documented
  - [ ] Optional fields documented
  - [ ] Authentication setup instructions

- [ ] Troubleshooting guide
  - [ ] Common errors documented
  - [ ] Solutions provided
  - [ ] Debug logging instructions

### API Documentation

- [ ] API integration documented
  - [ ] API endpoints used
  - [ ] Authentication methods
  - [ ] Rate limits
  - [ ] Error codes

---

## Deployment Verification

### Build Process

- [ ] Build configuration
  - [ ] `go.mod` with correct dependencies
  - [ ] Build succeeds on Linux, macOS, Windows
  - [ ] Cross-compilation tested

- [ ] Binary naming
  - [ ] Format: `finfocus-plugin-<name>`
  - [ ] Platform-specific suffixes (.exe for Windows)

### Manifest File

- [ ] `plugin.manifest.yaml` created
  - [ ] Name matches plugin identifier
  - [ ] Version follows semantic versioning
  - [ ] Description is clear and concise
  - [ ] Author field populated
  - [ ] Supported providers listed
  - [ ] Protocols set to `["grpc"]`
  - [ ] Binary path correct

- [ ] Manifest validation
  - [ ] Passes `Manifest.Validate()`
  - [ ] All required fields present
  - [ ] Field values correctly formatted

### Installation Testing

- [ ] Installation structure
  - [ ] Creates `~/.finfocus/plugins/<name>/<version>/`
  - [ ] Binary placed in correct directory
  - [ ] Manifest file placed in correct directory
  - [ ] Binary has execute permissions

- [ ] Installation script (optional)
  - [ ] Detects platform correctly
  - [ ] Downloads correct binary
  - [ ] Sets permissions
  - [ ] Verifies installation

### Integration with FinFocus

- [ ] Plugin discovery
  - [ ] `finfocus plugin list` shows plugin
  - [ ] Version displayed correctly

- [ ] Plugin validation
  - [ ] `finfocus plugin validate` succeeds
  - [ ] No connection errors
  - [ ] gRPC communication working

- [ ] Cost calculation
  - [ ] `finfocus cost projected` uses plugin
  - [ ] Costs calculated correctly
  - [ ] Output format correct

---

## Quality Assurance

### Code Quality

- [ ] Linting
  - [ ] `go fmt` applied
  - [ ] `go vet` passes
  - [ ] `golangci-lint` passes

- [ ] Code review
  - [ ] No hardcoded credentials
  - [ ] No sensitive data logged
  - [ ] Error handling comprehensive
  - [ ] Resource cleanup implemented

### Performance

- [ ] Response times
  - [ ] `GetProjectedCost()` < 100ms for cached data
  - [ ] `GetProjectedCost()` < 1s for API calls
  - [ ] `GetActualCost()` < 5s for typical queries

- [ ] Resource usage
  - [ ] Memory usage reasonable (<100MB typical)
  - [ ] No memory leaks
  - [ ] Goroutine leaks prevented

- [ ] Caching
  - [ ] Pricing data cached appropriately
  - [ ] Cache TTL configured
  - [ ] Cache invalidation working

### Security

- [ ] Credential handling
  - [ ] Credentials loaded from config file
  - [ ] No credentials in logs
  - [ ] No credentials in error messages
  - [ ] Environment variables supported for sensitive data

- [ ] API security
  - [ ] HTTPS used for API calls
  - [ ] Certificate validation enabled
  - [ ] Authentication tokens refreshed

- [ ] Input validation
  - [ ] Resource descriptors validated
  - [ ] Tag values sanitized
  - [ ] No injection vulnerabilities

### Reliability

- [ ] Error recovery
  - [ ] Transient errors retried
  - [ ] Circuit breaker implemented (if needed)
  - [ ] Graceful degradation

- [ ] Logging
  - [ ] Structured logging used
  - [ ] Log levels appropriate
  - [ ] No sensitive data logged

- [ ] Monitoring
  - [ ] Health check implemented (optional)
  - [ ] Metrics exposed (optional)
  - [ ] Request duration tracked

### Release Checklist

- [ ] Version management
  - [ ] Version number incremented
  - [ ] Changelog updated
  - [ ] Git tag created

- [ ] Release artifacts
  - [ ] Binaries built for all platforms
  - [ ] Checksums generated
  - [ ] Release notes written

- [ ] Distribution
  - [ ] GitHub release created
  - [ ] Installation script updated
  - [ ] Documentation published

- [ ] Post-release
  - [ ] Installation tested from release
  - [ ] Integration verified
  - [ ] Announcement sent (if applicable)

---

## Pre-Release Validation

Before releasing your plugin, ensure ALL items are checked:

### Critical Items

- [ ] Plugin implements `Plugin` interface completely
- [ ] All supported providers and resource types declared
- [ ] Unit tests pass with >= 80% coverage
- [ ] Integration tests with FinFocus CLI succeed
- [ ] Documentation complete (README, configuration, troubleshooting)
- [ ] Manifest file valid and complete
- [ ] No hardcoded credentials or sensitive data
- [ ] Cross-platform builds succeed

### Recommended Items

- [ ] Health check implemented
- [ ] Metrics exposed
- [ ] Caching implemented for API calls
- [ ] Rate limiting implemented
- [ ] Retry logic with exponential backoff
- [ ] Comprehensive error messages
- [ ] Installation script provided
- [ ] Example configurations included

### Optional Enhancements

- [ ] Support for multiple cloud providers
- [ ] Advanced filtering capabilities
- [ ] Custom billing modes
- [ ] Cost anomaly detection
- [ ] Budget threshold alerts
- [ ] Dashboard integration

---

## Related Documentation

- [Plugin Development Guide](plugin-development.md) - Complete guide
- [Plugin SDK Reference](plugin-sdk.md) - API documentation
- [Plugin Examples](plugin-examples.md) - Code patterns
- [Plugin Protocol](../architecture/plugin-protocol.md) - gRPC spec
