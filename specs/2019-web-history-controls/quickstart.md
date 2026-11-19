# Verify the browser chart

Start the dashboard from a project with saved checkpoint history.

```bash
finfocus --web
```

On **Cost**, open **Cost history**. Zoom in, pan, and reset. Toggle the provider
and budget lines. Select a marker and check the annotation and comparison.
Repeat with the keyboard using the deployment buttons below the chart.

Run the frontend and Chromium tests from the repository root.

```bash
make test-frontend
make test-e2e-web
```
