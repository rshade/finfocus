# Quickstart: Cost Anomalies Command

```bash
make build
./bin/finfocus cost anomalies --pulumi-state state.json --output json
./bin/finfocus cost anomalies --stack dev --filter provider=aws
./bin/finfocus cost anomalies --pulumi-state state.json --output nope   # exit 2
```

Verify: JSON has `considered` and `anomalies`; an empty stack prints
`{"considered":0,"anomalies":[]}` and exits 0.
Web: `finfocus --web --no-browser --port 8123`, then
`GET /api/overview/anomalies` with the session cookie.
