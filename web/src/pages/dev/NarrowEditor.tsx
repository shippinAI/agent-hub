import { useState } from 'react';
import type { Request } from '../../lib/dtypes';
import { Button } from '../../ui/Button';
import { Select } from '../../ui/Select';
import { TextField } from '../../ui/Field';

const LEVELS = ['read-only', 'workspace-write', 'desktop', 'unsandboxed'];

// Narrow editor: lower the sandbox level, trim paths/argv, shorten duration (07). Widening is refused by the hub.
export function NarrowEditor({ req, onCancel, onApprove }: {
  req: Request; onCancel: () => void; onApprove: (body: unknown) => void;
}) {
  const [level, setLevel] = useState(req.sandbox_level);
  const paths0 = (req.selector.paths as string[] | undefined) ?? [];
  const [paths, setPaths] = useState(paths0.join('\n'));
  const lowerLevels = LEVELS.slice(0, Math.max(1, LEVELS.indexOf(req.sandbox_level) + 1));
  const submit = () => {
    const selector: Record<string, unknown> = { ...req.selector };
    if (paths0.length) selector.paths = paths.split('\n').map((p) => p.trim()).filter(Boolean);
    onApprove({ sandbox_level: level, selector });
  };
  return (
    <div className="stack-3 narrow">
      <h3>Approve narrower</h3>
      <Select label="Sandbox level (only lower)" value={level} onChange={setLevel}
        options={lowerLevels.map((l) => ({ id: l, label: l }))} />
      {paths0.length > 0 && <TextField label="Paths (one per line; remove to narrow)" value={paths} onChange={setPaths} multiline rows={3} />}
      <div className="row gap-2 end">
        <Button onPress={onCancel}>Back</Button>
        <Button variant="primary" onPress={submit}>Approve narrowed</Button>
      </div>
    </div>
  );
}
