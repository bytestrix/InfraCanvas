'use client'

import { memo } from 'react'
import { Handle, Position, type NodeProps } from 'reactflow'
import { MoreVertical } from 'lucide-react'
import { type GroupNodeData } from '@/lib/graphPreprocess'
import NodeSvgIcon from './NodeSvgIcon'

/**
 * GroupNode — renders a collapsed type-group on the canvas.
 * Clicking opens a GroupDrawer (handled in InfraCanvas via onNodeClick).
 */
const GroupNode = memo(({ id, data, selected }: NodeProps<GroupNodeData>) => {
  const { groupType, label, count, healthCounts, isCritical, onOpenPicker } = data

  const total = Math.max(count, 1)
  const pHealthy   = (healthCounts.healthy   / total) * 100
  const pDegraded  = (healthCounts.degraded  / total) * 100
  const pUnhealthy = (healthCounts.unhealthy / total) * 100
  const hasBadHealth = healthCounts.unhealthy > 0 || healthCounts.degraded > 0

  return (
    <div style={{
      width: 260,
      background: 'var(--surface)',
      border: `1px solid ${selected ? 'var(--ink)' : hasBadHealth ? 'var(--line3)' : 'var(--line)'}`,
      borderRadius: 10,
      boxShadow: selected ? '0 0 0 1px var(--ink), 0 4px 20px rgba(0,0,0,0.15)' : '0 2px 12px rgba(0,0,0,0.1)',
      position: 'relative',
      overflow: 'hidden',
      transition: 'border-color 0.12s, box-shadow 0.12s',
      cursor: 'pointer',
    }}>
      {/* Health bar top — semantic colors */}
      <div style={{ display: 'flex', height: 2, width: '100%' }}>
        {pHealthy   > 0 && <div style={{ flex: pHealthy,   background: '#22c55e' }} title={`${healthCounts.healthy} healthy`} />}
        {pDegraded  > 0 && <div style={{ flex: pDegraded,  background: '#f59e0b' }} title={`${healthCounts.degraded} degraded`} />}
        {pUnhealthy > 0 && <div style={{ flex: pUnhealthy, background: '#ef4444' }} title={`${healthCounts.unhealthy} unhealthy`} />}
        {(100 - pHealthy - pDegraded - pUnhealthy) > 0 && <div style={{ flex: (100 - pHealthy - pDegraded - pUnhealthy), background: 'var(--line)' }} />}
      </div>

      {/* Critical pulse */}
      {isCritical && (
        <div style={{
          position: 'absolute', top: 6, right: 6,
          width: 6, height: 6, borderRadius: '50%',
          background: 'var(--ink)', border: '1px solid var(--surface)',
          animation: 'pulse 1.5s ease-in-out infinite',
          zIndex: 10,
        }} />
      )}

      <div style={{ padding: '10px 12px 10px 12px' }}>
        {/* Row 1: icon + label + count */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <div style={{ width: 22, height: 22, borderRadius: 6, background: 'var(--surface-2)', border: '1px solid var(--line2)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
            <NodeSvgIcon type={groupType} size={14} />
          </div>
          <span style={{
            fontSize: 12, fontWeight: 600, color: 'var(--ink)', flex: 1,
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            letterSpacing: '-0.01em',
          }}>{label}</span>
          <span style={{
            fontSize: 11, fontWeight: 700, color: 'var(--ink2)',
            background: 'var(--line)', border: '1px solid var(--line2)',
            borderRadius: 20, padding: '1px 8px', flexShrink: 0,
            fontFamily: 'var(--font-geist-mono,"Geist Mono","JetBrains Mono",ui-monospace,monospace)',
          }}>×{count}</span>
          {onOpenPicker && (
            <button
              onClick={(e) => { e.stopPropagation(); onOpenPicker(id, e) }}
              title="Show individual members"
              style={{
                width: 20, height: 20, borderRadius: 5, flexShrink: 0,
                background: 'transparent', border: 'none', color: 'var(--ink4)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer',
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--line)'; e.currentTarget.style.color = 'var(--ink2)' }}
              onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--ink4)' }}
            >
              <MoreVertical size={13} strokeWidth={1.5} />
            </button>
          )}
        </div>

        {/* Row 2: health summary */}
        <div style={{ display: 'flex', gap: 8 }}>
          {healthCounts.healthy > 0 && (
            <span style={{ fontSize: 10, color: '#22c55e', fontFamily: 'var(--font-geist-mono,"Geist Mono","JetBrains Mono",ui-monospace,monospace)' }}>
              {healthCounts.healthy} ok
            </span>
          )}
          {healthCounts.degraded > 0 && (
            <span style={{ fontSize: 10, color: '#f59e0b', fontFamily: 'var(--font-geist-mono,"Geist Mono","JetBrains Mono",ui-monospace,monospace)' }}>
              {healthCounts.degraded} warn
            </span>
          )}
          {healthCounts.unhealthy > 0 && (
            <span style={{ fontSize: 10, color: '#ef4444', fontFamily: 'var(--font-geist-mono,"Geist Mono","JetBrains Mono",ui-monospace,monospace)' }}>
              {healthCounts.unhealthy} err
            </span>
          )}
        </div>
      </div>

      <Handle type="target" position={Position.Top} style={{ background: 'var(--line2)', width: 5, height: 5, border: '1px solid var(--surface)' }} />
      <Handle type="source" position={Position.Bottom} style={{ background: 'var(--line2)', width: 5, height: 5, border: '1px solid var(--surface)' }} />
    </div>
  )
})

GroupNode.displayName = 'GroupNode'
export default GroupNode
