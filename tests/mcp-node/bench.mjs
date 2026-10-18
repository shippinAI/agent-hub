// Startup and op latency for devagotchi vs Desktop Commander, measured with the same official TS MCP client.
// Usage: node bench.mjs <label> <runs> <workdir> -- <command> [args...]
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";

const [label, runsS, work, sep, cmd, ...args] = process.argv.slice(2);
const runs = Number(runsS);
const ms = (t) => Number((performance.now() - t).toFixed(1));
const median = (a) => [...a].sort((x, y) => x - y)[Math.floor(a.length / 2)];
const env = { ...process.env };

async function connect() {
  const t = performance.now();
  const c = new Client({ name: "bench", version: "1" });
  await c.connect(new StdioClientTransport({ command: cmd, args, env, stderr: "ignore" }));
  return [c, ms(t)];
}
async function timed(c, name, a) {
  const t = performance.now();
  try {
    const r = await c.callTool({ name, arguments: a }, undefined, { timeout: 600000 });
    const text = (r.content || []).map((x) => x.text || "").join("");
    return { ms: ms(t), ok: !r.isError, text: text.slice(0, 160).replace(/\s+/g, " ") };
  } catch (e) { return { ms: ms(t), ok: false, text: String(e.message).slice(0, 160) }; }
}

const init = [], list = [];
for (let i = 0; i < runs; i++) {
  const [c, t] = await connect();
  init.push(t);
  const t2 = performance.now();
  const tl = await c.listTools();
  list.push(ms(t2));
  if (i === runs - 1) console.log(`${label}: ${tl.tools.length} tools`);
  await c.close();
}
const [c] = await connect();
const out = { label, runs, init_ms_median: median(init), init_ms_max: Math.max(...init), tools_list_ms_median: median(list) };
const ops = {
  read_file_1000_lines: ["read_file", { path: `${work}/big.txt`, offset: 0, length: 1000 }],
  list_directory: ["list_directory", { path: work }],
  start_search_content: ["start_search", { path: work, pattern: "needle", searchType: "content" }],
  start_process_echo: ["start_process", { command: "echo hi", timeout_ms: 5000 }],
};
for (const [k, [n, a]] of Object.entries(ops)) {
  const rs = [];
  for (let i = 0; i < 5; i++) rs.push(await timed(c, n, a));
  out[k] = { ms_median: median(rs.map((r) => r.ms)), ok: rs.every((r) => r.ok), sample: rs[0].text };
}
if (process.env.LONG) out.start_process_sleep75_timeout80s = await timed(c, "start_process", { command: "sleep 75; echo done", timeout_ms: 80000 });
await c.close();
console.log(JSON.stringify(out, null, 1));
