import type { ReactNode } from 'react';
import { Dialog, DialogTrigger, Heading, Modal as RACModal, ModalOverlay } from 'react-aria-components';

// A trigger + modal dialog. On phones it is a bottom sheet; on desktop a centred card.
export function Modal({ trigger, title, children, alert, wide }: {
  trigger: ReactNode; title: string; alert?: boolean; wide?: boolean;
  children: (close: () => void) => ReactNode;
}) {
  return (
    <DialogTrigger>
      {trigger}
      <ModalOverlay className="overlay" isDismissable={!alert}>
        <RACModal className={`modal ${wide ? 'modal-wide' : ''}`}>
          <Dialog role={alert ? 'alertdialog' : 'dialog'} className="dialog">
            {({ close }) => (
              <>
                <Heading slot="title" className="dialog-title">{title}</Heading>
                {children(close)}
              </>
            )}
          </Dialog>
        </RACModal>
      </ModalOverlay>
    </DialogTrigger>
  );
}
