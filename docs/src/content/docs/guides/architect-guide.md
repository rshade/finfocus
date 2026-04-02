---
layout: default
title: Architect Guide
description: System design and integration guide for software architects
---

This guide is for **software architects** who need to design and integrate FinFocus into their infrastructure.

## Table of Contents

1. [System Architecture](#system-architecture)
2. [Design Patterns](#design-patterns)
3. [Integration Patterns](#integration-patterns)
4. [Security Considerations](#security-considerations)
5. [Scaling and Performance](#scaling-and-performance)
6. [Deployment Strategies](#deployment-strategies)
7. [Operational Concerns](#operational-concerns)

---

## System Architecture

### Component Diagram

```text
┌──────────────────────────────────────────────────────────────┐
│                        FinFocus CLI                         │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              Command Interface (Cobra)                  │ │
│  │  • cost projected  • cost actual                        │ │
│  │  • plugin list     • plugin validate                    │ │
│  └──────────────────────┬──────────────────────────────────┘ │
└─────────────────────────┼──────────────────────────────────────┘
                          │
                          ▼
         ┌────────────────────────────┐
         │   Cost Calculation Engine  │
         │  ┌──────────────────────┐  │
         │  │ Resource Ingestion   │  │
         │  │ • Pulumi JSON parse  │  │
         │  │ • Resource mapping   │  │
         │  └──────────────────────┘  │
         │  ┌──────────────────────┐  │
         │  │ Cost Queries         │  │
         │  │ • Plugin queries     │  │
         │  │ • Spec fallback      │  │
         │  └──────────────────────┘  │
         │  ┌──────────────────────┐  │
         │  │ Cost Aggregation     │  │
         │  │ • Grouping           │  │
         │  │ • Filtering          │  │
         │  │ • Time-based agg     │  │
         │  └──────────────────────┘  │
         └────────────────────────────┘
              │              │
       ┌──────┘              └─────────┐
       │                                │
       ▼                                ▼
  ┌──────────┐                   ┌──────────┐
  │ Plugins  │                   │  Specs   │
  │ (gRPC)   │                   │ (YAML)   │
  └──────────┘                   └──────────┘
       │
       ▼
  ┌──────────────────────┐
  │ Cost Data Sources    │
  │ • Vantage API        │
  │ • Kubecost API       │
  │ • Future providers   │
  └──────────────────────┘
```

### Data Flow

**Projected Costs:**

```text
User Input → CLI Parser → Engine → Plugin/Spec → Cost Calculation → Output
Pulumi JSON ─────────────────────────────────────────────────────────►
```

**Actual Costs:**

```text
User Input → CLI Parser → Engine → Plugin Query → API Call → Aggregation → Output
Date Range ────────────────────────────────────────────────────────────►
```

---

## Design Patterns

### Plugin Architecture

**Pattern:** External gRPC-based plugins

**Benefits:**

- Language-agnostic: Plugins can be written in any language
- Process isolation: Plugins run in separate processes
- Version independence: Update plugins without rebuilding core
- Failure isolation: Plugin crashes don't crash the CLI

**Implementation:**

- gRPC protocol with Protocol Buffers
- TCP-based communication
- 10-second timeout with retry logic
- Graceful error handling

### Cost Aggregation

**Pattern:** Multi-stage aggregation with filtering

**Stages:**

1. Resource ingestion (from Pulumi JSON)
2. Cost fetching (from plugins/specs)
3. Filtering (by tags, type, etc.)
4. Grouping (by provider, type, date)
5. Aggregation (sum, average, etc.)

**Benefits:**

- Flexible filtering at multiple levels
- Supports complex grouping scenarios
- Efficient processing of large datasets
- Clear separation of concerns

### Fallback Strategy

**Pattern:** Try plugins first, fallback to local specs

```text
┌──────────────────┐
│  Need Cost Data  │
└────────┬─────────┘
         │
         ▼
    ┌────────┐
    │ Plugins│ ← Try first (network, API calls)
    └────┬───┘
         │ If no data
         ▼
    ┌────────┐
    │ Specs  │ ← Fallback (local YAML files)
    └────┬───┘
         │ If no data
         ▼
    ┌──────────────┐
    │ Default/None │ ← Last resort
    └──────────────┘
```

---

## Integration Patterns

### CI/CD Integration

**Pattern:** Cost reports in CI/CD pipeline

```bash
# Pull request workflow
pulumi preview --json > plan.json
finfocus cost projected --pulumi-json plan.json \
  --output json | jq '.summary.totalMonthly'

# If cost > threshold, comment on PR
```

### Cost Tracking

**Pattern:** Periodic cost tracking and alerting

```bash
# Daily cost check (cron job)
finfocus cost actual --group-by daily \
  --output json > cost_report.json

# Alert if cost > budget
# Send to Slack, email, etc.
```

### Cost Attribution

**Pattern:** Tag-based cost allocation

```bash
# Cost by team
finfocus cost actual --filter "tag:team=platform" \
  --group-by "tag:project"

# Cost by environment
finfocus cost actual --filter "tag:env=prod" \
  --group-by provider
```

---

## Security Considerations

### API Credentials

**Challenge:** Secure handling of cloud provider credentials

**Recommendations:**

- Use environment variables (not hardcoded)
- Use IAM roles when available (AWS, Azure, GCP)
- Rotate credentials regularly
- Audit access logs

**Implementation:**

```bash
# Use IAM role (recommended)
export AWS_ROLE_ARN="arn:aws:iam::123456789:role/finfocus"

# Or use temporary credentials
export AWS_ACCESS_KEY_ID="temporary-key"
export AWS_SECRET_ACCESS_KEY="temporary-secret"
export AWS_SESSION_TOKEN="session-token"

finfocus cost actual --from 2024-01-01
```

### Plugin Security

**Challenge:** Running untrusted plugin code

**Mitigations:**

- Plugins run in separate processes (isolation)
- Plugins can only receive/send through gRPC
- No access to host filesystem
- Network access limited to configured APIs

### Data Privacy

**Challenge:** Handling sensitive infrastructure data

**Recommendations:**

- Pulumi JSON contains resource details - treat as sensitive
- Filter output before sharing (JSON/NDJSON allows filtering)
- Store reports securely
- Use encryption in transit (TLS for plugins)

---

## Scaling and Performance

### Resource Handling

**Challenge:** Large infrastructure (1000+ resources)

**Solutions:**

- **Streaming output:** Use NDJSON for large datasets
- **Filtering:** Process subset of resources
- **Caching:** Cache plugin responses (implement in plugins)
- **Pagination:** Process in batches

### Plugin Performance

**Pattern:** Async plugin queries with timeout

**Characteristics:**

- 10-second timeout per plugin call
- Automatic retry with exponential backoff
- Graceful degradation (continue if plugin fails)
- Parallel plugin execution (when applicable)

### Cost Data Volume

**Challenge:** Handling years of historical cost data

**Solutions:**

- **Date filtering:** Limit query ranges
- **Aggregation:** Use daily/monthly grouping
- **Sampling:** Analyze subsets when full data is too large
- **Caching:** Implement caching in plugins

---

## Deployment Strategies

### Local Installation

```bash
# User installs FinFocus binary
# Runs from workstation for ad-hoc cost checks
# No infrastructure required
```

**Pros:** Minimal infrastructure, no setup required
**Cons:** Manual execution, no historical tracking

### CI/CD Pipeline

```bash
# Integrated in GitHub Actions/GitLab CI/Jenkins
# Automatic cost reports on every plan
# PR comments with cost estimates
```

**Pros:** Automated, integrated in workflow
**Cons:** Requires CI/CD setup

### Scheduled Service

```bash
# Cron job or Kubernetes CronJob
# Daily cost reports
# Cost tracking dashboard
```

**Pros:** Automated tracking, continuous monitoring
**Cons:** Infrastructure required, complexity

### Docker Container

```dockerfile
FROM alpine:latest
COPY finfocus /usr/local/bin/
ENTRYPOINT ["finfocus"]
```

**Pros:** Portable, easy deployment
**Cons:** Docker required

---

## Operational Concerns

### Monitoring

**What to monitor:**

- CLI execution time
- Plugin response times
- Plugin failures/timeouts
- Cost calculation anomalies
- API rate limits

### Logging

**Recommended approach:**

- Log all cost queries (for auditing)
- Log plugin interactions (for debugging)
- Structured logging (JSON format)
- Log levels: INFO, WARN, ERROR

### Troubleshooting

**Common issues:**

- Plugin not found → Check plugin installation
- API errors → Check credentials and connectivity
- Timeout errors → Check network, plugin performance
- No cost data → Check filter criteria, data availability

### Alerting

**Set up alerts for:**

- Unexpected cost increases
- Plugin failures
- API errors
- Missing data

---

## Design Trade-offs

| Decision                | Pros                           | Cons                          |
| ----------------------- | ------------------------------ | ----------------------------- |
| **Plugin architecture** | Language-agnostic, isolated    | Complexity, gRPC overhead     |
| **Local fallback**      | Works offline, no dependencies | Limited functionality         |
| **gRPC protocol**       | Efficient, strongly typed      | Learning curve, binary format |
| **Process isolation**   | Security, stability            | Performance overhead          |
| **Flexible grouping**   | Powerful, flexible             | Complex implementation        |

---

## Future Considerations

- **Real-time cost streaming:** Stream costs as infrastructure changes
- **Multi-region support:** Unified cost view across regions
- **ML-based forecasting:** Predict future costs with trends
- **Cost optimization recommendations:** AI-driven suggestions
- **Enterprise features:** RBAC, audit trails, multi-tenancy

---

## Resources

- **System Overview:** [System Architecture Diagram](../architecture/system-overview.md)
- **Plugin Protocol:** [Plugin Protocol Specification](../architecture/plugin-protocol.md)
- **Cost Calculation:** [Cost Calculation Flow](../architecture/cost-calculation.md)
- **Deployment:** [Deployment Guide](../deployment/deployment.md)
- **Security:** [Security Best Practices](../deployment/security.md)

---

**Last Updated:** 2025-10-29
