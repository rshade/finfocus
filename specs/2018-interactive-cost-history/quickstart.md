# Verify interactive history

From a Pulumi project with saved history, open the terminal chart.

```bash
finfocus cost history view --stack dev
```

Pan with the arrows, zoom with plus and minus, and toggle providers with
`s`. Press Tab to cycle the highlighted provider. Press `b` to toggle budget
thresholds, a number key to select a deployment, and `d` to show its details.
Press `q` to quit.

Run the command with `--plain` or redirect stdout to verify the text chart.
Run with `--output json` to verify the structured document.

Run the focused and browser checks from the repository.

```bash
go test ./internal/tui ./internal/history ./internal/cli ./internal/webui -run 'History|Chart'
make test-e2e-web
```
