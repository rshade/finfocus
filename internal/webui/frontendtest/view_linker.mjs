import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const spotUrl = new URL('../static/spot.js', import.meta.url).href;
const appUrl = new URL('../static/app.js', import.meta.url).href;
const historyUrl = new URL('../static/history.js', import.meta.url).href;

// Links a view module against the given app module and the real shared spot helper.
// Each import specifier is resolved against the view's own URL and must hit one of
// those two files exactly; anything else throws, as a 404 would in the browser.
export async function linkView(mod, app, context, view) {
  const viewUrl = new URL(`../static/views/${view}.js`, import.meta.url);
  const spot = new vm.SourceTextModule(await readFile(new URL(spotUrl), 'utf8'), { context });
  await spot.link((specifier) => {
    const target = new URL(specifier, spotUrl).href;
    if (target !== appUrl) throw new Error(`spot.js imports unexpected module ${specifier}`);
    return app;
  });
  const history = new vm.SourceTextModule(await readFile(new URL(historyUrl), 'utf8'), { context });
  await history.link((specifier) => {
    const target = new URL(specifier, historyUrl).href;
    if (target !== appUrl) throw new Error(`history.js imports unexpected module ${specifier}`);
    return app;
  });
  await mod.link((specifier) => {
    const target = new URL(specifier, viewUrl).href;
    if (target === spotUrl) return spot;
    if (target === appUrl) return app;
    if (target === historyUrl) return history;
    throw new Error(`${view}.js imports unresolvable module ${specifier}`);
  });
}
