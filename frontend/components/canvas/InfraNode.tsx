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
  healthy:'#22c55e', degraded:'#f59e0b', unhealthy:'#ef4444', unknown:'var(--line3)',
}

const MONO = 'var(--font-geist-mono,"Geist Mono","JetBrains Mono",ui-monospace,monospace)'

function meterColor(pct: number): string {
  if (pct > 80) return '#ef4444'
  if (pct > 50) return '#f59e0b'
  return '#22c55e'
}

interface Meters { cpu: number | null; mem: number | null }

function getMeters(nodeType: string, metadata: Record<string, any>): Meters | null {
  if (nodeType === 'host') {
    const cpu = typeof metadata.cpu_percent === 'number' ? metadata.cpu_percent : null
    const mem = typeof metadata.memory_percent === 'number' ? metadata.memory_percent : null
    if (cpu === null && mem === null) return null
    return { cpu, mem }
  }
  if (nodeType === 'container') {
    // Only show for running containers — stopped containers have no live stats
    if (metadata.state !== 'running') return null
    const cpu = typeof metadata.cpu === 'number' ? metadata.cpu : null
    const memUsage = typeof metadata.memory === 'number' ? metadata.memory : 0
    const memLimit = typeof metadata.memoryLimit === 'number' ? metadata.memoryLimit : 0
    const mem = memLimit > 0 ? (memUsage / memLimit) * 100 : null
    if (cpu === null && mem === null) return null
    return { cpu, mem }
  }
  return null
}

function MiniBar({ label, value }: { label: string; value: number }) {
  const clamped = Math.min(100, Math.max(0, value))
  const color = meterColor(clamped)
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 5 }}>
      <span style={{
        fontSize: 9, color: 'var(--ink4)', width: 22, flexShrink: 0,
        fontFamily: MONO, letterSpacing: '0.04em',
      }}>{label}</span>
      <div style={{ flex: 1, height: 3, background: 'var(--line)', borderRadius: 2, overflow: 'hidden' }}>
        <div style={{
          width: `${clamped}%`, height: '100%', background: color, borderRadius: 2,
          transition: 'width 0.4s ease',
        }} />
      </div>
      <span style={{
        fontSize: 9, color: 'var(--ink3)', width: 30, textAlign: 'right', flexShrink: 0,
        fontFamily: MONO,
      }}>{clamped.toFixed(1)}%</span>
    </div>
  )
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
  const meters = getMeters(nodeType, metadata)
  const dotColor = HEALTH_DOT[health] ?? 'var(--ink4)'
  const barColor = HEALTH_BAR[health] ?? 'var(--line3)'
  const isUnhealthy = health === 'unhealthy' || health === 'degraded'
  const hasContent = keyMeta.length > 0 || meters !== null

  return (
    <div style={{
      width: 220,
      background: 'var(--surface)',
      border: `1px solid ${selected ? 'var(--ink)' : isUnhealthy ? 'var(--line3)' : 'var(--line)'}`,
      borderRadius: 9,
      boxShadow: selected ? '0 0 0 1px var(--ink)' : '0 2px 8px rgba(0,0,0,0.12)',
      transition: 'border-color 0.12s, box-shadow 0.12s',
      position: 'relative',
      overflow: 'hidden',
    }}>
      {/* health bar top */}
      <div style={{ height: 2, background: barColor, width: '100%' }} />

      <div style={{ padding: '9px 10px 9px 12px' }}>
        {/* Row 1: icon + label + dot */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 5 }}>
          <div style={{ width: 24, height: 24, borderRadius: 6, background: 'var(--surface-2)', border: '1px solid var(--line2)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
            <NodeSvgIcon type={nodeType} size={14} />
          </div>
          <span style={{
            fontSize: 12, fontWeight: 500, color: 'var(--ink)', flex: 1,
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
            letterSpacing: '-0.01em',
          }} title={label}>
            {label.length > 22 ? label.slice(0, 20) + '…' : label}
          </span>
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: dotColor, flexShrink: 0 }} title={health} />
        </div>

        {/* Row 2: type badge */}
        <div style={{ marginBottom: hasContent ? 5 : 0 }}>
          <span style={{
            fontSize: 9.5, padding: '1px 6px', borderRadius: 4,
            background: 'var(--line)', color: 'var(--ink3)', border: '1px solid var(--line2)',
            fontFamily: MONO,
            letterSpacing: '0.04em',
          }}>{nodeType}</span>
        </div>

        {/* Row 3: key meta */}
        {keyMeta.length > 0 && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 2, marginBottom: meters ? 6 : 0 }}>
            {keyMeta.map((line, i) => (
              <span key={i} style={{
                fontSize: 10, color: 'var(--ink3)',
                fontFamily: MONO,
                overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }} title={line}>{line}</span>
            ))}
          </div>
        )}

        {/* Row 4: resource meters — host always, containers only when running */}
        {meters && (
          <div style={{
            display: 'flex', flexDirection: 'column', gap: 3,
            borderTop: keyMeta.length > 0 ? '1px solid var(--line)' : 'none',
            paddingTop: keyMeta.length > 0 ? 6 : 0,
          }}>
            {meters.cpu !== null && <MiniBar label="CPU" value={meters.cpu} />}
            {meters.mem !== null && <MiniBar label="MEM" value={meters.mem} />}
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
