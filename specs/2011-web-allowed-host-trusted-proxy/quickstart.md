# Quickstart

```bash
# token flow through Tailscale Serve (User Story 1)
finfocus --web --no-browser --port 8765 --allowed-host dash.example.ts.net
tailscale serve --bg --https=443 http://127.0.0.1:8765
# open https://dash.example.ts.net/?token=<printed token>

# identity flow (User Story 2)
finfocus --web --no-browser --port 8765 \
  --allowed-host dash.example.ts.net \
  --trusted-proxy --allowed-user alice@example.com
# open https://dash.example.ts.net/ from a tailnet device signed in as alice
```

Verify the refusals:

```bash
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: other.example' http://127.0.0.1:8765/
# 403
```

Tailscale Funnel traffic carries no identity header: use the token URL.
