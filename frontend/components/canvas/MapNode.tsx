'use client'

import { memo, useState } from 'react'
import { Handle, Position, type NodeProps } from 'reactflow'
import { type InfraNodeData } from './InfraNode'
import NodeSvgIcon from './NodeSvgIcon'
import { MAP_NODE_WIDTH, MAP_NODE_HEIGHT } from '@/lib/radialMap'

/**
 * A single node in the map view.
 *
 * Deliberately not a card. The point of this view is that the whole inventory
 * is legible at once, so a node is a 40px disc with its name beside it and
 * nothing else. Everything else is one hover away — the tooltip carries the
 * fields that would otherwise need a 260x92 box to show, which is exactly the
 * box that made the grouped canvas unreadable.
 */

const RING: Record<string, string> = {
  healthy: '#22c55e',
  degraded: '#f59e0b',
  unhealthy: '#ef4444',
  unknown: '#6b7280',
}

/** Nodes worth calling out visually even when healthy. */
const ACCENT: Record<string, string> = {
  host: '#64748b',
  cluster: '#326CE5',
  namespace: '#6366f1',
}

const META_ROWS: Array<[string, string]> = [
  ['kind', 'kind'],
  ['type', 'type'],
  ['namespace', 'ns'],
  ['status', 'state'],
  ['node', 'node'],
  ['pod', 'pod'],
  ['container', 'container'],
  ['image', 'image'],
  ['ready', 'ready'],
  ['restarts', 'restarts'],
  ['version', 'version'],
  ['address', 'address'],
]

function rowsFor(meta: Record<string, any> | undefined): Array<[string, string]> {
  if (!meta) return []
  const out: Array<[string, string]> = []
  for (const [k, label] of META_ROWS) {
    const v = meta[k]
    if (v === undefined || v === null || v === '') continue
    const s = typeof v === 'object' ? JSON.stringify(v) : String(v)
    if (s.length > 90) continue
    out.push([label, s])
  }
  return out
}

const MapNode = memo(({ data, selected }: NodeProps<InfraNodeData>) => {
  const [hover, setHover] = useState(false)
  const { nodeType, label, health, metadata } = data
  const ring = RING[health] ?? RING.unknown
  const fill = ACCENT[nodeType] ?? ring
  const rows = rowsFor(metadata)

  return (
    <div
      style={{
        position: 'relative',
        display: 'flex', alignItems: 'center', gap: 8,
        width: MAP_NODE_WIDTH, height: MAP_NODE_HEIGHT,
        pointerEvents: 'all',
      }}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      <Handle type="target" position={Position.Left}
        style={{ opacity: 0, width: 1, height: 1, border: 'none', background: 'transparent' }} />
      <Handle type="source" position={Position.Right}
        style={{ opacity: 0, width: 1, height: 1, border: 'none', background: 'transparent' }} />

      {/* Disc */}
      <div style={{
        width: 40, height: 40, borderRadius: '50%', flexShrink: 0,
        background: 'var(--surface)',
        border: `2px solid ${ring}`,
        boxShadow: health === 'unhealthy'
          ? `0 0 0 4px ${ring}22, 0 2px 10px rgba(0,0,0,0.6)`
          : '0 2px 10px rgba(0,0,0,0.6)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        position: 'relative',
      }}>
        <NodeSvgIcon type={nodeType} size={17} />
        {health !== 'healthy' && (
          <span style={{
            position: 'absolute', top: -2, right: -2,
            width: 9, height: 9, borderRadius: '50%',
            background: ring, border: '1.5px solid var(--surface)',
          }} />
        )}
      </div>

      {/* Name, always visible */}
      <div style={{ minWidth: 0, flex: 1 }}>
        <div style={{
          fontSize: 11.5, fontWeight: 550, color: 'var(--ink)',
          letterSpacing: '-0.01em', lineHeight: 1.25,
          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
          ...(selected ? { color: 'var(--ink)' } : {}),
        }}>{label}</div>
        <div style={{
          fontSize: 9.5, color: 'var(--ink4)', marginTop: 1,
          fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
        }}>{nodeType}{metadata?.namespace ? ` · ${metadata.namespace}` : ''}</div>
      </div>

      {/* Hover detail */}
      {hover && (
        <div style={{
          position: 'absolute', left: 0, top: '100%', zIndex: 30,
          marginTop: 6, minWidth: 220, maxWidth: 320,
          background: 'var(--surface)', border: '1px solid var(--line2)',
          borderRadius: 8, padding: '9px 11px',
          boxShadow: '0 8px 28px rgba(0,0,0,0.65)',
          pointerEvents: 'none',
        }}>
          <div style={{
            fontSize: 11.5, fontWeight: 600, color: 'var(--ink)',
            marginBottom: 2, wordBreak: 'break-word',
          }}>{label}</div>
          <div style={{
            fontSize: 9.5, color: ring, marginBottom: 6, textTransform: 'uppercase',
            letterSpacing: '0.05em', fontWeight: 600,
          }}>{nodeType} · {health}</div>
          {rows.length > 0 ? rows.map(([k, v]) => (
            <div key={k} style={{ display: 'flex', gap: 8, marginTop: 2 }}>
              <span style={{
                fontSize: 10, color: 'var(--ink4)', width: 62, flexShrink: 0,
                fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
              }}>{k}</span>
              <span style={{
                fontSize: 10, color: 'var(--ink2)', wordBreak: 'break-all',
                fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
              }}>{v}</span>
            </div>
          )) : (
            <div style={{ fontSize: 10, color: 'var(--ink4)' }}>No additional detail</div>
          )}
        </div>
      )}
    </div>
  )
})

MapNode.displayName = 'MapNode'
export default MapNode
