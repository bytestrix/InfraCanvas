'use client'

import { memo } from 'react'
import { Handle, Position, type NodeProps } from 'reactflow'
import { type GroupNodeData } from '@/lib/graphPreprocess'

/**
 * GroupNode — renders a collapsed type-group on the canvas.
 *
 * Shows:
 *   - Left colour accent bar
 *   - Type icon + label
 *   - Count badge
 *   - Proportional health bar (healthy green / degraded amber / unhealthy red / unknown gray)
 *   - Health summary badges
 *
 * Clicking opens a GroupDrawer (handled in InfraCanvas via onNodeClick).
 */
const GroupNode = memo(({ data, selected }: NodeProps<GroupNodeData>) => {
  const { color, icon, label, count, healthCounts, isCritical } = data

  const total = Math.max(count, 1)
  const pHealthy   = (healthCounts.healthy   / total) * 100
  const pDegraded  = (healthCounts.degraded  / total) * 100
  const pUnhealthy = (healthCounts.unhealthy / total) * 100
  const pUnknown   = (healthCounts.unknown   / total) * 100

  const hasBadHealth = healthCounts.unhealthy > 0 || healthCounts.degraded > 0

  // Dominant health → border + glow colour
  const accentColor = healthCounts.unhealthy > 0
    ? '#ef4444'
    : healthCounts.degraded > 0
      ? '#f59e0b'
      : color

  const borderColor = selected
    ? accentColor
    : hasBadHealth
      ? `${accentColor}60`
      : '#2a2a4a'

  const glowColor = selected
    ? `${accentColor}22`
    : healthCounts.unhealthy > 0
      ? 'rgba(239,68,68,0.06)'
      : healthCounts.degraded > 0
        ? 'rgba(245,158,11,0.05)'
        : 'transparent'

  return (
    <div
      style={{
        width: 260,
        background: '#0e0e1a',
        border: `1px solid ${borderColor}`,
        borderRadius: 10,
        boxShadow: selected
          ? `0 0 0 2px ${accentColor}33, 0 4px 20px rgba(0,0,0,0.6)`
          : `0 2px 14px rgba(0,0,0,0.5), inset 0 0 0 100px ${glowColor}`,
        position: 'relative',
        overflow: 'hidden',
        transition: 'border-color 0.15s, box-shadow 0.15s',
        cursor: 'pointer',
      }}
    >
      {/* Critical pulse dot — shown when >50% of group is degraded/unhealthy */}
      {isCritical && (
        <div
          style={{
            position: 'absolute',
            top: -4,
            right: -4,
            width: 10,
            height: 10,
            borderRadius: '50%',
            background: healthCounts.unhealthy > 0 ? '#ef4444' : '#f59e0b',
            border: '2px solid #070711',
            zIndex: 10,
            animation: 'pulse 1.5s ease-in-out infinite',
          }}
        />
      )}
      {/* Left accent bar */}
      <div
        style={{
          position: 'absolute',
          left: 0,
          top: 0,
          bottom: 0,
          width: 3,
          background: color,
          borderRadius: '10px 0 0 10px',
        }}
      />

      {/* Main content */}
      <div style={{ paddingLeft: 14, paddingRight: 12, paddingTop: 10, paddingBottom: 10 }}>

        {/* Row 1: icon + label + count badge */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <span style={{ fontSize: 15, color, fontFamily: 'system-ui', flexShrink: 0 }}>
            {icon}
          </span>
          <span
            style={{
              fontSize: 12,
              fontWeight: 600,
              color: '#e2e8f0',
              flex: 1,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {label}
          </span>
          {/* Count badge */}
          <span
            style={{
              fontSize: 11,
              fontWeight: 700,
              color: color,
              background: `${color}18`,
              border: `1px solid ${color}30`,
              borderRadius: 20,
              padding: '1px 8px',
              flexShrink: 0,
              fontFamily: 'JetBrains Mono, monospace',
            }}
          >
            ×{count}
          </span>
        </div>

        {/* Row 2: health bar */}
        <div
          style={{
            display: 'flex',
            height: 5,
            borderRadius: 3,
            overflow: 'hidden',
            gap: 1.5,
            marginBottom: 7,
          }}
        >
          {pHealthy > 0 && (
            <div
              title={`${healthCounts.healthy} healthy`}
              style={{ width: `${pHealthy}%`, background: '#10b981', borderRadius: 2 }}
            />
          )}
          {pDegraded > 0 && (
            <div
              title={`${healthCounts.degraded} degraded`}
              style={{ width: `${pDegraded}%`, background: '#f59e0b', borderRadius: 2 }}
            />
          )}
          {pUnhealthy > 0 && (
            <div
              title={`${healthCounts.unhealthy} unhealthy`}
              style={{ width: `${pUnhealthy}%`, background: '#ef4444', borderRadius: 2 }}
            />
          )}
          {pUnknown > 0 && (
            <div
              title={`${healthCounts.unknown} unknown`}
              style={{ width: `${pUnknown}%`, background: '#334155', borderRadius: 2 }}
            />
          )}
        </div>

        {/* Row 3: health summary text */}
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {healthCounts.healthy > 0 && (
            <span style={{ fontSize: 10, color: '#10b981', display: 'flex', alignItems: 'center', gap: 3 }}>
              <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#10b981', display: 'inline-block' }} />
              {healthCounts.healthy}
            </span>
          )}
          {healthCounts.degraded > 0 && (
            <span style={{ fontSize: 10, color: '#f59e0b', display: 'flex', alignItems: 'center', gap: 3 }}>
              <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#f59e0b', display: 'inline-block' }} />
              {healthCounts.degraded}
            </span>
          )}
          {healthCounts.unhealthy > 0 && (
            <span style={{ fontSize: 10, color: '#ef4444', display: 'flex', alignItems: 'center', gap: 3 }}>
              <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#ef4444', display: 'inline-block' }} />
              {healthCounts.unhealthy}
            </span>
          )}
          {healthCounts.unknown > 0 && (
            <span style={{ fontSize: 10, color: '#475569', display: 'flex', alignItems: 'center', gap: 3 }}>
              <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#475569', display: 'inline-block' }} />
              {healthCounts.unknown}
            </span>
          )}
          <span style={{ fontSize: 10, color: '#2d2d52', marginLeft: 'auto' }}>
            click to expand →
          </span>
        </div>
      </div>

      <Handle
        type="target"
        position={Position.Top}
        style={{ background: color, width: 7, height: 7, border: '1px solid #070711' }}
      />
      <Handle
        type="source"
        position={Position.Bottom}
        style={{ background: color, width: 7, height: 7, border: '1px solid #070711' }}
      />
    </div>
  )
})

GroupNode.displayName = 'GroupNode'
export default GroupNode
