'use client'

import { NodeProps } from 'reactflow'
import { Handle, Position } from 'reactflow'
import { InfraNodeData } from './InfraNode'
import NodeSvgIcon from './NodeSvgIcon'

/**
 * Namespace group node — a standalone card, same footprint as InfraNode.
 * Every caller constructs it with a fixed 220×100 (see buildGroupedGraph /
 * buildFlatFlowElements), so it sizes itself explicitly rather than via
 * width/height:'100%' — ReactFlow doesn't apply the node's width/height
 * data fields as inline CSS on the wrapper, so percentage sizing here
 * collapsed the whole card to a few pixels (nothing but Handles and
 * absolutely-positioned children contribute to intrinsic size).
 */
export default function NamespaceGroupNode({ data, selected }: NodeProps<InfraNodeData>) {
  const healthDot = data.health === 'healthy' ? 'var(--ink)' : data.health === 'degraded' ? 'var(--ink3)' : 'var(--line3)'

  return (
    <div style={{
      width: 220, height: 100, borderRadius: 10,
      border: `1px dashed ${selected ? 'var(--ink2)' : 'var(--line2)'}`,
      background: 'var(--surface-2)',
      position: 'relative', boxSizing: 'border-box', pointerEvents: 'all',
    }}>
      <Handle type="target" position={Position.Top} style={{ background: 'var(--line2)', border: '1px solid var(--line)', width: 7, height: 7 }} />

      <div style={{
        position: 'absolute', top: 0, left: 0, right: 0, height: 30,
        display: 'flex', alignItems: 'center', gap: 6, padding: '0 10px',
        borderBottom: `1px dashed ${selected ? 'var(--line2)' : 'var(--line)'}`,
        borderRadius: '9px 9px 0 0',
        background: 'var(--surface)',
      }}>
        <NodeSvgIcon type="namespace" size={12} />
        <span style={{
          fontSize: 10,
          fontFamily: 'var(--font-geist-mono,"Geist Mono","JetBrains Mono",ui-monospace,monospace)',
          color: 'var(--ink2)', fontWeight: 600, letterSpacing: '0.02em',
          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
        }}>{data.label}</span>
        <div style={{ width: 5, height: 5, borderRadius: '50%', background: healthDot, marginLeft: 'auto', flexShrink: 0 }} />
      </div>

      <Handle type="source" position={Position.Bottom} style={{ background: 'var(--line2)', border: '1px solid var(--line)', width: 7, height: 7 }} />
    </div>
  )
}
