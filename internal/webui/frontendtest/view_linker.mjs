import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

// Links a view module against the given app module and the real shared spot helper.
export async function linkView(mod, app, context) {
  const spot = new vm.SourceTextModule(await readFile(new URL('../static/spot.js', import.meta.url), 'utf8'), { context });
  await spot.link(() => app);
  await mod.link((specifier) => (specifier.endsWith('spot.js') ? spot : app));
}
