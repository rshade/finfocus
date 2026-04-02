---
title: Vantage Plugin Authentication
description: API key management and security best practices for the FinFocus Vantage plugin
---

> **Note:** The Vantage plugin is currently **in development** and not yet available for installation.
> This documentation is provided for reference and will be updated when the plugin is released.

This guide explains how to securely configure authentication for the Vantage
plugin, including API key management, credential storage, and security best
practices.

## Table of Contents

1. [Overview](#overview)
2. [Obtaining API Tokens](#obtaining-api-tokens)
3. [Configuration Setup](#configuration-setup)
4. [Environment Variables](#environment-variables)
5. [Secrets Management](#secrets-management)
6. [Token Types](#token-types)
7. [Security Best Practices](#security-best-practices)
8. [Credential Rotation](#credential-rotation)
9. [Troubleshooting Authentication](#troubleshooting-authentication)

---

## Overview

The Vantage plugin requires API authentication to access cost data from
Vantage's REST API. Authentication uses bearer tokens that must be provided via
environment variables or configuration files.

### Authentication Flow

```text
1. Plugin loads configuration (config.yaml)
2. Reads API token from environment variable
3. Attaches bearer token to all API requests
4. Vantage validates token and returns cost data
```

### Security Principles

- **Never hardcode tokens** in configuration files
- **Use environment variables** or secrets management systems
- **Rotate credentials regularly** (every 90 days recommended)
- **Use least-privilege tokens** (Cost Report tokens over Workspace tokens)
- **Monitor token usage** for suspicious activity

---

## Obtaining API Tokens

### Step 1: Log into Vantage Console

1. Navigate to [https://console.vantage.sh](https://console.vantage.sh)
2. Sign in with your Vantage account credentials
3. Navigate to **Settings** → **API Tokens**

### Step 2: Generate New API Token

#### Option A: Create Cost Report Token (Recommended)

1. Go to **Cost Reports** in Vantage console
2. Select the report you want to use
3. Click **API Access** → **Generate Token**
4. Name the token (e.g., `finfocus-production`)
5. Copy the generated token (starts with `cr_`)
6. Store securely (you won't be able to see it again)

#### Option B: Create Workspace Token (Fallback)

1. Go to **Settings** → **API Tokens**
2. Click **Generate New Token**
3. Name the token (e.g., `finfocus-workspace`)
4. Select permissions: **Read-only cost access**
5. Copy the generated token (starts with `ws_`)
6. Store securely

### Step 3: Verify Token Permissions

Test the token has correct permissions:

```bash
# Set token
export FINFOCUS_VANTAGE_TOKEN="your_token_here"

# Test API access
curl -H "Authorization: Bearer $FINFOCUS_VANTAGE_TOKEN" \
  https://api.vantage.sh/costs

# Expected: 200 OK or 400 (bad request params)
# NOT 401 (unauthorized) or 403 (forbidden)
```

---

## Configuration Setup

### Method 1: Environment Variable (Recommended)

Configure the token via environment variable reference in `config.yaml`:

```yaml
version: 0.1
source: vantage

credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN} # Reference env var

params:
  cost_report_token: 'cr_abc123def456'
  granularity: 'day'
```

Set the environment variable:

```bash
export FINFOCUS_VANTAGE_TOKEN="your_actual_token_value"
```

### Method 2: Direct Configuration (Development Only)

**WARNING:** Only use for local development. Never commit tokens to version
control.

```yaml
version: 0.1
source: vantage

credentials:
  token: 'vantage_token_value_here' # Direct value (NOT RECOMMENDED)

params:
  cost_report_token: 'cr_abc123def456'
  granularity: 'day'
```

### Method 3: Multiple Environment Variables

Configure different token types:

```yaml
version: 0.1
source: vantage

credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}

params:
  # Use env var for cost report token too
  cost_report_token: ${FINFOCUS_VANTAGE_COST_REPORT_TOKEN}
  granularity: 'day'
```

Set both variables:

```bash
export FINFOCUS_VANTAGE_TOKEN="vantage_api_token"
export FINFOCUS_VANTAGE_COST_REPORT_TOKEN="cr_abc123def456"
```

---

## Environment Variables

### Standard Environment Variables

The plugin supports these environment variables:

| Variable                             | Purpose        | Format | Example           |
| ------------------------------------ | -------------- | ------ | ----------------- |
| `FINFOCUS_VANTAGE_TOKEN`             | Main API token | String | `vantage_3f4g...` |
| `FINFOCUS_VANTAGE_COST_REPORT_TOKEN` | Cost Report    | `cr_*` | `cr_abc123`       |
| `FINFOCUS_VANTAGE_WORKSPACE_TOKEN`   | Workspace      | `ws_*` | `ws_xyz789`       |

### Setting Environment Variables

**Bash/Zsh:**

```bash
export FINFOCUS_VANTAGE_TOKEN="your_token"

# Persist in shell profile
echo 'export FINFOCUS_VANTAGE_TOKEN="your_token"' >> ~/.bashrc
source ~/.bashrc
```

**Fish Shell:**

```fish
set -Ux FINFOCUS_VANTAGE_TOKEN "your_token"
```

**Windows PowerShell:**

```powershell
$env:FINFOCUS_VANTAGE_TOKEN="your_token"

# Persist for user
[Environment]::SetEnvironmentVariable("FINFOCUS_VANTAGE_TOKEN", "your_token", "User")
```

### Verifying Environment Variables

```bash
# Check if set
echo $FINFOCUS_VANTAGE_TOKEN

# Should output your token (not empty)
# If empty, token not set
```

---

## Secrets Management

### AWS Secrets Manager

Store Vantage tokens in AWS Secrets Manager:

```bash
# Store secret
aws secretsmanager create-secret \
  --name finfocus/vantage/token \
  --secret-string "your_vantage_token"

# Retrieve and use
export FINFOCUS_VANTAGE_TOKEN=$(aws secretsmanager get-secret-value \
  --secret-id finfocus/vantage/token \
  --query SecretString \
  --output text)

# Run plugin
finfocus-vantage pull --config config.yaml
```

### HashiCorp Vault

Store tokens in Vault:

```bash
# Store secret
vault kv put secret/finfocus/vantage token="your_vantage_token"

# Retrieve and use
export FINFOCUS_VANTAGE_TOKEN=$(vault kv get -field=token secret/finfocus/vantage)

# Run plugin
finfocus-vantage pull --config config.yaml
```

### Kubernetes Secrets

Store as Kubernetes secret for containerized deployments:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: finfocus-vantage
  namespace: default
type: Opaque
stringData:
  token: 'your_vantage_token'
  cost_report_token: 'cr_abc123def456'
```

Reference in pod:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: finfocus-vantage-sync
spec:
  schedule: '0 2 * * *'
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: vantage-sync
              image: finfocus-vantage:latest
              env:
                - name: FINFOCUS_VANTAGE_TOKEN
                  valueFrom:
                    secretKeyRef:
                      name: finfocus-vantage
                      key: token
                - name: FINFOCUS_VANTAGE_COST_REPORT_TOKEN
                  valueFrom:
                    secretKeyRef:
                      name: finfocus-vantage
                      key: cost_report_token
```

### Docker Secrets

For Docker Swarm deployments:

```bash
# Create secret
echo "your_vantage_token" | docker secret create vantage_token -

# Use in service
docker service create \
  --name finfocus-vantage \
  --secret vantage_token \
  --env FINFOCUS_VANTAGE_TOKEN_FILE=/run/secrets/vantage_token \
  finfocus-vantage:latest
```

---

## Token Types

### Cost Report Token (Preferred)

**Format:** `cr_` followed by alphanumeric characters

**Characteristics:**

- Scoped to specific cost report
- Predefined filters and grouping
- Better performance (smaller dataset)
- More secure (narrower scope)
- Recommended for production

**Use Cases:**

- Production deployments
- Automated scheduled syncs
- Team-specific cost reports
- Compliance-sensitive environments

**Example Configuration:**

```yaml
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}

params:
  cost_report_token: 'cr_abc123def456' # Preferred
  granularity: 'day'
```

### Workspace Token (Fallback)

**Format:** `ws_` followed by alphanumeric characters

**Characteristics:**

- Broad access to all workspace data
- Requires additional filtering
- Less performant (larger dataset)
- Use when Cost Report tokens unavailable

**Use Cases:**

- Initial testing and evaluation
- Ad-hoc queries across multiple reports
- Development environments
- Exploratory analysis

**Example Configuration:**

```yaml
credentials:
  token: ${FINFOCUS_VANTAGE_TOKEN}

params:
  workspace_token: 'ws_xyz789ghi012' # Fallback
  granularity: 'day'
```

---

## Security Best Practices

### Do's ✅

1. **Use Environment Variables**

   ```bash
   export FINFOCUS_VANTAGE_TOKEN="your_token"
   ```

2. **Prefer Cost Report Tokens**
   - Narrowest scope principle
   - Better security posture

3. **Rotate Tokens Regularly**
   - Every 90 days recommended
   - Immediately upon suspected compromise

4. **Use Secrets Management Systems**
   - AWS Secrets Manager
   - HashiCorp Vault
   - Kubernetes Secrets

5. **Restrict Token Permissions**
   - Read-only cost access
   - No write or admin permissions

6. **Monitor Token Usage**
   - Review API access logs
   - Alert on suspicious patterns
   - Track token activity

7. **Use Different Tokens Per Environment**

   ```bash
   # Production
   export FINFOCUS_VANTAGE_TOKEN="$PROD_TOKEN"

   # Staging
   export FINFOCUS_VANTAGE_TOKEN="$STAGING_TOKEN"

   # Development
   export FINFOCUS_VANTAGE_TOKEN="$DEV_TOKEN"
   ```

### Don'ts ❌

1. **Never Hardcode Tokens**

   ```yaml
   # BAD - Don't do this
   credentials:
     token: 'vantage_actual_token_value'
   ```

2. **Never Commit Tokens to Git**

   ```bash
   # Add to .gitignore
   echo "*.token" >> .gitignore
   echo "*.secret" >> .gitignore
   echo "config.yaml" >> .gitignore  # If contains tokens
   ```

3. **Never Log Token Values**

   ```bash
   # BAD - Tokens may leak in logs
   echo "Token: $FINFOCUS_VANTAGE_TOKEN"
   ```

4. **Never Share Tokens via Email/Chat**
   - Use secrets management systems
   - Share securely via 1Password/LastPass
   - Generate new token for recipient

5. **Never Use Workspace Tokens When Cost Report Available**
   - Prefer narrower scope
   - Better security and performance

6. **Never Reuse Tokens Across Environments**
   - Separate tokens for dev/staging/prod
   - Limits blast radius of compromise

---

## Credential Rotation

### Rotation Schedule

**Recommended Frequency:**

- **Production:** Every 90 days
- **Staging:** Every 180 days
- **Development:** Every 365 days or on team member departure

### Rotation Procedure

#### Step 1: Generate New Token

1. Log into Vantage console
2. Navigate to **Settings** → **API Tokens**
3. Generate new token with same permissions
4. Name it with rotation date (e.g., `finfocus-2024-Q1`)

#### Step 2: Update Secrets Management

```bash
# AWS Secrets Manager
aws secretsmanager update-secret \
  --secret-id finfocus/vantage/token \
  --secret-string "new_token_value"

# HashiCorp Vault
vault kv put secret/finfocus/vantage token="new_token_value"

# Kubernetes
kubectl create secret generic finfocus-vantage \
  --from-literal=token="new_token_value" \
  --dry-run=client -o yaml | kubectl apply -f -
```

#### Step 3: Verify New Token

```bash
# Test new token
export FINFOCUS_VANTAGE_TOKEN="new_token_value"
curl -H "Authorization: Bearer $FINFOCUS_VANTAGE_TOKEN" \
  https://api.vantage.sh/costs

# Expected: 200 OK
```

#### Step 4: Deploy Updated Configuration

```bash
# Restart services using the token
systemctl restart finfocus-vantage

# Or for Kubernetes
kubectl rollout restart deployment/finfocus-vantage
```

#### Step 5: Revoke Old Token

1. Log into Vantage console
2. Navigate to **Settings** → **API Tokens**
3. Find old token
4. Click **Revoke**
5. Confirm revocation

#### Step 6: Verify Services Still Working

```bash
# Check logs for auth errors
journalctl -u finfocus-vantage -n 50

# Or for Kubernetes
kubectl logs -l app=finfocus-vantage --tail=50
```

### Emergency Rotation

If token compromised, rotate immediately:

1. **Generate new token** (Step 1 above)
2. **Update secrets** (Step 2 above)
3. **Revoke compromised token immediately**
4. **Deploy updated configuration**
5. **Review access logs** for unauthorized access
6. **Notify security team** if breach detected

---

## Troubleshooting Authentication

### Error: 401 Unauthorized

**Symptoms:**

```text
Error: 401 Unauthorized
Failed to authenticate with Vantage API
```

**Causes:**

- Token not set or empty
- Token expired or revoked
- Token lacks required permissions

**Solutions:**

1. Verify token is set:

   ```bash
   echo $FINFOCUS_VANTAGE_TOKEN
   ```

2. Test token validity:

   ```bash
   curl -H "Authorization: Bearer $FINFOCUS_VANTAGE_TOKEN" \
     https://api.vantage.sh/costs
   ```

3. Regenerate token in Vantage console

### Error: 403 Forbidden

**Symptoms:**

```text
Error: 403 Forbidden
Insufficient permissions to access cost data
```

**Causes:**

- Token has wrong permissions
- Cost Report Token doesn't have access to specified report
- Workspace Token doesn't have cost access

**Solutions:**

1. Verify token permissions in Vantage console
2. Ensure token has **Read-only cost access**
3. For Cost Report Token, verify report access
4. Generate new token with correct permissions

### Error: Token Not Found in Environment

**Symptoms:**

```text
Error: FINFOCUS_VANTAGE_TOKEN environment variable not set
```

**Solutions:**

1. Set environment variable:

   ```bash
   export FINFOCUS_VANTAGE_TOKEN="your_token"
   ```

2. Verify it's set:

   ```bash
   echo $FINFOCUS_VANTAGE_TOKEN
   ```

3. Ensure it persists across sessions:

   ```bash
   echo 'export FINFOCUS_VANTAGE_TOKEN="your_token"' >> ~/.bashrc
   source ~/.bashrc
   ```

### Error: Invalid Token Format

**Symptoms:**

```text
Error: Invalid token format
```

**Causes:**

- Token contains whitespace or newlines
- Token truncated or incomplete
- Wrong token type provided

**Solutions:**

1. Verify token format:

   ```bash
   # Cost Report Token should start with cr_
   echo $FINFOCUS_VANTAGE_TOKEN | grep "^cr_"

   # Workspace Token should start with ws_
   echo $FINFOCUS_VANTAGE_TOKEN | grep "^ws_"
   ```

2. Ensure no whitespace:

   ```bash
   export FINFOCUS_VANTAGE_TOKEN=$(echo "your_token" | tr -d '[:space:]')
   ```

---

## Additional Resources

- [Vantage API Documentation](https://docs.vantage.sh/api)
- [Vantage Security Best Practices](https://docs.vantage.sh/security)
- [Setup Guide](setup.md) - Installation and configuration
- [Features Guide](features.md) - Supported capabilities
- [Troubleshooting Guide](troubleshooting.md) - Common issues
