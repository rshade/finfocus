# Quickstart: Multi-Stack Web Dashboard

```bash
make build
cd ../finfocus-demo   # a Pulumi project with stacks dev and test
PULUMI_CONFIG_PASSPHRASE='' ../finfocus/bin/finfocus --web --no-browser --port 8765 --stack=dev,test
```

1. Open the printed URL. Two tabs appear; `dev` is selected and loads.
2. Select `test`. The address becomes `#/s/test/overview`; the header shows
   `test`; only now does `test` start its plugins.
3. `--stack dev` or no `--stack`: no tab bar.
4. `--stack dev,nope`: exit 2, listing the project's stacks.
5. `--stack dev,test --pulumi-json plan.json`: exit 2.
