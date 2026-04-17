'use client'

import { useRouter } from 'next/navigation'
import { VMState } from '@/types'
import { sendCommand } from '@/lib/wsManager'
import {
  Layers,
  Box,
  Network,
  Container,
  Server,
  RefreshCw,
  ExternalLink,
  X,
  Loader2,
  AlertTriangle,
  WifiOff,
  Cpu,
  MemoryStick,
} from 'lucide-react'

interface VMCardProps {
  vm: VMState
  onDisconnect: () => void
}

function ProgressBar({ value, color }: { value: number; color: string }) {
  const pct = Math.min(100, Math.max(0, value))
  const getColor = () => {
    if (pct > 85) return '#ef4444'
    if (pct > 65) return '#f59e0b'
    return color
  }
  return (
    <div className="progress-bar">
      <div
        className="progress-fill"
        style={{ width: `${pct}%`, background: getColor() }}
      />
    </div>
  )
}

function StatItem({
  label,
  value,
  icon,
}: {
  label: string
  value: number | undefined
  icon: React.ReactNode
}) {
  return (
    <div
      className="flex flex-col items-center gap-1 p-2 rounded-lg"
      style={{ background: '#070711' }}
    >
      <div style={{ color: '#475569' }}>{icon}</div>
      <span className="text-base font-bold" style={{ color: '#e2e8f0' }}>
        {value ?? 0}
      </span>
      <span className="text-xs" style={{ color: '#475569' }}>
        {label}
      </span>
    </div>
  )
}

export default function VMCard({ vm, onDisconnect }: VMCardProps) {
  const router = useRouter()

  const isConnected = vm.status === 'connected'
  const isConnecting = vm.status === 'connecting' || vm.status === 'paired'
  const isError = vm.status === 'error'
  const isDisconnected = vm.status === 'disconnected'

  // Extract host node for metrics
  const hostNode = vm.graph?.nodes?.find((n) => n.type === 'host')
  const clusterNode = vm.graph?.nodes?.find((n) => n.type === 'cluster')

  const cpu = parseFloat(hostNode?.metadata?.cpu ?? '0')
  const mem = parseFloat(hostNode?.metadata?.memory ?? '0')
  const cloudProvider =
    hostNode?.metadata?.cloudProvider ||
    clusterNode?.metadata?.platform ||
    hostNode?.metadata?.platform ||
    null

  const stats = vm.graph?.stats?.nodesByType ?? {}
  const snapshot = vm.graph?.snapshot

  function formatDuration(ms: number) {
    if (ms < 1000) return `${ms}ms`
    return `${(ms / 1000).toFixed(1)}s`
  }

  function formatTime(ts: string) {
    try {
      return new Date(ts).toLocaleTimeString()
    } catch {
      return ts
    }
  }

  return (
    <div
      className="rounded-xl flex flex-col transition-all duration-200"
      style={{
        background: '#0e0e1a',
        border: `1px solid ${isConnected ? '#1e1e3a' : isError ? 'rgba(239,68,68,0.2)' : '#1e1e3a'}`,
        boxShadow: isConnected ? '0 4px 24px rgba(0,0,0,0.3)' : 'none',
      }}
    >
      {/* ─── Card Header ──────────────────────────────────────── */}
      <div className="px-4 pt-4 pb-3 flex items-start justify-between gap-2">
        <div className="flex items-start gap-3 min-w-0">
          {/* Status indicator */}
          <div className="mt-0.5 flex-shrink-0">
            {isConnecting && (
              <Loader2 size={14} className="animate-spin" style={{ color: '#6366f1' }} />
            )}
            {isConnected && (
              <span
                className="block w-2.5 h-2.5 rounded-full status-dot-pulse"
                style={{ background: '#10b981' }}
              />
            )}
            {isDisconnected && (
              <span className="block w-2.5 h-2.5 rounded-full" style={{ background: '#475569' }} />
            )}
            {isError && (
              <span className="block w-2.5 h-2.5 rounded-full" style={{ background: '#ef4444' }} />
            )}
          </div>

          <div className="min-w-0">
            <h3 className="font-semibold text-sm truncate" style={{ color: '#e2e8f0' }}>
              {vm.hostname ?? vm.code}
            </h3>
            <p className="text-xs font-mono mt-0.5" style={{ color: '#475569' }}>
              {vm.code}
            </p>
          </div>
        </div>

        {/* Right badges */}
        <div className="flex items-center gap-2 flex-shrink-0">
          {cloudProvider && (
            <span
              className="text-xs px-2 py-0.5 rounded-full font-medium"
              style={{
                background: 'rgba(99,102,241,0.1)',
                color: '#818cf8',
                border: '1px solid rgba(99,102,241,0.15)',
              }}
            >
              {cloudProvider}
            </span>
          )}
          <span
            className="text-xs px-2 py-0.5 rounded-full"
            style={{
              background: isConnected
                ? 'rgba(16,185,129,0.1)'
                : isConnecting
                  ? 'rgba(99,102,241,0.1)'
                  : isError
                    ? 'rgba(239,68,68,0.1)'
                    : 'rgba(71,85,105,0.1)',
              color: isConnected
                ? '#10b981'
                : isConnecting
                  ? '#6366f1'
                  : isError
                    ? '#ef4444'
                    : '#64748b',
            }}
          >
            {isConnecting ? 'Connecting…' : vm.status}
          </span>
        </div>
      </div>

      {/* ─── Error state ──────────────────────────────────────── */}
      {isError && vm.error && (
        <div className="mx-4 mb-3 flex items-start gap-2 p-3 rounded-lg" style={{ background: 'rgba(239,68,68,0.06)', border: '1px solid rgba(239,68,68,0.15)' }}>
          <AlertTriangle size={14} style={{ color: '#ef4444', flexShrink: 0, marginTop: 1 }} />
          <p className="text-xs" style={{ color: '#fca5a5' }}>{vm.error}</p>
        </div>
      )}

      {/* ─── Disconnected state ────────────────────────────────── */}
      {isDisconnected && (
        <div className="mx-4 mb-3 flex items-center gap-2 p-3 rounded-lg" style={{ background: 'rgba(71,85,105,0.1)', border: '1px solid #1e1e3a' }}>
          <WifiOff size={14} style={{ color: '#64748b' }} />
          <p className="text-xs" style={{ color: '#64748b' }}>Agent disconnected</p>
        </div>
      )}

      {/* ─── Metrics (only when connected + has graph) ─────────── */}
      {isConnected && vm.graph && (
        <>
          {/* CPU + Memory */}
          {(cpu > 0 || mem > 0) && (
            <div className="px-4 pb-3 flex flex-col gap-2">
              <div className="flex items-center justify-between gap-4">
                <div className="flex-1">
                  <div className="flex items-center justify-between mb-1">
                    <span className="flex items-center gap-1 text-xs" style={{ color: '#475569' }}>
                      <Cpu size={11} />
                      CPU
                    </span>
                    <span className="text-xs font-medium" style={{ color: '#94a3b8' }}>
                      {cpu.toFixed(1)}%
                    </span>
                  </div>
                  <ProgressBar value={cpu} color="#6366f1" />
                </div>
                <div className="flex-1">
                  <div className="flex items-center justify-between mb-1">
                    <span className="flex items-center gap-1 text-xs" style={{ color: '#475569' }}>
                      <MemoryStick size={11} />
                      Mem
                    </span>
                    <span className="text-xs font-medium" style={{ color: '#94a3b8' }}>
                      {mem.toFixed(1)}%
                    </span>
                  </div>
                  <ProgressBar value={mem} color="#8b5cf6" />
                </div>
              </div>
            </div>
          )}

          {/* Stats grid */}
          <div className="px-4 pb-3 grid grid-cols-3 gap-2">
            <StatItem label="Pods" value={stats.pod} icon={<span className="text-sm">◉</span>} />
            <StatItem
              label="Deploys"
              value={stats.deployment}
              icon={<Layers size={14} />}
            />
            <StatItem
              label="Services"
              value={stats.k8s_service}
              icon={<Network size={14} />}
            />
            <StatItem
              label="Containers"
              value={stats.container}
              icon={<Box size={14} />}
            />
            <StatItem
              label="Namespaces"
              value={stats.namespace}
              icon={<span className="text-sm">⬡</span>}
            />
            <StatItem
              label="Nodes"
              value={stats.node}
              icon={<Server size={14} />}
            />
          </div>

          {/* Snapshot meta */}
          {snapshot && (
            <div
              className="mx-4 mb-3 px-3 py-2 rounded-lg flex items-center justify-between"
              style={{ background: '#070711' }}
            >
              <span className="text-xs" style={{ color: '#475569' }}>
                {vm.graph.stats.totalNodes} nodes · {vm.graph.stats.totalEdges} edges
              </span>
              <span className="text-xs" style={{ color: '#475569' }}>
                {formatTime(snapshot.timestamp)}
              </span>
            </div>
          )}
        </>
      )}

      {/* ─── Loading state ─────────────────────────────────────── */}
      {(isConnecting || (isConnected && !vm.graph)) && (
        <div className="px-4 pb-4 flex items-center gap-2">
          <div
            className="flex-1 h-1.5 rounded overflow-hidden"
            style={{ background: '#13131f' }}
          >
            <div
              className="h-full rounded"
              style={{
                background: 'linear-gradient(90deg, #6366f1, #8b5cf6)',
                width: '40%',
                animation: 'slide-progress 1.5s ease-in-out infinite',
              }}
            />
          </div>
          <span className="text-xs" style={{ color: '#475569' }}>
            {isConnecting ? 'Pairing…' : 'Loading graph…'}
          </span>
        </div>
      )}

      {/* ─── Actions ──────────────────────────────────────────── */}
      <div
        className="px-4 pb-4 pt-2 flex gap-2 border-t"
        style={{ borderColor: '#13131f' }}
      >
        <button
          onClick={() => router.push(`/vm/${vm.code}`)}
          disabled={!isConnected || !vm.graph}
          className="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg text-xs font-semibold transition-all"
          style={{
            background:
              isConnected && vm.graph
                ? 'linear-gradient(135deg, #6366f1, #8b5cf6)'
                : '#13131f',
            color: isConnected && vm.graph ? '#fff' : '#475569',
            cursor: isConnected && vm.graph ? 'pointer' : 'not-allowed',
          }}
          onMouseEnter={(e) => {
            if (isConnected && vm.graph) e.currentTarget.style.opacity = '0.85'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.opacity = '1'
          }}
        >
          <ExternalLink size={13} />
          View Canvas
        </button>
        <button
          onClick={() => sendCommand(vm.code, 'refresh')}
          disabled={!isConnected}
          className="flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium transition-all"
          style={{
            background: '#13131f',
            border: '1px solid #1e1e3a',
            color: isConnected ? '#94a3b8' : '#475569',
            cursor: isConnected ? 'pointer' : 'not-allowed',
          }}
          onMouseEnter={(e) => {
            if (isConnected) {
              e.currentTarget.style.borderColor = '#2d2d52'
              e.currentTarget.style.color = '#e2e8f0'
            }
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.borderColor = '#1e1e3a'
            e.currentTarget.style.color = isConnected ? '#94a3b8' : '#475569'
          }}
          title="Refresh graph"
        >
          <RefreshCw size={13} />
        </button>
        <button
          onClick={onDisconnect}
          className="flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium transition-all"
          style={{
            background: '#13131f',
            border: '1px solid #1e1e3a',
            color: '#475569',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.borderColor = 'rgba(239,68,68,0.3)'
            e.currentTarget.style.color = '#ef4444'
            e.currentTarget.style.background = 'rgba(239,68,68,0.06)'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.borderColor = '#1e1e3a'
            e.currentTarget.style.color = '#475569'
            e.currentTarget.style.background = '#13131f'
          }}
          title="Disconnect"
        >
          <X size={13} />
        </button>
      </div>
    </div>
  )
}
