'use client'

import { memo } from 'react'
import { Handle, Position, NodeProps } from 'reactflow'
import { getNodeColor, getHealthColor, getNodeIcon, NodeHealth } from '@/types'

export interface InfraNodeData {
  nodeType: string
  label: string
  health: NodeHealth
  metadata: Record<string, any>
  selected?: boolean
}

function getKeyMetadata(nodeType: string, metadata: Record<string, any>): string[] {
  const lines: string[] = []

  switch (nodeType) {
    case 'pod':
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      if (metadata.phase) lines.push(metadata.phase)
      if (metadata.restartCount) lines.push(`restarts: ${metadata.restartCount}`)
      break
    case 'deployment':
    case 'statefulset':
    case 'daemonset':
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      if (metadata.replicas !== undefined) lines.push(`replicas: ${metadata.replicas}`)
      if (metadata.readyReplicas !== undefined) lines.push(`ready: ${metadata.readyReplicas}`)
      break
    case 'k8s_service':
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      if (metadata.type) lines.push(metadata.type)
      if (metadata.clusterIP) lines.push(metadata.clusterIP)
      break
    case 'namespace':
      if (metadata.status) lines.push(metadata.status)
      break
    case 'container':
      if (metadata.image) lines.push(metadata.image.split('/').pop()?.split(':')[0] ?? metadata.image)
      if (metadata.status) lines.push(metadata.status)
      break
    case 'node':
      if (metadata.osImage) lines.push(metadata.osImage)
      if (metadata.cpu) lines.push(`cpu: ${metadata.cpu}`)
      if (metadata.memory) lines.push(`mem: ${metadata.memory}`)
      break
    case 'host':
      if (metadata.os) lines.push(metadata.os)
      if (metadata.cpu) lines.push(`cpu: ${metadata.cpu}`)
      if (metadata.memory) lines.push(`mem: ${metadata.memory}`)
      break
    case 'ingress':
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      if (metadata.host) lines.push(metadata.host)
      break
    case 'pvc':
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      if (metadata.capacity) lines.push(metadata.capacity)
      if (metadata.accessModes) lines.push(metadata.accessModes)
      break
    case 'cronjob':
    case 'job':
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      if (metadata.schedule) lines.push(metadata.schedule)
      break
    case 'cluster':
      if (metadata.platform) lines.push(metadata.platform)
      if (metadata.version) lines.push(`v${metadata.version}`)
      break
    case 'image':
      if (metadata.size) lines.push(metadata.size)
      break
    default:
      if (metadata.namespace) lines.push(`ns: ${metadata.namespace}`)
      break
  }

  return lines.slice(0, 2)
}

function HealthDot({ health }: { health: NodeHealth }) {
  const color = getHealthColor(health)
  return (
    <span
      className="inline-block w-1.5 h-1.5 rounded-full flex-shrink-0"
      style={{ background: color }}
      title={health}
    />
  )
}

const InfraNode = memo(({ data, selected }: NodeProps<InfraNodeData>) => {
  const { nodeType, label, health, metadata } = data
  const color = getNodeColor(nodeType, health)
  const icon = getNodeIcon(nodeType)
  const keyMeta = getKeyMetadata(nodeType, metadata)

  const displayLabel = label.length > 28 ? label.slice(0, 26) + '…' : label

  return (
    <div
      className="relative rounded-lg overflow-hidden"
      style={{
        width: 220,
        background: '#0e0e1a',
        border: `1px solid ${selected ? color : '#1e1e3a'}`,
        boxShadow: selected ? `0 0 0 2px ${color}33, 0 4px 16px rgba(0,0,0,0.5)` : '0 2px 12px rgba(0,0,0,0.4)',
        transition: 'border-color 0.15s, box-shadow 0.15s',
      }}
    >
      {/* Left color accent bar */}
      <div
        className="absolute left-0 top-0 bottom-0 w-0.5"
        style={{ background: color }}
      />

      {/* Content */}
      <div className="pl-3 pr-2.5 py-2.5">
        {/* Top row: icon + label + health */}
        <div className="flex items-center gap-2 mb-1">
          <span
            className="text-sm flex-shrink-0 leading-none"
            style={{ color, fontFamily: 'system-ui' }}
          >
            {icon}
          </span>
          <span
            className="text-xs font-semibold flex-1 min-w-0 truncate leading-tight"
            style={{ color: '#e2e8f0' }}
            title={label}
          >
            {displayLabel}
          </span>
          <HealthDot health={health} />
        </div>

        {/* Type badge */}
        <div className="flex items-center gap-1.5 mb-1.5">
          <span
            className="text-xs px-1.5 py-0.5 rounded font-medium leading-none"
            style={{
              background: `${color}18`,
              color: color,
              border: `1px solid ${color}28`,
              fontFamily: 'JetBrains Mono, monospace',
            }}
          >
            {nodeType}
          </span>
        </div>

        {/* Key metadata */}
        {keyMeta.length > 0 && (
          <div className="flex flex-col gap-0.5">
            {keyMeta.map((line, i) => (
              <p
                key={i}
                className="text-xs truncate leading-snug"
                style={{
                  color: '#475569',
                  fontFamily: 'JetBrains Mono, monospace',
                  fontSize: '10px',
                }}
                title={line}
              >
                {line}
              </p>
            ))}
          </div>
        )}
      </div>

      {/* React Flow handles */}
      <Handle
        type="target"
        position={Position.Top}
        style={{
          background: color,
          width: 6,
          height: 6,
          border: '1px solid #070711',
        }}
      />
      <Handle
        type="source"
        position={Position.Bottom}
        style={{
          background: color,
          width: 6,
          height: 6,
          border: '1px solid #070711',
        }}
      />
    </div>
  )
})

InfraNode.displayName = 'InfraNode'

export default InfraNode
