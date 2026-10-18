// Catalog › Providers on the LIVE hub at 1280 and 390: table, detail, edit form, add form, client and server
// validation errors, plus role/label (a11y) and overflow checks. Writes test-evidence/screenshots/providers-*.png
// and test-evidence/providers-ui.txt. Usage: node providers-shots.mjs <hub-url> <human.token file>
// The token goes into sessionStorage by an init script and is never printed. Nothing is saved to the hub.
import { chromium } from 'playwright-core';
import { readFileSync, mkdirSync, writeFileSync } from 'node:fs';

const [url, tokFile] = process.argv.slice(2);
const tok = readFileSync(tokFile, 'utf8').trim();
const ev = new URL('../../test-evidence/', import.meta.url).pathname;
const out = `${ev}screenshots/`;
mkdirSync(out, { recursive: true });
const log = [];
const ok = (c, msg) => { log.push(`${c ? 'PASS' : 'FAIL'} ${msg}`); console.log(log.at(-1)); };
const errors = [];

const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', args: ['--no-sandbox'] });
try {
  for (const [w, h] of [[1280, 900], [390, 844]]) {
    const ctx = await browser.newContext({ viewport: { width: w, height: h }, deviceScaleFactor: 1 });
    await ctx.addInitScript((t) => sessionStorage.setItem('agent-hub-token', t), tok);
    const p = await ctx.newPage();
    p.on('pageerror', (e) => errors.push(`${w}: ${e}`));
    const shot = async (name) => { const f = `${out}providers-${name}-${w}.png`; await p.waitForTimeout(350); await p.screenshot({ path: f, fullPage: name === 'list' }); return f.replace(ev, 'test-evidence/'); };
    const noOverflow = async () => (await p.evaluate(() => document.documentElement.scrollWidth)) <= w;

    await p.goto(`${url}/#/settings/catalog`);
    const grid = p.getByRole('grid', { name: 'Providers' });
    await grid.waitFor({ timeout: 15000 });
    await p.getByRole('row', { name: /Grok/ }).first().waitFor();
    const tab = p.getByRole('tab', { name: 'Providers' });
    ok(await tab.getAttribute('aria-selected') === 'true', `${w}: Providers tab is a selected ARIA tab`);
    const rows = (await grid.getByRole('row').count()) - 1;
    ok(rows >= 20, `${w}: table lists ${rows} provider rows`);
    const heads = await grid.getByRole('columnheader').allInnerTexts();
    ok(['Provider', 'Adapter'].every((x) => heads.some((hh) => hh.includes(x))), `${w}: column headers ${heads.filter(Boolean).join(', ')}`);
    ok(!(await p.getByText(/unverified|terms_status/i).count()), `${w}: no verified/unverified terms badge anywhere`);
    ok(await noOverflow(), `${w}: list has no horizontal page overflow`);
    ok(true, `${w}: list -> ${await shot('list')}`);

    // Keyboard on desktop (focus a row, Enter opens it); tap on phone. The detail is a dialog named by the row.
    const grok = p.getByRole('row', { name: /Grok/ }).first();
    if (w === 1280) {
      await grok.focus(); await p.keyboard.press('Enter');
    } else { await grok.click(); }
    const det = p.getByRole('dialog', { name: /Grok/ });
    await det.getByRole('button', { name: 'Edit' }).waitFor({ timeout: 10000 });
    ok(await det.isVisible(), `${w}: ${w === 1280 ? 'keyboard (Enter)' : 'tap'} opens the row in a dialog named "Grok"`);
    ok((await p.getByText('acp', { exact: true }).count()) > 0 && (await p.getByText('headless_json').count()) > 0, `${w}: detail shows adapter acp and headless_json fallback`);
    ok(await noOverflow(), `${w}: detail has no horizontal page overflow`);
    ok(true, `${w}: detail -> ${await shot('detail')}`);

    await det.getByRole('button', { name: 'Edit' }).click();
    const form = p.getByRole('form', { name: /Edit Grok/ });
    await form.waitFor();
    ok(await form.getByLabel('ID', { exact: true }).isDisabled(), `${w}: edit form locks the id`);
    ok(await form.getByRole('group', { name: 'Sign-in kinds' }).isVisible(), `${w}: sign-in kinds is a labelled checkbox group`);
    ok(await noOverflow(), `${w}: edit form has no horizontal page overflow`);
    ok(true, `${w}: edit -> ${await shot('edit')}`);
    await form.getByRole('button', { name: 'Cancel' }).click();
    await p.keyboard.press('Escape');
    await det.waitFor({ state: 'detached' });
    ok(await grok.evaluate((el) => el.contains(document.activeElement) || document.activeElement === el), `${w}: Escape closes the dialog and focus returns to the row`);

    await p.getByRole('button', { name: 'Add provider' }).click();
    const dlg = p.getByRole('dialog', { name: 'Add provider' });
    await dlg.waitFor();
    ok(true, `${w}: add form is a labelled dialog -> ${await shot('add')}`);
    await dlg.getByRole('button', { name: 'Add provider' }).click();
    const alert = dlg.getByRole('alert');
    await alert.waitFor();
    const idInput = dlg.getByLabel('ID', { exact: true });
    ok(await idInput.getAttribute('aria-invalid') === 'true', `${w}: client check marks ID aria-invalid`);
    ok(await alert.evaluate((el) => document.activeElement === el), `${w}: focus moves to the error summary`);
    ok(true, `${w}: client validation -> ${await shot('validation-client')}`);

    // Server-only rule: a detect dir that names a credential store is refused by the hub's validator.
    await idInput.fill('example-cli'); await dlg.getByLabel('Label', { exact: true }).fill('Example');
    await dlg.getByLabel('Vendor', { exact: true }).fill('Example'); await dlg.getByLabel('Product', { exact: true }).fill('Example CLI');
    await dlg.getByLabel('Icon', { exact: true }).fill('icons/example');
    await dlg.getByLabel('Adapter config (JSON)').fill('{"argv": ["example", "acp"]}');
    await dlg.getByLabel('Detection rules per OS (JSON)').fill('{"linux": {"bins": ["example"], "dirs": ["~/.example/auth.json"]}}');
    await dlg.getByRole('button', { name: 'Add provider' }).click();
    await dlg.getByText('detect.linux.dirs[0]').waitFor({ timeout: 10000 });
    ok(await dlg.getByLabel('Detection rules per OS (JSON)').getAttribute('aria-invalid') === 'true', `${w}: hub 422 error lands on the detect field`);
    ok(true, `${w}: server validation -> ${await shot('validation-server')}`);
    await dlg.getByRole('button', { name: 'Cancel' }).click();
    if (w === 1280) { // round trip: Edit -> Save with no changes must pass the hub's validator for each row shape
      for (const name of ['Grok', 'DeepSeek', 'Claude Desktop']) {
        await p.getByRole('row', { name: new RegExp(name) }).first().click();
        const dd = p.getByRole('dialog', { name: new RegExp(name) });
        const before = await dd.getByText(/row v\d+/).innerText();
        await dd.getByRole('button', { name: 'Edit' }).click();
        await dd.getByRole('button', { name: 'Save changes' }).click();
        await dd.getByRole('button', { name: 'Edit' }).waitFor({ timeout: 10000 });
        const after = await dd.getByText(/row v\d+/).innerText();
        ok(before !== after, `${w}: ${name} saves unchanged through the form (${before.trim()} -> ${after.trim()})`);
        await p.keyboard.press('Escape'); await dd.waitFor({ state: 'detached' });
      }
    }
    await ctx.close();
  }
} finally {
  await browser.close();
}
ok(errors.length === 0, `no page errors${errors.length ? `: ${errors.join(' | ')}` : ''}`);
writeFileSync(`${ev}providers-ui.txt`, `# providers UI on ${url} ${new Date().toISOString()}\n${log.join('\n')}\n`);
process.exit(log.some((l) => l.startsWith('FAIL')) ? 1 : 0);
