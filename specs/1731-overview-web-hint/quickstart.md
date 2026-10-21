# Quickstart: Verifying the Overview Web Hint

Run these from the repository root after implementation. The TUI waits for
input, so wrap every interactive run in `timeout`.

## Automated

```bash
go test -run 'TestGolden|TestOverview' ./internal/tui/...
go test -run 'Overview|Web' ./internal/cli/
make test
make lint
```

## Interactive (real terminal)

Use a populated project (`../finfocus-demo`, empty passphrase).

```bash
make build
cd ../finfocus-demo
PULUMI_CONFIG_PASSPHRASE='' timeout 60 ../finfocus/bin/finfocus
```

Check, in order:

1. The footer shows `[w] web dashboard · finfocus --web` under the status
   line. Resize the terminal narrow: the line shortens to `[w] web`, then
   disappears, and never wraps.
2. Press `/`, type `w`: the character lands in the filter box and nothing
   else happens. Press Esc.
3. Press `w`: the screen clears, the web URL prints, and a browser opens.
4. Press Ctrl+C: the server stops. Then confirm no orphans:

   ```bash
   ps -eo pid,args | grep '[f]infocus-plugin'
   ```

   The output is empty.

## Non-interactive (must be unchanged)

```bash
cd ../finfocus-demo
PULUMI_CONFIG_PASSPHRASE='' ../finfocus/bin/finfocus </dev/null | cat
PULUMI_CONFIG_PASSPHRASE='' AGENT_MODE=1 ../finfocus/bin/finfocus --plain
```

Both print the plain table, contain no `[w]` text, and exit 0 without waiting
for input. Compare against the output of the previous release binary for byte
equality.

## Outside a project

```bash
cd "$(mktemp -d)" && /path/to/finfocus/bin/finfocus </dev/null | head -3
```

Prints help.
