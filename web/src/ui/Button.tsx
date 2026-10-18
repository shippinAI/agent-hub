import type { Ref } from 'react';
import { Button as RACButton, ButtonProps, Link as RACLink, LinkProps } from 'react-aria-components';

type Variant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'destructive';
type Size = 'sm' | 'md' | 'icon';

// Button styled like a modern design system; behaviour (press, focus, a11y) comes from React Aria.
export function Button({ variant = 'secondary', size = 'md', className = '', ...p }: ButtonProps & { variant?: Variant; size?: Size; ref?: Ref<HTMLButtonElement> }) {
  return <RACButton {...p} className={`btn btn-${variant} btn-${size} ${className}`} />;
}

export function LinkButton({ variant = 'ghost', size = 'md', className = '', ...p }: LinkProps & { variant?: Variant; size?: Size }) {
  return <RACLink {...p} className={`btn btn-${variant} btn-${size} ${className}`} />;
}
