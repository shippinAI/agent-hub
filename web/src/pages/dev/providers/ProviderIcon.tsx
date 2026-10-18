// A provider's icon. icon_ref names an icon set entry (icons/<vendor>); until the icon set ships, the UI
// draws a monogram with a stable hue from that ref. Decorative: the label is always next to it.
export function ProviderIcon({ iconRef, vendor, size = 28 }: { iconRef: string; vendor: string; size?: number }) {
  let h = 0;
  for (const c of iconRef || vendor) h = (h * 31 + c.charCodeAt(0)) % 360;
  const letters = vendor.replace(/[^A-Za-z0-9]/g, '').slice(0, 2) || '?';
  return (
    <span className="provider-icon" aria-hidden data-icon={iconRef}
      style={{ width: size, height: size, color: `hsl(${h} 60% 32%)`, background: `hsl(${h} 75% 92%)` }}>
      {letters}
    </span>
  );
}
