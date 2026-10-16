# Quickstart: Web Config Page

## See the effective configuration

1. Write `output.precision` `2` in the global `config.hujson` and `4` in the
   project `config.hujson`.
2. Export `FINFOCUS_LOG_LEVEL=debug`.
3. Run `finfocus --web` from the project directory and open Config.
4. Precision shows `4` from the project tier. The logging level shows `debug`
   from the environment. Both stored file values are visible. The logging
   level's environment value is not written when another setting is saved.

## Edit one tier

1. Choose Project. Set precision to `3`. Choose Preview.
2. Confirm the diff lists precision `4` to `3` and that neither file changed
   yet.
3. Confirm the save. The project file changes that one value and keeps its
   comments. The global file is untouched.
4. The page says the running session is unchanged. A new `finfocus config get`
   against that project reports `3`.

## Replace a secret

1. Store `plugins.aws.secret_key` in the global file.
2. Open Config. The row says Set. The page source does not contain the key's
   value.
3. Choose Replace, enter a new value, and save. The file contains the new
   value. Saving again without Replace leaves it in place.
4. `finfocus config get plugins.aws.secret_key` does not print the value.

## Conflict

1. Open Config in two tabs.
2. Save precision from the first tab.
3. Save from the second tab. The second save is rejected and asks for a
   reload. The first tab's value remains.
