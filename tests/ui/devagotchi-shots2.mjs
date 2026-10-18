// Screenshots for the newer devagotchi screens (guides, GUI sessions, role bindings, catalog) at 1280
// and 390. Usage: node devagotchi-shots2.mjs <url> <screenshot-dir> <results-file>
import fs from 'node:fs';
import puppeteer from 'puppeteer-core';
import { check, click, results, shot, sleep, noOverflow } from './lib.mjs';

const [url, out, resFile] = process.argv.slice(2);
const sid = await (await fetch(`${url}/api/v1/gui-sessions?live=true`)).json().then((a) => a[0]?.id ?? '');
const pages = [
  ['guides', 'dev/guides', 'Guides'],
  ['guide-editor', 'dev/guide/local-enroll-machine', 'Add a machine'],
  ['guide-updates', 'dev/guide-updates', 'Guide updates'],
  ['gui-sessions', 'dev/gui-sessions', 'GUI sessions'],
  ['gui-session', `dev/gui-session/${sid}`, 'GUI session'],
  ['role-bindings', 'dev/role-bindings', 'Role bindings'],
  ['catalog', 'settings/catalog', 'Catalog'],
];
const browser = await puppeteer.launch({ executablePath: process.env.CHROME ?? '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });
const errors = [];
try {
  const d = await browser.newPage();
  d.on('pageerror', (e) => errors.push(String(e)));
  d.on('console', (m) => m.type() === 'error' && errors.push(m.text()));
  await d.setViewport({ width: 1280, height: 950 });
  for (const [name, hash, title] of pages) {
    await d.goto(`${url}/#/${hash}`, { waitUntil: 'networkidle2' });
    await sleep(1000);
    check(await d.evaluate((t) => document.body.innerText.includes(t), title), `desktop ${name} renders`, title);
    await shot(d, out, `devagotchi-desktop-1280-${name}`, true);
  }
  // Guide editor tabs: Versions (publish), Preview (runner), JSON
  await d.goto(`${url}/#/dev/guide/local-enroll-machine`, { waitUntil: 'networkidle2' }); await sleep(900);
  await click(d, '.tab', 'Versions'); await sleep(500);
  check(await d.evaluate(() => document.body.innerText.includes('Publish') || document.body.innerText.includes('draft')), 'guide editor Versions tab shows a draft to publish');
  await shot(d, out, 'devagotchi-desktop-1280-guide-versions', true);
  // Preview needs a published version, so demo it on the stable shippin guide.
  await d.goto(`${url}/#/dev/guide/enroll-machine`, { waitUntil: 'networkidle2' }); await sleep(900);
  await click(d, '.tab', 'Preview'); await sleep(1500);
  check(await d.evaluate(() => !!document.querySelector('.runner')), 'guide editor Preview renders the runner side panel');
  await shot(d, out, 'devagotchi-desktop-1280-guide-preview', true);
  // Guide updates: open the diff
  await d.goto(`${url}/#/dev/guide-updates`, { waitUntil: 'networkidle2' }); await sleep(900);
  check(await d.evaluate(() => document.body.innerText.includes('Accept')), 'guide updates inbox offers Accept/Merge/Skip');

  // Phone viewport
  await d.setViewport({ width: 390, height: 860 });
  for (const [name, hash] of pages) {
    await d.goto(`${url}/#/${hash}`, { waitUntil: 'networkidle2' });
    await sleep(1000);
    check(await noOverflow(d, 390), `phone ${name} has no horizontal overflow`);
    await shot(d, out, `devagotchi-phone-390-${name}`, true);
  }
  check(errors.length === 0, 'no console/page errors', errors.slice(0, 3).join(' | '));
} finally {
  await browser.close();
  fs.writeFileSync(resFile, results.join('\n') + '\n');
}
