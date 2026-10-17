'use client'

import { memo } from 'react'
import { Handle, Position, type NodeProps } from 'reactflow'
import NodeSvgIcon from './NodeSvgIcon'
import { ORB_D, ORB_CELL_W, ORB_CELL_H, HUB_D, ROOT_D } from '@/lib/hubMap'

/**
 * The three shapes in the map.
 *
 * A **root** (Kubernetes) in the middle, a **hub** per resource type around it,
 * and small **orbs** for individual resources around each hub. Deliberately
 * circles rather than cards — the whole point is that a glance tells you
 * "those are pods, and they belong to that hub".
 */

const RING: Record<string, string> = {
  healthy: '#22c55e', degraded: '#f59e0b', unhealthy: '#ef4444', unknown: '#6b7280',
}

function Label({ text, size = 10.5, weight = 550 }: { text: string; size?: number; weight?: number }) {
  return (
    <span style={{
      fontSize: size, fontWeight: weight, color: 'var(--ink)',
      letterSpacing: '-0.01em',
      overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
    }}>{text}</span>
  )
}

/** Centre: the platform. */
export const MapRoot = memo(({ data, selected }: NodeProps<{ label: string; count: number; color: string }>) => {
  return (
    <div style={{
      display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 5,
      width: ROOT_D + 16,
    }}>
      <Handle type="source" position={Position.Bottom}
        style={{ opacity: 0, width: 1, height: 1, border: 'none', background: 'transparent' }} />
      <div style={{
        width: ROOT_D, height: ROOT_D, borderRadius: '50%',
        background: data.color, border: `2px solid ${selected ? '#fff' : 'rgba(255,255,255,0.75)'}`,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        boxShadow: `0 0 0 8px ${data.color}22, 0 4px 18px rgba(0,0,0,0.6)`,
      }}>
        <span style={{ fontSize: 12, fontWeight: 750, color: '#0b0b12', letterSpacing: '-0.02em' }}>
          {data.label}
        </span>
      </div>
      <span style={{
        fontSize: 9.5, color: 'var(--ink4)',
        fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
      }}>{data.count} resources</span>
    </div>
  )
})
MapRoot.displayName = 'MapRoot'

/** One per resource type, on a ring around the root. */
export const MapHub = memo(({ data, selected }: NodeProps<{ label: string; kind: string; count: number; color: string }>) => {
  const color = data.color || '#64748b'
  return (
    <div style={{
      display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4,
      width: HUB_D + 16,
    }}>
      <Handle type="target" position={Position.Top}
        style={{ opacity: 0, width: 1, height: 1, border: 'none', background: 'transparent' }} />
      <Handle type="source" position={Position.Bottom}
        style={{ opacity: 0, width: 1, height: 1, border: 'none', background: 'transparent' }} />
      <div style={{
        width: HUB_D, height: HUB_D, borderRadius: '50%',
        background: `${color}22`, border: `2px solid ${color}`,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        boxShadow: `0 0 0 5px ${color}12, 0 3px 12px rgba(0,0,0,0.55)`,
        opacity: selected ? 1 : 0.95,
      }}>
        <NodeSvgIcon type={data.kind} size={22} />
      </div>
      <span style={{
        fontSize: 10.5, fontWeight: 650, color, letterSpacing: '-0.01em',
        whiteSpace: 'nowrap',
      }}>{data.label}</span>
      <span style={{
        fontSize: 9.5, color: 'var(--ink4)',
        fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
      }}>{data.count}</span>
    </div>
  )
})
MapHub.displayName = 'MapHub'

/** An individual resource. Hover for detail. */
export const MapOrb = memo(({ data, selected }: NodeProps<{ nodeType: string; label: string; health: string; metadata?: Record<string, any> }>) => {
  const ring = RING[data.health] ?? RING.unknown
  const ns = typeof data.metadata?.namespace === 'string' ? data.metadata.namespace : ''
  return (
    <div
      title={`${data.label} · ${data.nodeType} · ${data.health}`}
      style={{
        display: 'flex', alignItems: 'center', gap: 7,
        width: ORB_CELL_W, height: ORB_CELL_H,
      }}
    >
      <Handle type="target" position={Position.Left}
        style={{ opacity: 0, width: 1, height: 1, border: 'none', background: 'transparent' }} />
      <div style={{
        width: ORB_D, height: ORB_D, borderRadius: '50%', flexShrink: 0,
        background: 'var(--surface)', border: `2px solid ${ring}`,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        boxShadow: selected ? `0 0 0 3px ${ring}55` : '0 2px 6px rgba(0,0,0,0.5)',
      }}>
        <NodeSvgIcon type={data.nodeType} size={12} />
      </div>
      <div style={{ minWidth: 0, flex: 1 }}>
        <Label text={data.label} />
        <div style={{
          fontSize: 8.5, color: 'var(--ink4)', marginTop: 0,
          fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
        }}>{data.nodeType}{ns ? ` · ${ns}` : ''}</div>
      </div>
    </div>
  )
})
MapOrb.displayName = 'MapOrb'
