# Quickstart: Verify Shared Plugin Processes

1. Build: `make build`.
2. From a Pulumi project with at least two stacks (for example initial-30
   with `dev` and `test`), start the dashboard:

   ```bash
   PULUMI_CONFIG_PASSPHRASE='' <repo>/bin/finfocus --web --no-browser \
     --port 8765 --stack dev,test
   ```

3. Open both tabs in the browser so both stacks activate.
4. Count plugin processes: `ps -eo pid,args | grep '[f]infocus-plugin-'`.
   Each installed plugin (and each aws-public region child) appears once, not
   once per stack.
5. Stop the dashboard with Ctrl+C and repeat step 4: nothing remains.
