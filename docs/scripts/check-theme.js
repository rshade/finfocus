import { existsSync } from 'node:fs';

const tokens = new URL('../theme/tokens.css', import.meta.url);

if (!existsSync(tokens)) {
  console.error(
    'ERROR: docs/theme submodule missing; run: git submodule update --init --recursive'
  );
  process.exit(1);
}
