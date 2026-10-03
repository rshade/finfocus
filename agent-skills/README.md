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
| [finfocus-budget](finfocus-budget/) | Configure budget thresholds, health, and CI exit codes | M |
| [finfocus-diagnose](finfocus-diagnose/) | Debug connectivity, config resolution, cache, and zero cost | M |
| [finfocus-plugin-upgrade](finfocus-plugin-upgrade/) | Upgrade a plugin project to a newer finfocus-spec version | M |
| [finfocus-plugin-dev](finfocus-plugin-dev/) | Implement and test a plugin scaffolded by `plugin init` | M |

`finfocus plugin init` and `finfocus plugin upgrade` install
`finfocus-plugin-dev` and `finfocus-plugin-upgrade` into the plugin
repository with `npx skills add`, pinned to the finfocus release (`--no-skill`
skips this). To add them by hand:

```bash
npx -y skills@1.7.0 add https://github.com/rshade/finfocus/tree/main/agent-skills \
  --skill finfocus-plugin-dev --skill finfocus-plugin-upgrade \
  --agent codex --agent claude-code --copy -y
```

## Planned Skills

| Skill | Issue | LOE |
|-------|-------|-----|
| plugin-manage | [#911](https://github.com/rshade/finfocus/issues/911) | M |

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
