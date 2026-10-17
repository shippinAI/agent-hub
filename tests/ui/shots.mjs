// Drives the real UI in headless Chrome at 1280px and 390px: screenshots + interaction checks.
// Usage: node shots.mjs <url> <screenshot-dir> <tokens-dir> <results-file>
import fs from 'node:fs';
import puppeteer from 'puppeteer-core';
import { api, check, click, noOverflow, results, shot, sleep } from './lib.mjs';
import { mobileRun } from './mobile.mjs';

const [url, out, toks, resFile] = process.argv.slice(2);
const jesse = api(url, `${toks}/jesse.token`);
const codex = api(url, `${toks}/codex.token`);
const browser = await puppeteer.launch({ executablePath: process.env.CHROME ?? '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });
const errors = [];
try {
  const d = await browser.newPage();
  d.on('pageerror', (e) => errors.push(String(e)));
  d.on('console', (m) => m.type() === 'error' && errors.push(m.text()));
  await d.setViewport({ width: 1280, height: 900 });
  await d.goto(url, { waitUntil: 'networkidle2' });
  await sleep(1200);
  await shot(d, out, 'desktop-1280-01-board');
  const lease = await d.evaluate(() => [...document.querySelectorAll('.lease-text')].map((e) => e.textContent));
  check(lease.some((t) => /\d+m \d{2}s/.test(t)), 'board shows live lease countdowns', lease.join(' | '));
  const holders = await d.evaluate(() => [...document.querySelectorAll('.holder')].map((e) => e.textContent));
  check(holders.includes('Codex CLI') && holders.includes('Claude Code'), 'board shows claim owners', holders.join(', '));
  const t1 = await d.evaluate(() => document.querySelector('.lease-text')?.textContent);
  await sleep(2100);
  const t2 = await d.evaluate(() => document.querySelector('.lease-text')?.textContent);
  check(t1 !== t2, 'countdown ticks without reload', `${t1} -> ${t2}`);
  await codex('POST', '/tasks', { project: 'general', title: 'Live SSE arrival check', labels: ['sse'] });
  await sleep(1800);
  check(await d.evaluate(() => document.body.innerText.includes('Live SSE arrival check')), 'new task appears live via SSE (no reload)');

  await d.goto(`${url}/#/task/2`, { waitUntil: 'networkidle2' }); await sleep(800);
  await shot(d, out, 'desktop-1280-02-task-handoff-thread', true);
  await d.type('textarea', 'Reviewed from the UI: go ahead with the SDK client test.');
  await click(d, 'button', 'Add note'); await sleep(800);
  const t2d = await jesse('GET', '/tasks/2');
  check(t2d.notes.some((n) => n.author === 'jesse' && n.body.startsWith('Reviewed from the UI')), 'note added through the UI is stored');
  await shot(d, out, 'desktop-1280-03-note-added', true);

  await d.goto(`${url}/#/task/3`, { waitUntil: 'networkidle2' }); await sleep(800);
  await click(d, 'button', 'Force release'); await sleep(500);
  await d.type('[role=alertdialog] textarea', 'Claude Code session crashed; freeing the UI task.');
  await shot(d, out, 'desktop-1280-04-force-release-dialog');
  await click(d, '[role=alertdialog] button', 'Force release'); await sleep(1000);
  const t3 = await jesse('GET', '/tasks/3');
  check(t3.task.holder === null && t3.events.some((e) => e.action === 'task.force_released'), 'force release via alertdialog clears holder + audits it');
  await shot(d, out, 'desktop-1280-05-after-force-release');

  await d.goto(`${url}/#/agents`, { waitUntil: 'networkidle2' }); await sleep(800);
  await shot(d, out, 'desktop-1280-06-agents', true);
  const cards = await d.$$('.agent');
  await (await cards[2].$('button:nth-of-type(2)')).click(); await sleep(600);
  await d.type('[role=dialog] input', 'laptop');
  await click(d, '[role=dialog] button', 'Issue'); await sleep(800);
  check(await d.evaluate(() => document.body.innerText.includes('shown only once')), 'token issued from UI is shown once');
  await shot(d, out, 'desktop-1280-07-token-issued-masked');
  await d.keyboard.press('Escape'); await sleep(400);
  await click(d, 'button', 'Connect'); await sleep(500);
  await shot(d, out, 'desktop-1280-08-connect-snippets');
  await d.keyboard.press('Escape');

  for (const [tab, n] of [['policies', '09'], ['workflows', '10'], ['lease', '11']]) {
    await d.goto(`${url}/#/settings/${tab}`, { waitUntil: 'networkidle2' }); await sleep(800);
    await shot(d, out, `desktop-1280-${n}-settings-${tab}`, true);
  }
  const knob = await d.$$('.knob');
  for (const k of knob) {
    if ((await k.evaluate((e) => e.textContent)).includes('wip_limit_default')) {
      const inp = await k.$('input');
      await inp.click(); await d.keyboard.down('Control'); await d.keyboard.press('a'); await d.keyboard.up('Control');
      await d.keyboard.press('Backspace'); await inp.type('3');
      await (await k.$('button')).click(); await sleep(800);
    }
  }
  const cfg = await jesse('GET', '/config');
  check(cfg.settings.find((s) => s.key === 'wip_limit_default').value === 3, 'WIP default edited as data from the UI');

  await d.goto(`${url}/#/settings/policies`, { waitUntil: 'networkidle2' }); await sleep(600);
  await click(d, 'button', '+ Policy'); await sleep(400);
  await shot(d, out, 'desktop-1280-12-policy-editor');
  await d.keyboard.press('Escape');

  await d.goto(`${url}/#/audit`, { waitUntil: 'networkidle2' }); await sleep(1000);
  check(await d.evaluate(() => document.body.innerText.includes('chain ok')), 'audit page shows hash chain verified');
  await shot(d, out, 'desktop-1280-13-audit');
  await d.goto(url, { waitUntil: 'networkidle2' }); await sleep(600);
  await click(d, 'button', 'New task'); await sleep(400);
  await shot(d, out, 'desktop-1280-14-new-task-dialog');

  await mobileRun(browser, url, out, errors);
  check(errors.length === 0, 'no console/page errors', errors.slice(0, 3).join(' | '));
} catch (e) {
  check(false, 'ui run aborted', String(e));
} finally {
  await browser.close();
  fs.writeFileSync(resFile, results.join('\n') + '\n');
}
