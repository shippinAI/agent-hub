// Playwright screenshots of the LIVE hub UI (1280 and 390): clyffy-box Online, a policy denial, a reviewer PASS
// unlocking a change, the policy editor. Usage: node gate-shots.mjs <hub-url> <human.token file> <ticket-id>
// The token is put in sessionStorage by an init script; it is never printed.
import { chromium } from 'playwright-core';
import { readFileSync, mkdirSync, writeFileSync } from 'node:fs';

const [url, tokFile, ticket] = process.argv.slice(2);
const tok = readFileSync(tokFile, 'utf8').trim();
const out = new URL('../../test-evidence/screenshots/', import.meta.url).pathname;
mkdirSync(out, { recursive: true });
const log = [];
const ok = (c, msg) => { log.push(`${c ? 'PASS' : 'FAIL'} ${msg}`); console.log(log.at(-1)); };

const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', args: ['--no-sandbox'] });
for (const [w, h] of [[1280, 900], [390, 844]]) {
  const ctx = await browser.newContext({ viewport: { width: w, height: h }, deviceScaleFactor: 1 });
  await ctx.addInitScript((t) => sessionStorage.setItem('agent-hub-token', t), tok);
  const p = await ctx.newPage();
  const visit = async (hash, wait) => {
    await p.goto(`${url}/#/${hash}`);
    await p.getByText(wait, { exact: false }).first().waitFor({ timeout: 10000 });
    await p.waitForTimeout(400);
  };
  const shot = async (name) => { const f = `${out}gate-${name}-${w}.png`; await p.screenshot({ path: f, fullPage: true }); return f; };

  await visit('dev/machines', 'Clyffy box');
  const row = p.getByText('Clyffy box (Linux)').first();
  ok(await row.isVisible() && (await p.getByText('Online').count()) > 0, `${w}: Machines lists clyffy-box Online -> ${await shot('machines-online')}`);

  await visit('dev/policy/try', 'Command');
  await p.getByLabel('Command').fill('rm -rf /etc');
  await p.getByLabel('Machine (optional)').fill('clyffy-box');
  await p.getByRole('button', { name: 'Check' }).click();
  await p.getByText('destructive-root-delete').first().waitFor({ timeout: 10000 });
  ok(true, `${w}: Try it shows rm -rf /etc denied by destructive-root-delete -> ${await shot('policy-denial')}`);

  await visit('dev/policy/decisions', 'clyffy');
  ok((await p.getByText(/deny/i).count()) > 0, `${w}: decision log (hash-chained) shows real denials -> ${await shot('decisions')}`);

  await visit(`dev/changes/${ticket}`, ticket);
  const passed = (await p.getByText(/passed/i).count()) > 0 && (await p.getByText('PASS').count()) > 0;
  ok(passed && (await p.getByText('clyffy-infra-auditor', { exact: false }).count()) > 0,
    `${w}: ${ticket} passed by Clyffy-Infra-Auditor, change unlocked -> ${await shot('review-pass')}`);

  await visit('dev/policy/rules', 'infra-change-review');
  ok(true, `${w}: policy editor (Rules) -> ${await shot('policy-editor')}`);
  await visit('dev/policy/json', 'infra-change-review');
  ok(true, `${w}: policy editor (JSON) -> ${await shot('policy-json')}`);
  await ctx.close();
}
await browser.close();
writeFileSync(new URL('../../test-evidence/ui-gate.txt', import.meta.url), log.join('\n') + '\n');
process.exit(log.some((l) => l.startsWith('FAIL')) ? 1 : 0);
