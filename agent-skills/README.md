# FinFocus Agent Skills

Product-specific agent skills for AI coding assistants (Claude Code, Gemini
CLI, etc.) that automate common FinFocus workflows.

These skills are tightly coupled to FinFocus CLI commands, file paths, and
architecture. `cost-check` and `cost-drift` started in
[rshade/agent-skills](https://github.com/rshade/agent-skills) and now live
here. That repository keeps a stub for each that points back to this
directory. `cost-optimize` and `budget-setup` were planned there but were
never published. [finfocus-budget](finfocus-budget/) covers budgets.

## Available Skills

| Skill | Description | LOE |
|-------|-------------|-----|
| [finfocus-install](finfocus-install/) | Install CLI, detect providers, install plugins, init config | M |
| [finfocus-analyzer-setup](finfocus-analyzer-setup/) | Configure Pulumi Analyzer for inline cost estimation | S |
| [finfocus-routing](finfocus-routing/) | Configure plugin routing with priority, patterns, fallback | S |
| [finfocus-budget](finfocus-budget/) | Configure budget thresholds, health, and CI exit codes | M |
| [finfocus-diagnose](finfocus-diagnose/) | Debug connectivity, config resolution, cache, and zero cost | M |
| [cost-check](cost-check/) | Estimate monthly cost from a plan or state, flag thresholds, write COST_CHECK.md | M |
| [cost-drift](cost-drift/) | Compare actual and projected spend, flag drift, write COST_DRIFT.md | M |
| [finfocus-plugin-upgrade](finfocus-plugin-upgrade/) | Upgrade a plugin project to a newer finfocus-spec version | M |
| [finfocus-plugin-dev](finfocus-plugin-dev/) | Implement and test a plugin scaffolded by `plugin init` | M |
| [plugin-manage](plugin-manage/) | Install, update, remove, validate, and troubleshoot installed plugins | M |

`finfocus plugin init` and `finfocus plugin upgrade` install
`finfocus-plugin-dev` and `finfocus-plugin-upgrade` into the plugin
repository with `npx skills add`, pinned to the finfocus release (`--no-skill`
skips this). To add them by hand:

```bash
npx -y skills@1.7.0 add https://github.com/rshade/finfocus/tree/main/agent-skills \
  --skill finfocus-plugin-dev --skill finfocus-plugin-upgrade \
  --agent codex --agent claude-code --copy -y
```

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
- **Cost workflow skills** (`cost-check`, `cost-drift`) -- Run the FinFocus
  CLI and read its JSON, so they live here and ship with the CLI release they
  were checked against. Each one declares the minimum CLI version it needs.
- **Generic skills** -- Skills that do not depend on FinFocus live in
  [rshade/agent-skills](https://github.com/rshade/agent-skills).
