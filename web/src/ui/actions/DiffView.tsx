// Unified diff with +/- line roles announced as text (colour is never the only signal).
export function DiffView({ diff }: { diff: string }) {
  const lines = diff.replace(/\n$/, '').split('\n');
  return (
    <pre className="diff" aria-label="Diff">
      {lines.map((l, i) => {
        const kind = l.startsWith('+++') || l.startsWith('---') ? 'head' : l.startsWith('+') ? 'add' : l.startsWith('-') ? 'del' : l.startsWith('@@') ? 'hunk' : 'ctx';
        return (
          <span key={i} className={`diff-line diff-${kind}`}>
            {kind === 'add' && <span className="sr-only">added: </span>}
            {kind === 'del' && <span className="sr-only">removed: </span>}
            {l}{'\n'}
          </span>
        );
      })}
    </pre>
  );
}
