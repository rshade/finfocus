# FinFocus Agent Skills

Product-specific agent skills for AI coding assistants (Claude Code, Gemini
CLI, etc.) that automate common FinFocus workflows.

These skills are tightly coupled to FinFocus CLI commands, file paths, and
architecture. Generic cost workflow skills (cost-check, cost-drift,
cost-optimize, budget-setup) live in
[rshade/agent-skills](https://github.com/rshade/agent-skills).

## Available Skills

| Skill | Description | LOE |
|-------|-------------|-----|
| [finfocus-install](finfocus-install/) | Install CLI, detect providers, install plugins, init config | M |
| [finfocus-analyzer-setup](finfocus-analyzer-setup/) | Configure Pulumi Analyzer for inline cost estimation | S |
| [finfocus-routing](finfocus-routing/) | Configure plugin routing with priority, patterns, fallback | S |

## Planned Skills

| Skill | Issue | LOE |
|-------|-------|-----|
| plugin-manage | [#911](https://github.com/rshade/finfocus/issues/911) | M |
| finfocus-diagnose | [#913](https://github.com/rshade/finfocus/issues/913) | M |
| finfocus-budget | [#914](https://github.com/rshade/finfocus/issues/914) | M |

## Skill Structure

Each skill follows the SKILL.md + references/ format:

```text
<skill-name>/
├── SKILL.md              # Core workflow (triggers, steps, commands)
├── <skill-name>.skill    # Packaged distributable
└── references/           # Detailed reference material (loaded on demand)
```

## Placement Policy

- **Tool-specific skills** (this directory) -- Coupled to FinFocus CLI
  commands, config paths, and gRPC protocol. Live in `rshade/finfocus`.
- **Generic cost workflow skills** -- Multi-tool skills not tied to any
  specific cost tool. Live in
  [rshade/agent-skills](https://github.com/rshade/agent-skills).
