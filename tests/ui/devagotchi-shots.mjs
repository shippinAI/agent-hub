// Devagotchi UI screenshots at 1280 (desktop) and 390 (phone), driving the running hub with dev auth.
// Usage: node devagotchi-shots.mjs <url> <screenshot-dir> <results-file>
import fs from 'node:fs';
import puppeteer from 'puppeteer-core';
import { check, click, results, shot, sleep, noOverflow } from './lib.mjs';

const [url, out, resFile] = process.argv.slice(2);
const browser = await puppeteer.launch({ executablePath: process.env.CHROME ?? '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });
const errors = [];
const pages = [
  ['machines', 'dev/machines', 'Machines'],
  ['requests', 'dev/requests', 'Access requests'],
  ['grants', 'dev/grants', 'Grants'],
  ['subscriptions', 'dev/subscriptions', 'Subscriptions'],
];
try {
  const d = await browser.newPage();
  d.on('pageerror', (e) => errors.push(String(e)));
  d.on('console', (m) => m.type() === 'error' && errors.push(m.text()));
  await d.setViewport({ width: 1280, height: 900 });
  for (const [name, hash, title] of pages) {
    await d.goto(`${url}/#/${hash}`, { waitUntil: 'networkidle2' });
    await sleep(900);
    check(await d.evaluate((t) => document.body.innerText.includes(t), title), `desktop ${name} renders`, title);
    await shot(d, out, `devagotchi-desktop-1280-${name}`, true);
  }
  // Grants chip in the header
  check(await d.evaluate(() => !!document.querySelector('[data-testid=grants-chip]')), 'header shows the live grants chip');
  // Approve a pending request through the access card
  await d.goto(`${url}/#/dev/requests`, { waitUntil: 'networkidle2' }); await sleep(900);
  const hadCard = await d.evaluate(() => !!document.querySelector('.access-card'));
  if (hadCard) {
    await shot(d, out, 'devagotchi-desktop-1280-access-card', true);
    await click(d, '.access-card button', 'Approve narrower'); await sleep(500);
    await shot(d, out, 'devagotchi-desktop-1280-narrow-editor', true);
  }
  check(hadCard, 'requests inbox shows an access card');

  // Phone viewport
  await d.setViewport({ width: 390, height: 840 });
  for (const [name, hash] of pages) {
    await d.goto(`${url}/#/${hash}`, { waitUntil: 'networkidle2' });
    await sleep(900);
    check(await noOverflow(d, 390), `phone ${name} has no horizontal overflow`);
    await shot(d, out, `devagotchi-phone-390-${name}`, true);
  }
  check(errors.length === 0, 'no console/page errors', errors.slice(0, 3).join(' | '));
} finally {
  await browser.close();
  fs.writeFileSync(resFile, results.join('\n') + '\n');
}
