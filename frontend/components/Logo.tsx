/** Circuit-tree SVG mark — used in sidebar. */
export function LogoMark({ size = 22, color = 'currentColor' }: { size?: number; color?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-label="InfraCanvas">
      <line x1="12" y1="3.5" x2="12" y2="22" stroke={color} strokeWidth="1.8" strokeLinecap="round"/>
      <circle cx="12" cy="3.5" r="2.5" stroke={color} strokeWidth="1.8" fill="none"/>
      <line x1="12" y1="10" x2="5" y2="10" stroke={color} strokeWidth="1.8" strokeLinecap="round"/>
      <circle cx="5" cy="10" r="2.5" stroke={color} strokeWidth="1.8" fill="none"/>
      <line x1="12" y1="16" x2="19" y2="16" stroke={color} strokeWidth="1.8" strokeLinecap="round"/>
      <circle cx="19" cy="16" r="2.5" stroke={color} strokeWidth="1.8" fill="none"/>
    </svg>
  )
}
