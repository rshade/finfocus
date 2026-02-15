# Quickstart: Project-Local Configuration

## Setup per-project config (30 seconds)

```bash
# Navigate to your Pulumi project
cd ~/projects/web-app

# Initialize project-local FinFocus config
finfocus config init

# Output:
# Configuration initialized at ./web-app/.finfocus/config.yaml
# Created .gitignore to protect user-specific data
```

## Set a project-specific budget

Edit `.finfocus/config.yaml` in your project root:

```yaml
cost:
  budgets:
    global:
      amount: 5000.00
      currency: USD
      period: monthly
```

## Run cost commands (auto-detects project)

```bash
# From anywhere inside the project
cd ~/projects/web-app/src/handlers/

# Uses project config automatically
finfocus cost projected --pulumi-json plan.json
# → Uses $5,000 budget from project config
# → Inherits output format, logging from global ~/.finfocus/config.yaml
```

## Per-project dismissals

```bash
# In web-app project
cd ~/projects/web-app
finfocus cost recommendations dismiss rec-123 --reason "evaluated"
# → Stored in ./web-app/.finfocus/dismissed.json

# In data-pipeline project
cd ~/projects/data-pipeline
finfocus cost recommendations
# → rec-123 is still visible (different project)
```

## Override project directory

```bash
# Explicit project directory
finfocus cost projected --project-dir /path/to/project --pulumi-json plan.json

# Via environment variable
export FINFOCUS_PROJECT_DIR=/path/to/project
finfocus cost projected --pulumi-json plan.json
```

## Global config (no change from today)

```bash
# Outside any Pulumi project, everything works as before
cd ~/random-dir
finfocus cost projected --pulumi-json plan.json
# → Uses ~/.finfocus/config.yaml only
```

## Precedence Summary

| Setting | Resolution Order |
|---------|-----------------|
| Config | `--project-dir` > `$FINFOCUS_PROJECT_DIR` > Pulumi.yaml walk-up > `~/.finfocus/` |
| Plugins | Always `~/.finfocus/plugins/` (global) |
| Cache | Always `~/.finfocus/cache/` (global) |
| Logs | Always `~/.finfocus/logs/` (global) |
| Dismissals | Same as Config (project-local when available) |
