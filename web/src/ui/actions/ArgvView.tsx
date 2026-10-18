// Shows argv exactly as executed: one chip per element (no shell joining hides spaces or quotes), plus cwd.
export function ArgvView({ argv, cwd }: { argv?: string[]; cwd?: string }) {
  if (!argv?.length && !cwd) return null;
  return (
    <div className="argv">
      {cwd && <div className="argv-cwd"><span className="muted">cwd</span> <code>{cwd}</code></div>}
      {argv && argv.length > 0 && (
        <ol className="argv-list" aria-label="Command arguments">
          {argv.map((a, i) => <li key={i}><code className="argv-chip">{a === '' ? '""' : a}</code></li>)}
        </ol>
      )}
    </div>
  );
}

// shellQuote renders argv for copy/paste with POSIX single quotes.
export function shellQuote(argv: string[]): string {
  return argv.map((a) => (/^[\w@%+=:,./-]+$/.test(a) ? a : `'${a.replace(/'/g, `'\\''`)}'`)).join(' ');
}
