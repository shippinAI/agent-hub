// 390px phone pass: same pages as a touch device; checks there is no horizontal overflow.
import { check, click, noOverflow, shot, sleep } from './lib.mjs';

export async function mobileRun(browser, url, out, errors) {
  const m = await browser.newPage();
  m.on('pageerror', (e) => errors.push(String(e)));
  await m.setViewport({ width: 390, height: 844, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
  const pages = [
    ['01-board', ''], ['03-task-detail', '#/task/2'], ['05-agents', '#/agents'],
    ['06-settings-policies', '#/settings/policies'], ['07-settings-workflows', '#/settings/workflows'], ['08-audit', '#/audit'],
  ];
  for (const [name, hash] of pages) {
    await m.goto(`${url}/${hash}`, { waitUntil: 'networkidle2' }); await sleep(900);
    await shot(m, out, `mobile-390-${name}`);
    check(await noOverflow(m, 390), `390px: no horizontal overflow on ${name}`);
    if (name === '01-board') {
      await click(m, '.chip', 'In progress'); await sleep(500);
      await shot(m, out, 'mobile-390-02-board-in-progress');
      check(await m.evaluate(() => document.querySelectorAll('.column.is-selected .card-item').length > 0), '390px: state chips switch the list');
    }
    if (name === '03-task-detail') {
      await click(m, 'button', 'Force release'); await sleep(700);
      await shot(m, out, 'mobile-390-04-force-release-sheet');
      await m.keyboard.press('Escape');
    }
  }
  await m.goto(url, { waitUntil: 'networkidle2' }); await sleep(600);
  await click(m, 'button', 'New task'); await sleep(700);
  await shot(m, out, 'mobile-390-09-new-task-sheet');
  const tap = await m.evaluate(() => Math.min(...[...document.querySelectorAll('.tab-item')].map((e) => e.getBoundingClientRect().height)));
  check(tap >= 44, '390px: bottom tab targets >= 44px tall', `${tap}px`);
}
