'use client'

import { memo } from 'react'
import { Handle, Position, NodeProps } from 'reactflow'
import { NodeHealth } from '@/types'
import NodeSvgIcon from './NodeSvgIcon'

export interface InfraNodeData {
  nodeType: string
  label: string
  health: NodeHealth
  metadata: Record<string, any>
  selected?: boolean
}

const HEALTH_DOT: Record<NodeHealth, string> = {
  healthy:'#22c55e', degraded:'#f59e0b', unhealthy:'#ef4444', unknown:'#6b7280',
}
const HEALTH_BAR: Record<NodeHealth, string> = {
  healthy:'#22c55e', degraded:'#f59e0b', unhealthy:'#ef4444', unknown:'#383838',
}

function getKeyMeta(nodeType: string, metadata: Record<string, any>): string[] {
  const lines: string[] = []
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

const InfraNode = memo(({ data, selected }: NodeProps<InfraNodeData>) => {
  const { nodeType, label, health, metadata } = data
  const keyMeta = getKeyMeta(nodeType, metadata)
  const dotColor = HEALTH_DOT[health] ?? '#454545'
  const barColor = HEALTH_BAR[health] ?? '#383838'
  const isUnhealthy = health === 'unhealthy' || health === 'degraded'

  return (
    <div style={{
      width: 220,
      background: '#111111',
      border: `1px solid ${selected ? '#FAFAFA' : isUnhealthy ? '#383838' : '#1E1E1E'}`,
      borderRadius: 9,
      boxShadow: selected ? '0 0 0 1px #FAFAFA' : '0 2px 8px rgba(0,0,0,0.5)',
      transition: 'border-color 0.12s, box-shadow 0.12s',
      position: 'relative',
      overflow: 'hidden',
    }}>
      {/* health bar top */}
      <div style={{ height: 2, background: barColor, width: '100%' }} />

      <div style={{ padding: '9px 10px 9px 12px' }}>
        {/* Row 1: icon + label + dot */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 5 }}>
          <div style={{ width: 24, height: 24, borderRadius: 6, background: '#1A1A1A', border: '1px solid #2A2A2A', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
            <NodeSvgIcon type={nodeType} size={14} />
          </div>
          <span style={{
            fontSize: 12, fontWeight: 500, color: '#FAFAFA', flex: 1,
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            letterSpacing: '-0.01em',
          }} title={label}>
            {label.length > 22 ? label.slice(0, 20) + '…' : label}
          </span>
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: dotColor, flexShrink: 0 }} title={health} />
        </div>

        {/* Row 2: type badge */}
        <div style={{ marginBottom: keyMeta.length ? 5 : 0 }}>
          <span style={{
            fontSize: 9.5, padding: '1px 6px', borderRadius: 4,
            background: '#1E1E1E', color: '#6E6E6E', border: '1px solid #2A2A2A',
            fontFamily: 'var(--font-geist-mono,"Geist Mono","JetBrains Mono",ui-monospace,monospace)',
            letterSpacing: '0.04em',
          }}>{nodeType}</span>
        </div>

        {/* Row 3: key meta */}
        {keyMeta.length > 0 && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            {keyMeta.map((line, i) => (
              <span key={i} style={{
                fontSize: 10, color: '#6E6E6E',
                fontFamily: 'var(--font-geist-mono,"Geist Mono","JetBrains Mono",ui-monospace,monospace)',
                overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }} title={line}>{line}</span>
            ))}
          </div>
        )}
      </div>

      <Handle type="target" position={Position.Top}    style={{ background: '#2A2A2A', width: 5, height: 5, border: '1px solid #111111' }} />
      <Handle type="source" position={Position.Bottom} style={{ background: '#2A2A2A', width: 5, height: 5, border: '1px solid #111111' }} />
    </div>
  )
})

InfraNode.displayName = 'InfraNode'
export default InfraNode
