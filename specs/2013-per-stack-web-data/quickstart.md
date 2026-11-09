# Quickstart: Verifying Per-Stack-Safe Web Data

```bash
go test -race -shuffle=on -count=3 -run 'SharedStores|StackESC|PassphraseRelay|ListStacks|ValidateStacks' \
  ./internal/cli/ ./internal/pulumi/
make validate && make test && make lint
```

Regression proof: on `origin/main`, the two-engine test fails because the
second engine has a nil cache and history. Check it by running the new test
against the old `newEngineWithCacheAndHistory` wiring.

Single-stack smoke (no behavior change):

```bash
make build
cd ../finfocus-demo
PULUMI_CONFIG_PASSPHRASE='' FINFOCUS_LOG_LEVEL=debug \
  ../finfocus/bin/finfocus --web --no-browser --port 18080
```

Expect one "history store initialized" and no "proceeding without cache" or
"proceeding without history" warning. Stop the server with an anchored
`pkill -f '^<path>/finfocus --web'`.
