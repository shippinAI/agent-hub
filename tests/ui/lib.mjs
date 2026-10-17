// Shared helpers for the headless-Chrome UI run.
import fs from 'node:fs';
export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
export const results = [];
export function check(ok, name, detail = '') {
  results.push(`${ok ? 'PASS' : 'FAIL'} ${name}${detail ? ` (${detail})` : ''}`);
  console.log(results.at(-1));
}

// Click the first visible element matching sel whose text contains text, with a real mouse event.
export async function click(page, sel, text) {
  const handles = await page.$$(sel);
  for (const h of handles) {
    const t = await h.evaluate((e) => e.textContent.trim());
    const vis = await h.evaluate((e) => !!(e.offsetWidth || e.offsetHeight || e.getClientRects().length));
    if (vis && (!text || t.includes(text))) {
      await h.evaluate((e) => e.scrollIntoView({ block: 'center' }));
      await h.click();
      await sleep(400);
      return;
    }
  }
  throw new Error(`no visible ${sel} containing "${text}"`);
}

export async function shot(page, out, name, full = false) {
  await sleep(300);
  // Never put a secret on disk: mask anything that looks like a hub token before capturing.
  await page.evaluate(() => document.querySelectorAll('code').forEach((c) => {
    if (/ahk_[A-Za-z0-9_-]{20,}/.test(c.textContent)) c.textContent = 'ahk_•••••••• (masked in screenshot)';
  }));
  await page.screenshot({ path: `${out}/${name}.png`, fullPage: full });
  console.log('shot', name);
}

export async function noOverflow(page, width) {
  const w = await page.evaluate(() => document.documentElement.scrollWidth);
  return w <= width;
}

export function api(base, tokenFile) {
  const tok = fs.readFileSync(tokenFile, 'utf8').trim();
  return async (method, path, body) => {
    const r = await fetch(`${base}/api/v1${path}`, { method, headers: { Authorization: `Bearer ${tok}`, 'Content-Type': 'application/json' },
      body: body ? JSON.stringify(body) : undefined });
    return r.json();
  };
}
