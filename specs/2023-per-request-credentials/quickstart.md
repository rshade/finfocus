# Quickstart

```hujson
// config.hujson
{
  "esc": {
    "plugins": {
      "aws-ce": {
        "credentials": {
          "access_key_id": "AWS_ACCESS_KEY_ID",
          "secret_access_key": "AWS_SECRET_ACCESS_KEY",
          "session_token": "AWS_SESSION_TOKEN"
        }
      }
    }
  }
}
```

1. Run `finfocus --web --stack dev,prod` from a project whose stacks import
   different credentials.
2. Open both tabs. One `finfocus-plugin-aws-ce` process runs.
3. Remove `credentials` (list the variables under `variables` instead) and
   repeat: two processes run.

```bash
go test -race -shuffle=on -count=3 ./internal/pluginhost/... ./internal/esc/... ./internal/config/...
```
