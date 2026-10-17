// Independent MCP client (official TypeScript SDK, Streamable HTTP) acting as two agents.
// Usage: node client.mjs <hub-url> <tokens-dir>  -> prints PASS/FAIL lines, exit 1 on failure.
import fs from 'node:fs';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';

const [base, toks] = process.argv.slice(2);
let fails = 0;
const check = (ok, name, d = '') => { console.log(`${ok ? 'PASS' : 'FAIL'} mcp-node: ${name}${d ? ` (${d})` : ''}`); if (!ok) fails++; };
async function connect(who) {
  const tok = fs.readFileSync(`${toks}/${who}.token`, 'utf8').trim();
  const c = new Client({ name: `node-${who}`, version: '0.0.1' });
  await c.connect(new StreamableHTTPClientTransport(new URL(`${base}/mcp`), { requestInit: { headers: { Authorization: `Bearer ${tok}` } } }));
  return c;
}
const call = async (c, name, args = {}) => {
  const r = await c.callTool({ name, arguments: args });
  return { isError: !!r.isError, body: JSON.parse(r.content[0].text) };
};

const a = await connect('cursor');
const b = await connect('claude-code');
check((a.getInstructions() ?? '').startsWith('agent-hub:'), 'server instructions received');
const tools = (await a.listTools()).tools.map((t) => t.name);
check(['claim_next', 'heartbeat', 'handoff', 'complete', 'search'].every((t) => tools.includes(t)), 'tools listed', tools.length + ' tools');
const who = await call(a, 'whoami');
check(who.body.principal.id === 'cursor', 'identity from bearer token', who.body.principal.id);
const t = await call(a, 'create_task', { project: 'general', title: 'node SDK task', labels: ['mcp-node'], request_id: 'node-1' });
const l = await call(a, 'claim', { task_id: t.body.id, ttl_seconds: 120 });
check(l.body.claimed && l.body.fence > 0, 'claim returns a fence', `fence ${l.body.fence}`);
const steal = await call(b, 'claim', { task_id: t.body.id });
check(steal.isError && steal.body.error.code === 'held_by_other', 'second agent cannot take a live lease');
const stale = await call(a, 'complete', { task_id: t.body.id, fence: l.body.fence + 1000, text: 'x' });
check(stale.isError && stale.body.error.code === 'stale_fence', 'wrong fence rejected');
const h = await call(a, 'handoff', { task_id: t.body.id, fence: l.body.fence, note: 'node: half done, see notes', to: 'claude-code' });
check(!h.isError && h.body.assignee === 'claude-code', 'handoff reserves the task for the target');
const g = await call(b, 'get_task', { task_id: t.body.id });
check(g.body.notes.some((n) => n.kind === 'handoff'), 'handoff note visible to the next agent');
await a.close(); await b.close();
process.exit(fails ? 1 : 0);
