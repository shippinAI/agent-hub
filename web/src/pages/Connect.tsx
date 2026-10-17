import { useHub } from '../lib/hub';
import type { Principal } from '../lib/types';
import { Button } from '../ui/Button';
import { Modal } from '../ui/Modal';

// Per-client config snippets. The token stays in an env var (HUB_TOKEN), never in the snippet.
export function Connect({ principal: p }: { principal: Principal }) {
  const { me } = useHub();
  const mcp = `${me.public_url.replace(/\/$/, '')}/mcp`;
  const snippets: [string, string][] = [
    ['Claude Code', `claude mcp add --transport http hub ${mcp} \\\n  --header "Authorization: Bearer \${HUB_TOKEN}"`],
    ['Codex CLI (~/.codex/config.toml)', `[mcp_servers.hub]\nurl = "${mcp}"\nbearer_token_env_var = "HUB_TOKEN"`],
    ['Cursor (.cursor/mcp.json)', `{ "mcpServers": { "hub": { "url": "${mcp}",\n  "headers": { "Authorization": "Bearer \${env:HUB_TOKEN}" } } } }`],
    ['CLI / REST', `export HUB_URL=${me.public_url}\nexport HUB_TOKEN_FILE=~/.config/hub/${p.id}.token\nhub whoami && hub claim`],
  ];
  return (
    <Modal wide title={`Connect ${p.display_name}`} trigger={<Button variant="ghost" size="sm">Connect</Button>}>
      {(close) => (
        <div className="stack-3">
          <p className="muted">Issue a token under <b>Tokens</b>, put it in <code>HUB_TOKEN</code> on the agent's machine, then:</p>
          {snippets.map(([k, v]) => (
            <div key={k} className="snippet">
              <div className="row between"><strong>{k}</strong><Button size="sm" variant="ghost" onPress={() => navigator.clipboard?.writeText(v)}>Copy</Button></div>
              <pre className="code">{v}</pre>
            </div>
          ))}
          <div className="dialog-actions"><Button variant="ghost" onPress={close}>Close</Button></div>
        </div>
      )}
    </Modal>
  );
}
