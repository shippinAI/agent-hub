import { Button, FieldError, Key, Label, ListBox, ListBoxItem, Popover, Select as RACSelect, SelectValue, Text } from 'react-aria-components';

export type Option = { id: string; label: string; hint?: string };

// Single-select dropdown (React Aria Select + Popover + ListBox).
export function Select({ label, options, value, onChange, hideLabel, isDisabled, className = '', errorMessage }: {
  label: string; options: Option[]; value: string | null; onChange: (v: string) => void; hideLabel?: boolean; isDisabled?: boolean; className?: string;
  errorMessage?: string;
}) {
  return (
    <RACSelect selectedKey={value} onSelectionChange={(k: Key | null) => k != null && onChange(String(k))}
      isInvalid={errorMessage ? true : undefined} validationBehavior="aria"
      isDisabled={isDisabled} className={`field select ${className}`} aria-label={hideLabel ? label : undefined}>
      {!hideLabel && <Label className="label">{label}</Label>}
      <Button className="input select-trigger">
        <SelectValue className="select-value" />
        <svg viewBox="0 0 24 24" className="chev" aria-hidden><path d="m6 9 6 6 6-6" /></svg>
      </Button>
      <Popover className="popover">
        <ListBox className="listbox" items={options}>
          {(o) => (
            <ListBoxItem id={o.id} textValue={o.label} className="option">
              <Text slot="label">{o.label}</Text>{o.hint && <Text slot="description" className="option-hint">{o.hint}</Text>}
            </ListBoxItem>
          )}
        </ListBox>
      </Popover>
      <FieldError className="field-error">{errorMessage}</FieldError>
    </RACSelect>
  );
}
