import { useEffect, useState } from 'react';
import { get, post } from '../../lib/api';
import type { Alert } from '../../lib/dtypes';
import { Dialog, Modal, ModalOverlay, Heading } from 'react-aria-components';
import { Button } from '../../ui/Button';
import { RiskList } from './risk';

// Resume alert (04/07): modal alertdialog shown on resume while a full-access/unsandboxed grant is live.
// No close button and no "don't show again"; buttons are Continue, Revoke now, Pause machine.
export function ResumeAlert() {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const load = () => get<Alert[]>('/resume-alerts').then(setAlerts).catch(() => {});
  useEffect(() => {
    post('/resume-alerts/check', { ui_session_id: sessionId(), reason: 'page_load' }).then(load).catch(load);
  }, []);
  const a = alerts[0];
  if (!a) return null;
  const act = async (verb: string) => {
    if (verb === 'revoke') await post(`/grants/${a.grant_id}/revoke`, { reason: 'revoked from resume alert' });
    await post(`/resume-alerts/${a.id}/continue`).catch(() => {});
    setAlerts((xs) => xs.filter((x) => x.id !== a.id));
    load();
  };
  return (
    <ModalOverlay className="overlay" isDismissable={false} isOpen>
      <Modal className="modal"><Dialog role="alertdialog" className="dialog" aria-label="Resume alert">
        <Heading slot="title" className="dialog-title">Powerful access is live</Heading>
        <p>A {a.full_access ? 'full-access' : a.sandbox_level} grant for <strong>{a.principal_id}</strong> on <strong>{a.machine_label}</strong> is active.</p>
        <RiskList risks={a.risks} />
        <div className="row gap-2 actions">
          <Button variant="primary" onPress={() => act('continue')}>Continue</Button>
          <Button variant="destructive" onPress={() => act('revoke')}>Revoke now</Button>
        </div>
      </Dialog></Modal>
    </ModalOverlay>
  );
}

function sessionId(): string {
  let s = sessionStorage.getItem('dev-ui-session');
  if (!s) { s = crypto.randomUUID(); sessionStorage.setItem('dev-ui-session', s); }
  return s;
}
