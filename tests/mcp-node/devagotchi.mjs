// Independent MCP client (official TypeScript SDK) exercising the Devagotchi access tools.
// Usage: node devagotchi.mjs <hub-url> <agent-token-file>  -> PASS/FAIL lines, exit 1 on failure.
import fs from 'node:fs';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';

const [base, tokFile] = process.argv.slice(2);
let fails = 0;
const check = (ok, name, d = '') => { console.log(`${ok ? 'PASS' : 'FAIL'} mcp-node-dev: ${name}${d ? ` (${d})` : ''}`); if (!ok) fails++; };
const tok = fs.readFileSync(tokFile, 'utf8').trim();
const c = new Client({ name: 'node-devagotchi', version: '0.0.1' });
await c.connect(new StreamableHTTPClientTransport(new URL(`${base}/mcp`), { requestInit: { headers: { Authorization: `Bearer ${tok}` } } }));
const call = async (name, args = {}) => { const r = await c.callTool({ name, arguments: args }); return { isError: !!r.isError, body: JSON.parse(r.content[0].text) }; };

const tools = (await c.listTools()).tools.map((t) => t.name);
check(['machines_list', 'request_access', 'wait_for_access', 'my_grants', 'exec', 'capabilities_list'].every((t) => tools.includes(t)), 'devagotchi tools listed', `${tools.length} tools`);
const caps = await call('capabilities_list');
check(Array.isArray(caps.body.capabilities ?? caps.body) , 'capabilities_list returns the catalog');
const ms = await call('machines_list');
const machines = ms.body.machines ?? ms.body;
check(Array.isArray(machines) && machines.some((m) => m.id === 'laptop'), 'machines_list shows the seeded machine');
const noGrant = await call('exec', { machine: 'laptop', argv: ['echo', 'hi'] });
check(noGrant.isError && ['grant_required', 'machine_offline'].includes(noGrant.body.error.code), 'exec without a grant is refused', noGrant.body.error?.code);
const short = await call('request_access', { machine: 'laptop', capability: 'exec', selector: { argv_prefix: [['echo']] }, sandbox_level: 'workspace-write', duration: '1h', reason: 'too short' });
check(short.isError && short.body.error.code === 'reason_too_short', 'short reason refused');
const req = await call('request_access', { machine: 'laptop', capability: 'exec', selector: { argv_prefix: [['echo']] }, sandbox_level: 'workspace-write', duration: '1h', reason: 'Run echo on the laptop to verify the MCP access path end to end.' });
check(!req.isError && Array.isArray(req.body.request_ids) && req.body.request_ids.length === 1, 'request_access returns a pending request');
const mine = await call('my_grants');
check(!mine.isError, 'my_grants callable');
await c.close();
process.exit(fails ? 1 : 0);
