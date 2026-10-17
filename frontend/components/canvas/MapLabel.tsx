'use client'

import { memo } from 'react'
import { type NodeProps } from 'reactflow'

/**
 * Header for a block in the structured map.
 *
 * The point of the structured layout is that the block *says what it is* — a
 * namespace name and how much it holds — so a wall of individual nodes is
 * still navigable. Without this the map is just as anonymous as the radial
 * version, just rectangular.
 */

const KIND_STYLE: Record<string, { color: string; bg: string; border: string }> = {
  ns: { color: '#a5b4fc', bg: 'rgba(99,102,241,0.10)', border: 'rgba(99,102,241,0.30)' },
  type: { color: '#94a3b8', bg: 'rgba(100,116,139,0.10)', border: 'rgba(100,116,139,0.28)' },
}

const MapLabel = memo(({ data }: NodeProps<{ label: string; sub: string; kind: string }>) => {
  const s = KIND_STYLE[data.kind] ?? KIND_STYLE.type
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 8,
      height: '100%', width: '100%', pointerEvents: 'none',
      padding: '0 10px',
      background: s.bg,
      border: `1px solid ${s.border}`,
      borderRadius: 7,
    }}>
      <span style={{
        fontSize: 12, fontWeight: 650, color: s.color,
        letterSpacing: '-0.01em', whiteSpace: 'nowrap',
        overflow: 'hidden', textOverflow: 'ellipsis',
      }}>{data.label}</span>
      <span style={{
        fontSize: 10, color: 'var(--ink4)',
        fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
        whiteSpace: 'nowrap', flexShrink: 0,
      }}>{data.sub}</span>
    </div>
  )
})

MapLabel.displayName = 'MapLabel'
export default MapLabel
