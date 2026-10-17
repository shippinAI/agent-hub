import { useState } from 'react';
import { Form } from 'react-aria-components';
import { token } from '../lib/api';
import { Button } from '../ui/Button';
import { TextField } from '../ui/Field';

// Token login for human principals when no human auth mode (tailnet / proxy) is configured.
export function Login({ message }: { message: string }) {
  const [v, setV] = useState('');
  return (
    <main className="login">
      <div className="login-card card">
        <div className="brand brand-lg"><span className="brand-mark" aria-hidden />agent-hub</div>
        <p className="muted">Shared task board for your agents. Sign in with a human principal token.</p>
        <p className="hint">{message}</p>
        <Form onSubmit={(e) => { e.preventDefault(); if (v) { token.set(v); location.reload(); } }}>
          <TextField label="Access token" type="password" value={v} onChange={setV} isRequired autoFocus
            description="Kept in this tab's session storage only." />
          <Button type="submit" variant="primary" className="w-full">Sign in</Button>
        </Form>
      </div>
    </main>
  );
}
