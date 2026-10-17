'use client'

import { memo } from 'react'
import { Handle, Position, type NodeProps } from 'reactflow'
import { HIER_ROOT_W, HIER_ROOT_H, HIER_TYPE_W, HIER_TYPE_H } from '@/lib/hierarchyMap'
import NodeSvgIcon from './NodeSvgIcon'

/** Platform branding. The Kubernetes helm is the one logo worth drawing — it
 *  is what makes the top of the tree read as "this is Kubernetes" at a glance
 *  rather than as a generic box labelled with a word. */
function PlatformMark({ platform, size }: { platform: string; size: number }) {
  if (platform === 'Kubernetes') {
    return (
      <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden>
        <path d="M12 1.6 21.2 6.8v10.4L12 22.4 2.8 17.2V6.8L12 1.6z" fill="#326CE5" />
        <path d="M12 4.6 18.4 8.2v7.6L12 19.4 5.6 15.8V8.2L12 4.6z" fill="#fff" />
        <circle cx="12" cy="8" r="1.15" fill="#326CE5" />
        <circle cx="8.4" cy="13.4" r="1.15" fill="#326CE5" />
        <circle cx="15.6" cy="13.4" r="1.15" fill="#326CE5" />
        <circle cx="12" cy="16.4" r="1.15" fill="#326CE5" />
        <path d="M12 9.1v2.2m0 0-2.5 1.4m2.5-1.4 2.5 1.4m-2.5 1.6v1.9" stroke="#326CE5" strokeWidth="1" />
      </svg>
    )
  }
  if (platform === 'Docker') {
    return (
      <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden>
        <rect x="3" y="10" width="3.1" height="3.1" rx="0.5" fill="#2496ED" />
        <rect x="6.9" y="10" width="3.1" height="3.1" rx="0.5" fill="#2496ED" />
        <rect x="10.8" y="10" width="3.1" height="3.1" rx="0.5" fill="#2496ED" />
        <rect x="14.7" y="10" width="3.1" height="3.1" rx="0.5" fill="#2496ED" />
        <rect x="14.7" y="6.1" width="3.1" height="3.1" rx="0.5" fill="#2496ED" />
        <rect x="14.7" y="13.9" width="3.1" height="3.1" rx="0.5" fill="#2496ED" />
        <path d="M3 13.9c0 3.4 2.8 6.1 6.2 6.1 4.9 0 8.4-2.5 9.8-6.1H3z" fill="#2496ED" />
      </svg>
    )
  }
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden>
      <rect x="3" y="4" width="18" height="6" rx="1.6" stroke="#64748b" strokeWidth="1.6" />
      <rect x="3" y="14" width="18" height="6" rx="1.6" stroke="#64748b" strokeWidth="1.6" />
      <circle cx="7" cy="7" r="1.1" fill="#64748b" />
      <circle cx="7" cy="17" r="1.1" fill="#64748b" />
    </svg>
  )
}

/** Top-of-tree platform node: Kubernetes / Docker / Host. */
export const MapRoot = memo(({ data, selected }: NodeProps<{ label: string; count: number }>) => {
  return (
    <div style={{
      width: HIER_ROOT_W, height: HIER_ROOT_H,
      display: 'flex', alignItems: 'center', gap: 11,
      padding: '0 16px',
      background: 'var(--surface)',
      border: `1px solid ${selected ? 'var(--ink2)' : 'var(--line2)'}`,
      borderRadius: 12,
      boxShadow: '0 4px 18px rgba(0,0,0,0.55)',
      position: 'relative',
    }}>
      <Handle type="source" position={Position.Bottom}
        style={{ background: 'var(--line3)', width: 6, height: 6 }} />
      <PlatformMark platform={data.label} size={28} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{
          fontSize: 14.5, fontWeight: 700, color: 'var(--ink)',
          letterSpacing: '-0.02em', whiteSpace: 'nowrap',
          overflow: 'hidden', textOverflow: 'ellipsis',
        }}>{data.label}</div>
        <div style={{
          fontSize: 10.5, color: 'var(--ink4)', marginTop: 1,
          fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
        }}>{data.count} resources</div>
      </div>
    </div>
  )
})
MapRoot.displayName = 'MapRoot'

/** Resource-type node, e.g. "Pods · 43", hanging under a platform. */
export const MapType = memo(({ data, selected }: NodeProps<{ label: string; kind: string; count: number }>) => {
  return (
    <div style={{
      width: HIER_TYPE_W, height: HIER_TYPE_H,
      display: 'flex', alignItems: 'center', gap: 8,
      padding: '0 10px',
      background: 'var(--surface)',
      border: `1px solid ${selected ? 'var(--ink2)' : 'var(--line2)'}`,
      borderRadius: 9,
      boxShadow: '0 2px 10px rgba(0,0,0,0.5)',
      position: 'relative',
    }}>
      <Handle type="target" position={Position.Top}
        style={{ background: 'var(--line3)', width: 5, height: 5 }} />
      <Handle type="source" position={Position.Bottom}
        style={{ background: 'var(--line3)', width: 5, height: 5 }} />
      <NodeSvgIcon type={data.kind} size={14} />
      <span style={{
        fontSize: 11.5, fontWeight: 600, color: 'var(--ink)',
        flex: 1, minWidth: 0, letterSpacing: '-0.01em',
        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
      }}>{data.label}</span>
      <span style={{
        fontSize: 10, color: 'var(--ink4)', flexShrink: 0,
        fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
      }}>{data.count}</span>
    </div>
  )
})
MapType.displayName = 'MapType'
