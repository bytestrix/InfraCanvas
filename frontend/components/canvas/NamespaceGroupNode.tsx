'use client'

import { NodeProps } from 'reactflow'
import { Handle, Position } from 'reactflow'
import { InfraNodeData } from './InfraNode'

/**
 * Namespace group node — renders as a labeled container that wraps child nodes.
 * ReactFlow positions child nodes (with parentNode: this.id) inside this box.
 */
export default function NamespaceGroupNode({ data, selected }: NodeProps<InfraNodeData>) {
  const healthColor =
    data.health === 'healthy'
      ? '#10b981'
      : data.health === 'degraded'
        ? '#f59e0b'
        : data.health === 'unhealthy'
          ? '#ef4444'
          : '#475569'

  const borderColor = selected ? '#6366f1' : '#2a2a4a'

  return (
    <div
      style={{
        width: '100%',
        height: '100%',
        borderRadius: 10,
        border: `1.5px dashed ${borderColor}`,
        background: 'rgba(99,102,241,0.025)',
        position: 'relative',
        boxSizing: 'border-box',
        pointerEvents: 'all',
      }}
    >
      {/* Top connection handle */}
      <Handle
        type="target"
        position={Position.Top}
        style={{ background: '#2d2d52', border: '1px solid #3d3d6a', width: 8, height: 8 }}
      />

      {/* Namespace label bar */}
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          height: 32,
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          padding: '0 10px',
          borderBottom: `1px dashed ${borderColor}`,
          borderRadius: '9px 9px 0 0',
          background: 'rgba(7,7,17,0.7)',
        }}
      >
        <span style={{ fontSize: 11, color: '#6366f1' }}>⎈</span>
        <span
          style={{
            fontSize: 10,
            fontFamily: 'JetBrains Mono, monospace',
            color: '#818cf8',
            fontWeight: 600,
            letterSpacing: '0.02em',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {data.label}
        </span>
        {/* health dot */}
        <div
          style={{
            width: 5,
            height: 5,
            borderRadius: '50%',
            background: healthColor,
            marginLeft: 'auto',
            flexShrink: 0,
          }}
        />
      </div>

      {/* Bottom connection handle */}
      <Handle
        type="source"
        position={Position.Bottom}
        style={{ background: '#2d2d52', border: '1px solid #3d3d6a', width: 8, height: 8 }}
      />
    </div>
  )
}
