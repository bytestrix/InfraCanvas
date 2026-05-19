// Single source of truth for all infrastructure node type SVG icons.
// Used by: InfraNode (canvas), GroupDrawer (header), NodeDetailPanel (header).
// Always pass `size` — default 16. Do NOT add emoji or text fallbacks here.

const K8S  = '#326CE5'
const DOCK = '#2496ED'
const INK2 = 'var(--ink2)'
const INK3 = 'var(--ink3)'

interface Props { type: string; size?: number }

export default function NodeSvgIcon({ type, size = 16 }: Props) {
  const s = size
  switch (type) {
    case 'cluster': return (
      <svg width={s} height={s} viewBox="0 0 32 32" fill="none">
        <circle cx="16" cy="16" r="13" fill={K8S} fillOpacity="0.15"/>
        <circle cx="16" cy="16" r="5" stroke={K8S} strokeWidth="2.2" fill="none"/>
        <circle cx="16" cy="16" r="2" fill={K8S}/>
        {[0,60,120,180,240,300].map((deg,i) => {
          const r = deg*Math.PI/180
          return <line key={i} x1={16+7*Math.sin(r)} y1={16-7*Math.cos(r)} x2={16+11*Math.sin(r)} y2={16-11*Math.cos(r)} stroke={K8S} strokeWidth="2" strokeLinecap="round"/>
        })}
      </svg>
    )
    case 'node': return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={K8S} strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="2" y="5" width="20" height="6" rx="1.5"/>
        <rect x="2" y="13" width="20" height="6" rx="1.5"/>
        <circle cx="5.5" cy="8" r="1.2" fill={K8S} stroke="none"/>
        <circle cx="5.5" cy="16" r="1.2" fill={K8S} stroke="none"/>
      </svg>
    )
    case 'namespace': return (
      <svg width={s} height={s} viewBox="0 0 32 32" fill="none">
        <polygon points="16,3 27,9.5 27,22.5 16,29 5,22.5 5,9.5" stroke={K8S} strokeWidth="2.2" fill={K8S} fillOpacity="0.12"/>
        <polygon points="16,9 22,12.5 22,19.5 16,23 10,19.5 10,12.5" stroke={K8S} strokeWidth="1.5" fill="none"/>
      </svg>
    )
    case 'deployment': return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={K8S} strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="5" width="18" height="4" rx="1.5"/>
        <rect x="3" y="11" width="18" height="4" rx="1.5"/>
        <rect x="3" y="17" width="18" height="3" rx="1.5" opacity="0.4"/>
      </svg>
    )
    case 'statefulset': return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={K8S} strokeWidth="1.8" strokeLinecap="round">
        <ellipse cx="12" cy="6" rx="7" ry="2.5"/>
        <path d="M5 6v12c0 1.38 3.13 2.5 7 2.5s7-1.12 7-2.5V6"/>
        <path d="M5 12c0 1.38 3.13 2.5 7 2.5s7-1.12 7-2.5"/>
      </svg>
    )
    case 'daemonset': return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={K8S} strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="2" y="13" width="7" height="7" rx="1.5"/>
        <rect x="8.5" y="4" width="7" height="7" rx="1.5"/>
        <rect x="15" y="13" width="7" height="7" rx="1.5"/>
        <line x1="9" y1="11" x2="6" y2="13"/>
        <line x1="15" y1="11" x2="18" y2="13"/>
      </svg>
    )
    case 'pod': return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={K8S} strokeWidth="1.8" strokeLinecap="round">
        <circle cx="12" cy="12" r="9"/>
        <circle cx="12" cy="12" r="4" fill={K8S} fillOpacity="0.2" stroke="none"/>
        <circle cx="12" cy="12" r="2" fill={K8S} stroke="none"/>
      </svg>
    )
    case 'k8s_service': return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={K8S} strokeWidth="1.8" strokeLinecap="round">
        <circle cx="12" cy="12" r="3.5"/>
        <line x1="12" y1="3" x2="12" y2="8.5"/>
        <line x1="12" y1="15.5" x2="12" y2="21"/>
        <line x1="3" y1="12" x2="8.5" y2="12"/>
        <line x1="15.5" y1="12" x2="21" y2="12"/>
        <line x1="5.6" y1="5.6" x2="9.2" y2="9.2"/>
        <line x1="14.8" y1="14.8" x2="18.4" y2="18.4"/>
        <line x1="18.4" y1="5.6" x2="14.8" y2="9.2"/>
        <line x1="9.2" y1="14.8" x2="5.6" y2="18.4"/>
      </svg>
    )
    case 'ingress': return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={K8S} strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="9"/>
        <path d="M12 3c-2 3-3 5.5-3 9s1 6 3 9"/>
        <path d="M12 3c2 3 3 5.5 3 9s-1 6-3 9"/>
        <path d="M3 12h18"/>
        <path d="M4 8h16" opacity="0.35"/>
        <path d="M4 16h16" opacity="0.35"/>
      </svg>
    )
    case 'job': return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={INK2} strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="9"/>
        <polyline points="8.5,12.5 11,15 16,9"/>
      </svg>
    )
    case 'cronjob': return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={INK2} strokeWidth="1.8" strokeLinecap="round">
        <circle cx="12" cy="12" r="9"/>
        <line x1="12" y1="7" x2="12" y2="12"/>
        <line x1="12" y1="12" x2="16" y2="14"/>
      </svg>
    )
    case 'container':
    case 'container_runtime': return (
      <svg width={s} height={s} viewBox="0 0 40 40" fill="none">
        <path d="M8 18h4v4H8zM13 18h4v4h-4zM18 18h4v4h-4zM18 13h4v4h-4zM13 13h4v4h-4zM23 18h4v4h-4zM23 13h4v4h-4z" fill={DOCK}/>
        <path d="M33 20c-.5-2-2.5-3-4-3h-1c-.5-3-3-4.5-5-5l-1-.5-.5 1c-.5 1-.7 2.5-.5 3.5H6a1 1 0 0 0-1 1 12 12 0 0 0 1 5c1 3 3 4.5 6 5 2 .5 4 .5 6 .5 2.5 0 5-.3 7-1.5 2-1 3.5-2.5 4.5-5h.5c1.5 0 3-.8 3.5-2l.5-1.5z" fill={DOCK}/>
      </svg>
    )
    case 'host': return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={INK2} strokeWidth="1.8" strokeLinecap="round">
        <rect x="2" y="4" width="20" height="13" rx="2"/>
        <path d="M8 21h8M12 17v4"/>
      </svg>
    )
    case 'pvc':
    case 'pv':
    case 'volume': return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={INK2} strokeWidth="1.8" strokeLinecap="round">
        <ellipse cx="12" cy="7" rx="8" ry="3"/>
        <path d="M4 7v5c0 1.66 3.58 3 8 3s8-1.34 8-3V7"/>
        <path d="M4 12v5c0 1.66 3.58 3 8 3s8-1.34 8-3v-5"/>
      </svg>
    )
    default: return (
      <svg width={s} height={s} viewBox="0 0 24 24" fill="none" stroke={INK3} strokeWidth="1.8" strokeLinecap="round">
        <rect x="4" y="4" width="16" height="16" rx="3"/>
      </svg>
    )
  }
}
