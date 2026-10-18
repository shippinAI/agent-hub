// Live reviewer flow over MCP (official TypeScript SDK) against a running hub and a REAL bridge.
// Usage: node review.mjs <hub-url> <agent.token> <reviewer.token> <human.token> <machine>  -> PASS/FAIL lines.
// The human token is used only for the REST approve of the grant (agents can never approve).
import fs from 'node:fs';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';

const [base, agentF, reviewerF, humanF, machine] = process.argv.slice(2);
let fails = 0;
const check = (ok, name, d = '') => { console.log(`${ok ? 'PASS' : 'FAIL'} mcp-review: ${name}${d ? ` (${d})` : ''}`); if (!ok) fails++; };
const read = (f) => fs.readFileSync(f, 'utf8').trim();
async function session(tokFile, name) {
  const c = new Client({ name, version: '0.0.1' });
  await c.connect(new StreamableHTTPClientTransport(new URL(`${base}/mcp`), { requestInit: { headers: { Authorization: `Bearer ${read(tokFile)}` } } }));
  return { c, call: async (tool, args = {}) => { const r = await c.callTool({ name: tool, arguments: args }); let body; try { body = JSON.parse(r.content[0].text); } catch { body = { error: { code: 'text', message: r.content[0].text } }; } return { isError: !!r.isError, body }; } };
}
const rest = (method, path, body) => fetch(`${base}/api/v1${path}`, { method, headers: { Authorization: `Bearer ${read(humanF)}`, 'Content-Type': 'application/json', 'X-Hub-Client': 'ui' }, body: body && JSON.stringify(body) }).then((r) => r.json());

const agent = await session(agentF, 'clyffy-live');
const rev = await session(reviewerF, 'clyffy-infra-auditor-live');
const tools = (await rev.c.listTools()).tools.map((t) => t.name);
check(['review_queue', 'review_ticket', 'review_submit', 'ticket_create', 'policy_check'].every((t) => tools.includes(t)), 'reviewer + ticket tools listed', `${tools.length} tools`);

let grants = await agent.call('my_grants', { machine });
if (!(Array.isArray(grants.body) ? grants.body : []).some((g) => g.capability === 'exec' && JSON.stringify(g.selector?.argv_prefix ?? []).includes('"cp"'))) {
  const req = await agent.call('request_access', { machine, capability: 'exec', selector: { argv_prefix: [['uname'], ['cp'], ['rm'], ['sha256sum']] },
    sandbox_level: 'workspace-write', duration: '8h', reason: 'Live Devagotchi demo: run uname, sha256sum and a reviewed cp of an /etc file on the box.' });
  check(!req.isError, 'request_access filed', req.body.error?.code);
  const ok = await rest('POST', `/access-requests/${req.body.request_ids[0]}/approve`, {});
  check(!ok.error, 'human approved the grant over REST', ok.error?.code);
}
const un = await agent.call('exec', { machine, argv: ['uname', '-srm'] });
check(!un.isError && un.body.exit_code === 0, 'real exec on the bridge', (un.body.stdout ?? un.body.error?.code ?? '').trim());
const rm = await agent.call('exec', { machine, argv: ['rm', '-rf', '/etc'] });
check(rm.isError && rm.body.error.code === 'policy_denied', 'rm -rf /etc denied by policy (grant allows rm)', rm.body.error?.message?.split('\n')[1]);

const id = `CHG-${new Date().toISOString().slice(0, 19).replace(/[-:T]/g, '')}`;
const cmd = 'cp /etc/hostname /tmp/hostname.bak';
const snap = await agent.call('exec', { machine, argv: ['sha256sum', '/etc/hostname'] });
const snapText = `${id} pre-change snapshot (sha256sum /etc/hostname, exit ${snap.body.exit_code})\n${(snap.body.stdout || snap.body.stderr || '').slice(0, 4000)}`;
const t = await agent.call('ticket_create', { id, title: 'Back up /etc/hostname on clyffy-box before a rename', machine, command: cmd,
  fields: { topology: 'clyffy-box is a single Linux box; nothing depends on it', blast_radius: 'reads /etc/hostname, writes one file under the sandbox /tmp; nothing in /etc changes', snapshot: `/var/tmp/${id}.snapshot.txt` },
  lists: { research: ['https://man7.org/linux/man-pages/man5/hostname.5.html'], verification: ['sha256sum /tmp/hostname.bak matches the snapshot'], rollback: ['rm /tmp/hostname.bak'] } });
check(!t.isError && t.body.status === 'draft', 'ticket_create (draft)', t.body.error?.message);
const ev = await agent.call('ticket_evidence_add', { ticket: id, kind: 'snapshot', path: `/var/tmp/${id}.snapshot.txt`, text: snapText });
check(!ev.isError, 'snapshot evidence attached from a real exec');
const sub = await agent.call('ticket_submit', { ticket: id });
check(sub.body.status === 'pending_review', 'ticket submitted for review');
const early = await agent.call('exec', { machine, argv: cmd.split(' '), ticket: id });
check(early.isError && early.body.error.code === 'policy_denied' && /PASS/.test(early.body.error.message), 'change blocked until a reviewer PASS');
const self = await agent.call('review_submit', { ticket: id, verdict: 'PASS', summary: 'my own change', ticket_sha256: sub.body.sha256 });
check(self.isError && self.body.error.code === 'self_review', 'author cannot review own ticket');
const q = await rev.call('review_queue');
check((q.body.tickets ?? []).some((x) => x.id === id), 'ticket in Clyffy-Infra-Auditor review queue');
const view = await rev.call('review_ticket', { ticket: id });
check(view.body.evidence?.length === 1 && view.body.preview?.allow === false, 'reviewer sees evidence and the pending decision');
const pass = await rev.call('review_submit', { ticket: id, verdict: 'PASS', ticket_sha256: view.body.ticket.sha256,
  summary: 'Read-only copy of one /etc file into the sandbox /tmp; snapshot attached; blast radius is this box.', findings: [] });
check(!pass.isError && pass.body.status === 'passed', 'Clyffy-Infra-Auditor PASS', pass.body.error?.message);
const run = await agent.call('exec', { machine, argv: cmd.split(' '), ticket: id });
check(!run.isError && run.body.exit_code === 0, 'PASS unlocked the change; it ran on the real bridge', `exit ${run.body.exit_code}: ${(run.body.stdout ?? '').split('\n').length} lines ${(run.body.stderr ?? '').trim().slice(-200)}`);
fs.writeFileSync(process.env.REVIEW_OUT ?? '/dev/null', JSON.stringify({ ticket: id }));
await agent.c.close(); await rev.c.close();
process.exit(fails ? 1 : 0);
