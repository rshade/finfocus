import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const spotUrl = new URL('../static/spot.js', import.meta.url).href;
const appUrl = new URL('../static/app.js', import.meta.url).href;
const chartUrl = new URL('../static/history-chart.js', import.meta.url).href;
const historyUrl = new URL('../static/history.js', import.meta.url).href;

// Link a view against its production helpers and the test's app stub.
// Require exact relative import targets, as the browser does.
export async function linkView(mod, app, context, view) {
  const viewUrl = new URL(`../static/views/${view}.js`, import.meta.url);
  const spot = new vm.SourceTextModule(await readFile(new URL(spotUrl), 'utf8'), { context });
  await spot.link((specifier) => {
    const target = new URL(specifier, spotUrl).href;
    if (target !== appUrl) throw new Error(`spot.js imports unexpected module ${specifier}`);
    return app;
  });
  const chart = new vm.SourceTextModule(await readFile(new URL(chartUrl), 'utf8'), { context });
  await chart.link((specifier) => {
    if (new URL(specifier, chartUrl).href !== appUrl) throw new Error(`history-chart.js imports unexpected module ${specifier}`);
    return app;
  });
  const history = new vm.SourceTextModule(await readFile(new URL(historyUrl), 'utf8'), { context });
  await history.link((specifier) => {
    const target = new URL(specifier, historyUrl).href;
    if (target === appUrl) return app;
    if (target === chartUrl) return chart;
    throw new Error(`history.js imports unexpected module ${specifier}`);
  });
  await mod.link((specifier) => {
    const target = new URL(specifier, viewUrl).href;
    if (target === spotUrl) return spot;
    if (target === appUrl) return app;
    if (target === historyUrl) return history;
    throw new Error(`${view}.js imports unresolvable module ${specifier}`);
  });
}
