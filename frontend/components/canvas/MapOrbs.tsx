'use client'

import { memo } from 'react'
import { Handle, Position, type NodeProps } from 'reactflow'
import NodeSvgIcon from './NodeSvgIcon'
import { ORB_D, ORB_CELL_W, ORB_CELL_H, HUB_D, ROOT_D } from '@/lib/hubMap'

/**
 * Node size tracks how connected a node is, so importance is legible without
 * hovering. sqrt rather than linear because degree is heavily skewed — a
 * namespace with 40 members is 40× a leaf, which linear scaling would render
 * as an unusable blob while every leaf stayed invisible.
 */
const orbSize = (deg: number) => Math.round(Math.min(44, Math.max(18, 18 + Math.sqrt(Math.max(0, deg)) * 5)))
const hubSize = (deg: number) => Math.round(Math.min(104, Math.max(46, 46 + Math.sqrt(Math.max(0, deg)) * 3.2)))

/** The Kubernetes helm. It was dropped when the map nodes were first written,
 *  which left the centre of the map as a yellow disc with a word on it. */
function Helm({ size, color = '#0b0b12' }: { size: number; color?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden>
      <path d="M12 1.6 21.2 6.8v10.4L12 22.4 2.8 17.2V6.8L12 1.6z" fill="#fff" fillOpacity="0.92" />
      <circle cx="12" cy="7.6" r="1.25" fill={color} />
      <circle cx="8.1" cy="13.6" r="1.25" fill={color} />
      <circle cx="15.9" cy="13.6" r="1.25" fill={color} />
      <circle cx="12" cy="17.1" r="1.25" fill={color} />
      <path d="M12 8.9v2.3m0 0-2.8 1.6m2.8-1.6 2.8 1.6m-2.8 1.9v1.9" stroke={color} strokeWidth="1.1" />
    </svg>
  )
}

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

/** Centre: the platform. Sized by how much it owns. */
export const MapRoot = memo(({ data, selected }: NodeProps<{ label: string; count: number; color: string }>) => {
  const d = Math.round(Math.min(104, Math.max(76, 76 + Math.sqrt(Math.max(1, data.count)) * 2.4)))
  return (
    <div style={{
      display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 5,
      width: d + 18,
    }}>
      <Handle type="source" position={Position.Bottom}
        style={{ opacity: 0, width: 1, height: 1, border: 'none', background: 'transparent' }} />
      <div style={{
        width: d, height: d, borderRadius: '50%',
        background: data.color, border: `2px solid ${selected ? '#fff' : 'rgba(255,255,255,0.8)'}`,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        boxShadow: `0 0 0 9px ${data.color}22, 0 4px 18px rgba(0,0,0,0.6)`,
      }}>
        {data.label === 'Kubernetes'
          ? <Helm size={Math.round(d * 0.62)} />
          : <span style={{ fontSize: 12, fontWeight: 750, color: '#0b0b12' }}>{data.label}</span>}
      </div>
      <span style={{ fontSize: 11, fontWeight: 650, color: 'var(--ink)', letterSpacing: '-0.01em' }}>
        {data.label}
      </span>
      <span style={{
        fontSize: 9.5, color: 'var(--ink4)',
        fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
      }}>{data.count} resources</span>
    </div>
  )
})
MapRoot.displayName = 'MapRoot'

/** One per resource type, on a ring around the root. */
export const MapHub = memo(({ data, selected }: NodeProps<{ label: string; kind: string; count: number; color: string; degree?: number }>) => {
  const color = data.color || '#64748b'
  const d = hubSize(data.degree ?? data.count)
  return (
    <div style={{
      display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 4,
      width: Math.max(HUB_D, d) + 18,
    }}>
      <Handle type="target" position={Position.Top}
        style={{ opacity: 0, width: 1, height: 1, border: 'none', background: 'transparent' }} />
      <Handle type="source" position={Position.Bottom}
        style={{ opacity: 0, width: 1, height: 1, border: 'none', background: 'transparent' }} />
      <div style={{
        width: d, height: d, borderRadius: '50%',
        background: `${color}22`, border: `2px solid ${color}`,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        boxShadow: `0 0 0 5px ${color}12, 0 3px 12px rgba(0,0,0,0.55)`,
        opacity: selected ? 1 : 0.95,
      }}>
        <NodeSvgIcon type={data.kind} size={Math.round(d * 0.4)} />
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
export const MapOrb = memo(({ data, selected }: NodeProps<{ nodeType: string; label: string; health: string; metadata?: Record<string, any>; degree?: number }>) => {
  const ring = RING[data.health] ?? RING.unknown
  const d = orbSize(data.degree ?? 0)
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
        width: d, height: d, borderRadius: '50%', flexShrink: 0,
        background: 'var(--surface)', border: `2px solid ${ring}`,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        boxShadow: selected ? `0 0 0 3px ${ring}55` : '0 2px 6px rgba(0,0,0,0.5)',
      }}>
        <NodeSvgIcon type={data.nodeType} size={Math.round(d * 0.5)} />
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
