'use client'

import { memo } from 'react'
import { Handle, Position, NodeProps } from 'reactflow'
import { MoreVertical } from 'lucide-react'
import { NodeHealth } from '@/types'
import NodeSvgIcon from './NodeSvgIcon'

export interface InfraNodeData {
  nodeType: string
  label: string
  health: NodeHealth
  metadata: Record<string, any>
  selected?: boolean
  /** Present only on nodes that have a "•••" drill-down menu (currently: host). */
  onOpenPicker?: (nodeId: string, e: React.MouseEvent) => void
  /** Namespace headers only: what this namespace is parent of. */
  childSummary?: { groups: number; resources: number }
}

const HEALTH_DOT: Record<NodeHealth, string> = {
  healthy:'#22c55e', degraded:'#f59e0b', unhealthy:'#ef4444', unknown:'#6b7280',
}
const HEALTH_BAR: Record<NodeHealth, string> = {
  healthy:'#22c55e', degraded:'#f59e0b', unhealthy:'#ef4444', unknown:'var(--line3)',
}


function getKeyMeta(nodeType: string, metadata: Record<string, any>): string[] {
  const lines: string[] = []
  if (metadata.app) lines.push(`app: ${metadata.app}`)
  switch (nodeType) {
    case 'pod':
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      if (metadata.phase) lines.push(metadata.phase)
      if (metadata.restartCount > 0) lines.push(`↻ ${metadata.restartCount}`)
      break
    case 'deployment':
    case 'statefulset':
    case 'daemonset':
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      if (metadata.replicas !== undefined) lines.push(`${metadata.readyReplicas ?? '?'}/${metadata.replicas} ready`)
      break
    case 'k8s_service':
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      if (metadata.type) lines.push(metadata.type)
      break
    case 'namespace':
      if (metadata.status) lines.push(metadata.status)
      break
    case 'container':
      if (metadata.image) lines.push(metadata.image.split('/').pop()?.split(':')[0] ?? metadata.image)
      if (metadata.status) lines.push(metadata.status)
      break
    case 'node': {
      const nIP = metadata.internalIP ?? metadata.ip
      if (nIP) lines.push(nIP)
      if (metadata.osImage) lines.push(metadata.osImage)
      break
    }
    case 'host': {
      const hIP = metadata.ip ?? metadata.ip_address ?? metadata.ipAddress
      if (hIP && hIP !== '127.0.0.1' && !String(hIP).startsWith('127.')) lines.push(hIP)
      if (metadata.os) lines.push(metadata.os)
      break
    }
    case 'cluster':
      if (metadata.version) lines.push(`v${metadata.version}`)
      break
    case 'ingress':
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      if (metadata.host) lines.push(metadata.host)
      break
    case 'pvc':
      if (metadata.capacity) lines.push(metadata.capacity)
      break
    case 'cronjob':
    case 'job':
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      if (metadata.schedule) lines.push(metadata.schedule)
      break
    default:
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
  }
  return lines.slice(0, 2)
}

const InfraNode = memo(({ id, data, selected }: NodeProps<InfraNodeData>) => {
  const { nodeType, label, health, metadata, onOpenPicker } = data
  const keyMeta = getKeyMeta(nodeType, metadata)
  // The runtime node itself should say which engine it actually is — Docker's
  // whale icon would be wrong for a Podman-backed host.
  const iconType = nodeType === 'container_runtime' && metadata?.runtime_type === 'podman' ? 'podman' : nodeType
  const dotColor = HEALTH_DOT[health] ?? 'var(--ink4)'
  const barColor = HEALTH_BAR[health] ?? 'var(--line3)'
  const isUnhealthy = health === 'unhealthy' || health === 'degraded'

  return (
    <div style={{
      width: 220,
      background: 'var(--surface)',
      border: `1px solid ${selected ? 'var(--ink)' : isUnhealthy ? 'var(--line3)' : 'var(--line2)'}`,
      borderRadius: 9,
      boxShadow: selected ? '0 0 0 1px var(--ink)' : '0 2px 8px rgba(0,0,0,0.5)',
      transition: 'border-color 0.12s, box-shadow 0.12s',
      position: 'relative',
      overflow: 'hidden',
    }}>
      {/* health bar top */}
      <div style={{ height: 2, background: barColor, width: '100%' }} />

      <div style={{ padding: '9px 10px 9px 12px' }}>
        {/* Row 1: icon + label + dot */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 5 }}>
          <div style={{ width: 24, height: 24, borderRadius: 6, background: 'var(--surface-icon)', border: '1px solid var(--line2)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
            <NodeSvgIcon type={iconType} size={14} />
          </div>
          <span style={{
            fontSize: 12, fontWeight: 500, color: 'var(--ink)', flex: 1,
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            letterSpacing: '-0.01em',
          }} title={label}>
            {label.length > 22 ? label.slice(0, 20) + '…' : label}
          </span>
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: dotColor, flexShrink: 0 }} title={health} />
          {onOpenPicker && (
            <button
              onClick={(e) => { e.stopPropagation(); onOpenPicker(id, e) }}
              title="Show more"
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

        {/* Row 2: type badge + app badge */}
        <div style={{ display: 'flex', gap: 5, marginBottom: keyMeta.length ? 5 : 0, flexWrap: 'wrap' }}>
          <span style={{
            fontSize: 9.5, padding: '1px 6px', borderRadius: 4,
            background: 'var(--line)', color: 'var(--ink3)', border: '1px solid var(--line2)',
            fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
            letterSpacing: '0.04em',
          }}>{nodeType}</span>
          {metadata.app && (
            <span style={{
              fontSize: 9.5, padding: '1px 6px', borderRadius: 4,
              background: `hsl(${Math.abs(metadata.app.charCodeAt(0) * 7) % 360}, 45%, 20%)`,
              color: `hsl(${Math.abs(metadata.app.charCodeAt(0) * 7) % 360}, 70%, 60%)`,
              border: `1px solid hsl(${Math.abs(metadata.app.charCodeAt(0) * 7) % 360}, 60%, 40%)`,
              fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
              letterSpacing: '0.04em',
              fontWeight: 500,
            }} title={`app: ${metadata.app}`}>
              {metadata.app.length > 10 ? metadata.app.slice(0, 8) + '…' : metadata.app}
            </span>
          )}
        </div>

        {/* Row 3: key meta */}
        {keyMeta.length > 0 && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            {keyMeta.map((line, i) => (
              <span key={i} style={{
                fontSize: 10, color: 'var(--ink3)',
                fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
                overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }} title={line}>{line}</span>
            ))}
          </div>
        )}
      </div>

      <Handle type="target" position={Position.Top}    style={{ background: 'var(--line2)', width: 5, height: 5, border: '1px solid var(--surface)' }} />
      <Handle type="source" position={Position.Bottom} style={{ background: 'var(--line2)', width: 5, height: 5, border: '1px solid var(--surface)' }} />
    </div>
  )
})

InfraNode.displayName = 'InfraNode'
export default InfraNode
