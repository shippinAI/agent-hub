import {
  FieldError, Input, Label, NumberField as RACNumber, Switch as RACSwitch, Text, TextArea as RACTextArea, TextField as RACTextField,
  Checkbox, CheckboxGroup as RACCheckboxGroup, SearchField as RACSearch, Button as RACButton,
} from 'react-aria-components';

type Common = { label: string; description?: string; className?: string };

export function TextField({ label, description, className = '', multiline, rows = 3, errorMessage, ...p }: Common & {
  value?: string; onChange?: (v: string) => void; isRequired?: boolean; autoFocus?: boolean; placeholder?: string;
  multiline?: boolean; rows?: number; validate?: (v: string) => string | null; type?: string; name?: string; isDisabled?: boolean;
  errorMessage?: string;
}) {
  const { placeholder, ...rest } = p;
  return (
    <RACTextField {...rest} isInvalid={errorMessage ? true : undefined} validationBehavior="aria" className={`field ${className}`}>
      <Label className="label">{label}</Label>
      {multiline ? <RACTextArea className="input textarea" rows={rows} placeholder={placeholder} /> : <Input className="input" placeholder={placeholder} />}
      {description && <Text slot="description" className="hint">{description}</Text>}
      <FieldError className="field-error">{errorMessage}</FieldError>
    </RACTextField>
  );
}

export function NumberField({ label, description, errorMessage, ...p }: Common & { value?: number; onChange?: (v: number) => void; minValue?: number; maxValue?: number; errorMessage?: string }) {
  return (
    <RACNumber {...p} isInvalid={errorMessage ? true : undefined} validationBehavior="aria" className="field">
      <Label className="label">{label}</Label>
      <Input className="input" />
      {description && <Text slot="description" className="hint">{description}</Text>}
      <FieldError className="field-error">{errorMessage}</FieldError>
    </RACNumber>
  );
}

export function Switch({ children, ...p }: { children: string; isSelected?: boolean; onChange?: (v: boolean) => void; isDisabled?: boolean }) {
  return (
    <RACSwitch {...p} className="switch">
      <span className="switch-track" aria-hidden><span className="switch-thumb" /></span>
      {children}
    </RACSwitch>
  );
}

export function CheckboxGroup({ label, options, value, onChange, errorMessage, description }: {
  label: string; options: { id: string; label: string }[]; value: string[]; onChange: (v: string[]) => void; errorMessage?: string; description?: string;
}) {
  return (
    <RACCheckboxGroup value={value} onChange={onChange} isInvalid={errorMessage ? true : undefined} validationBehavior="aria" className="field">
      <Label className="label">{label}</Label>
      <div className="check-grid">
        {options.map((o) => (
          <Checkbox key={o.id} value={o.id} className="checkbox">
            <span className="checkbox-box" aria-hidden>
              <svg viewBox="0 0 18 18"><polyline points="2 9 7 14 16 4" /></svg>
            </span>
            {o.label}
          </Checkbox>
        ))}
      </div>
      {description && <Text slot="description" className="hint">{description}</Text>}
      <FieldError className="field-error">{errorMessage}</FieldError>
    </RACCheckboxGroup>
  );
}

export function SearchField({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (v: string) => void; placeholder?: string }) {
  return (
    <RACSearch value={value} onChange={onChange} className="search" aria-label={label}>
      <svg className="search-icon" viewBox="0 0 24 24" aria-hidden><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
      <Input className="input" placeholder={placeholder} />
      <RACButton className="search-clear" aria-label="Clear">✕</RACButton>
    </RACSearch>
  );
}
