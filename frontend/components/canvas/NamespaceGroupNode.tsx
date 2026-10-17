'use client'

import { NodeProps } from 'reactflow'
import { Handle, Position } from 'reactflow'
import { InfraNodeData } from './InfraNode'
import NodeSvgIcon from './NodeSvgIcon'

/**
 * Namespace header for the grouped canvas.
 *
 * Namespaces used to be bundled into an `Namespaces ×3` group node, so this
 * card was rarely on screen and its empty lower half went unnoticed. It is now
 * the anchor each namespace's child groups stack beneath, which makes it the
 * most looked-at node on the canvas — so it states what it contains rather
 * than rendering as a titled void.
 *
 * Fixed 280×76 for the same reason the old card was fixed: ReactFlow does not
 * apply a node's width/height data fields as inline CSS on the wrapper, so
 * percentage sizing collapses the card to nothing.
 */
const NS_HEADER_W = 280
const NS_HEADER_H = 76

export default function NamespaceGroupNode({ data, selected }: NodeProps<InfraNodeData>) {
  const healthDot = data.health === 'healthy' ? '#22c55e'
    : data.health === 'degraded' ? '#f59e0b'
    : data.health === 'unhealthy' ? '#ef4444' : '#6b7280'

  const groups = data.childSummary?.groups ?? 0
  const resources = data.childSummary?.resources ?? 0

  return (
    <div style={{
      width: NS_HEADER_W, height: NS_HEADER_H, borderRadius: 10,
      border: `1px solid ${selected ? 'var(--ink2)' : 'var(--line2)'}`,
      background: 'var(--surface)',
      position: 'relative', boxSizing: 'border-box', pointerEvents: 'all',
      boxShadow: '0 2px 12px rgba(0,0,0,0.5)',
      overflow: 'hidden',
      display: 'flex', flexDirection: 'column',
    }}>
      {/* Identity accent */}
      <div style={{ height: 2, width: '100%', background: healthDot, opacity: 0.85 }} />

      <Handle type="target" position={Position.Top}
        style={{ background: 'var(--line2)', border: '1px solid var(--surface)', width: 6, height: 6 }} />

      <div style={{
        flex: 1, display: 'flex', flexDirection: 'column', justifyContent: 'center',
        gap: 3, padding: '0 12px',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
          <NodeSvgIcon type="namespace" size={13} />
          <span style={{
            fontSize: 12.5, fontWeight: 650, color: 'var(--ink)',
            letterSpacing: '-0.01em', flex: 1, minWidth: 0,
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
          }}>{data.label}</span>
        </div>

        {/* What lives in here — the reason to look at this node at all. */}
        <div style={{
          display: 'flex', alignItems: 'center', gap: 6,
          fontSize: 10, color: 'var(--ink4)',
          fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
        }}>
          {resources === 0 ? (
            <span>empty</span>
          ) : (
            <>
              <span>{resources} resource{resources === 1 ? '' : 's'}</span>
              {/* A namespace holding a single pod renders that pod by name
                  instead of a `Pods ×1` card, so it has resources but no
                  bundled groups. Showing "· 0 groups" there is both noise and
                  wrong. */}
              {groups > 0 && (
                <>
                  <span style={{ color: 'var(--ink5)' }}>·</span>
                  <span>{groups} group{groups === 1 ? '' : 's'}</span>
                </>
              )}
            </>
          )}
        </div>
      </div>

      <Handle type="source" position={Position.Bottom}
        style={{ background: 'var(--line2)', border: '1px solid var(--surface)', width: 6, height: 6 }} />
    </div>
  )
}

export { NS_HEADER_W, NS_HEADER_H }
