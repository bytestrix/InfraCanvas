'use client'

import { memo } from 'react'
import { Handle, Position, type NodeProps } from 'reactflow'
import { type GroupNodeData } from '@/lib/graphPreprocess'
import NodeSvgIcon from './NodeSvgIcon'

/**
 * GroupNode — renders a collapsed type-group on the canvas.
 * Clicking opens a GroupDrawer (handled in InfraCanvas via onNodeClick).
 */
const GroupNode = memo(({ data, selected }: NodeProps<GroupNodeData>) => {
  const { groupType, label, count, healthCounts, isCritical } = data

  const total = Math.max(count, 1)
  const pHealthy   = (healthCounts.healthy   / total) * 100
  const pDegraded  = (healthCounts.degraded  / total) * 100
  const pUnhealthy = (healthCounts.unhealthy / total) * 100
  const hasBadHealth = healthCounts.unhealthy > 0 || healthCounts.degraded > 0

  return (
    <div style={{
      width: 260,
      background: '#111111',
      border: `1px solid ${selected ? '#FAFAFA' : hasBadHealth ? '#383838' : '#1E1E1E'}`,
      borderRadius: 10,
      boxShadow: selected ? '0 0 0 1px #FAFAFA, 0 4px 20px rgba(0,0,0,0.6)' : '0 2px 12px rgba(0,0,0,0.5)',
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
        {(100 - pHealthy - pDegraded - pUnhealthy) > 0 && <div style={{ flex: (100 - pHealthy - pDegraded - pUnhealthy), background: '#1E1E1E' }} />}
      </div>

      {/* Critical pulse */}
      {isCritical && (
        <div style={{
          position: 'absolute', top: 6, right: 6,
          width: 6, height: 6, borderRadius: '50%',
          background: '#FAFAFA', border: '1px solid #111111',
          animation: 'pulse 1.5s ease-in-out infinite',
          zIndex: 10,
        }} />
      )}

      <div style={{ padding: '10px 12px 10px 12px' }}>
        {/* Row 1: icon + label + count */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <div style={{ width: 22, height: 22, borderRadius: 6, background: '#1A1A1A', border: '1px solid #2A2A2A', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
            <NodeSvgIcon type={groupType} size={14} />
          </div>
          <span style={{
            fontSize: 12, fontWeight: 600, color: '#FAFAFA', flex: 1,
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            letterSpacing: '-0.01em',
          }}>{label}</span>
          <span style={{
            fontSize: 11, fontWeight: 700, color: '#A1A1A1',
            background: '#1E1E1E', border: '1px solid #2A2A2A',
            borderRadius: 20, padding: '1px 8px', flexShrink: 0,
            fontFamily: 'var(--font-geist-mono,"Geist Mono","JetBrains Mono",ui-monospace,monospace)',
          }}>×{count}</span>
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

      <Handle type="target" position={Position.Top} style={{ background: '#2A2A2A', width: 5, height: 5, border: '1px solid #111111' }} />
      <Handle type="source" position={Position.Bottom} style={{ background: '#2A2A2A', width: 5, height: 5, border: '1px solid #111111' }} />
    </div>
  )
})

GroupNode.displayName = 'GroupNode'
export default GroupNode
