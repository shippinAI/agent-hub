// Action feed (39) + audit retention admin (40) on the demo hub at 1280 and 390. Checks roles, that the demo
// secret never reaches the DOM, and page overflow. Usage: node actions-shots.mjs <demo.json> <out-dir> <log-file>
// demo.json holds {url, token} for a throwaway test hub; the token goes into sessionStorage and is never printed.
import { chromium } from 'playwright-core';
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';

const [demoFile, out, logFile] = process.argv.slice(2);
const { url, token } = JSON.parse(readFileSync(demoFile, 'utf8'));
mkdirSync(out, { recursive: true });
const log = [];
const ok = (c, m) => { log.push(`${c ? 'PASS' : 'FAIL'} ${m}`); console.log(log.at(-1)); };
const secret = 'sk-demo1234567890abcdef';
const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', args: ['--no-sandbox'] });
try {
  for (const [w, h] of [[1280, 900], [390, 844]]) {
    const ctx = await browser.newContext({ viewport: { width: w, height: h } });
    await ctx.addInitScript((t) => sessionStorage.setItem('agent-hub-token', t), token);
    const p = await ctx.newPage();
    const errors = [];
    p.on('pageerror', (e) => errors.push(String(e)));
    const shot = async (name, full = false) => {
      const f = `${out}/actions-${name}-${w}.png`;
      await p.waitForTimeout(400);
      await p.screenshot({ path: f, fullPage: full });
      ok(true, `${w}: ${name} -> ${f.slice(f.indexOf('test-evidence'))}`);
    };
    const noOverflow = async () => (await p.evaluate(() => document.documentElement.scrollWidth)) <= w;

    await p.goto(`${url}/#/actions`);
    const feed = p.getByRole('list', { name: 'Agent actions' });
    await feed.waitFor({ timeout: 15000 });
    await p.locator('.action-title', { hasText: 'edit_block' }).first().waitFor({ timeout: 15000 });
    const cards = await p.locator('article.action-card').count();
    ok(cards >= 6, `${w}: feed renders ${cards} action cards`);
    const html = await p.content();
    ok(!html.includes(secret) && html.includes('[redacted]'), `${w}: demo secret is masked in the DOM`);
    ok(await p.getByText('approved by').first().isVisible(), `${w}: policy line names the approver`);
    ok((await p.locator('.action-denied').count()) >= 1, `${w}: denied call is shown`);
    ok(await p.getByRole('list', { name: 'Command arguments' }).first().isVisible(), `${w}: argv is a labelled list`);
    ok(await noOverflow(), `${w}: feed has no horizontal overflow`);
    await shot('feed', true);
    const edit = p.locator('article.action-card').filter({ has: p.locator('.action-title', { hasText: 'edit_block' }) }).filter({ has: p.locator('.diff') }).first();
    await edit.scrollIntoViewIfNeeded();
    ok(await edit.getByText('+replicas: 3').isVisible(), `${w}: edit_block card shows the diff`);
    await edit.screenshot({ path: `${out}/actions-diff-card-${w}.png` });
    ok(true, `${w}: diff card -> test-evidence/screenshots/actions-diff-card-${w}.png`);

    await p.goto(`${url}/#/settings/retention`);
    const table = p.getByRole('grid', { name: 'Audit categories' });
    await table.waitFor({ timeout: 15000 });
    ok((await table.getByRole('row').count()) >= 7, `${w}: retention table lists the 6 seeded categories`);
    ok(await p.getByRole('tab', { name: 'Audit retention', selected: true }).isVisible(), `${w}: retention is a selected ARIA tab`);
    await shot('retention', true);
    if (w === 1280) {
      await p.getByRole('button', { name: 'Edit proc' }).click();
      const dlg = p.getByRole('dialog', { name: /Audit category: proc/ });
      await dlg.waitFor();
      ok(await dlg.getByRole('switch', { name: 'Record this category' }).isVisible(), 'category editor uses labelled switches');
      await shot('retention-edit');
      await dlg.getByRole('button', { name: 'Cancel' }).click();
      await p.getByRole('button', { name: 'Prune now' }).click();
      await p.getByText(/Pruned:/).waitFor({ timeout: 10000 });
      ok(true, 'Prune now reports its result');
      await shot('retention-pruned');
    }
    ok(errors.length === 0, `${w}: no page errors ${errors.join(' | ')}`);
    await ctx.close();
  }
} finally {
  await browser.close();
  writeFileSync(logFile, log.join('\n') + '\n');
}
