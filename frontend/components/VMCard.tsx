'use client'

import { useRouter } from 'next/navigation'
import { VMState } from '@/types'
import { sendCommand } from '@/lib/wsManager'
import {
  Layers, Network, Server, RefreshCw, ExternalLink, X,
  Loader2, AlertTriangle, WifiOff, Cpu, MemoryStick, Box,
} from 'lucide-react'

interface VMCardProps {
  vm: VMState
  onDisconnect: () => void
}

function MiniBar({ value, color }: { value: number; color: string }) {
  const pct = Math.min(100, Math.max(0, value))
  const bar = pct > 85 ? '#D95555' : pct > 65 ? '#C8993C' : color
  return (
    <div style={{ height: 3, borderRadius: 2, background: 'rgba(255,255,255,0.07)', overflow: 'hidden' }}>
      <div style={{ height: '100%', width: `${pct}%`, background: bar, borderRadius: 2, transition: 'width 0.6s ease' }} />
    </div>
  )
}

function Stat({ label, value, icon }: { label: string; value: number | undefined; icon: React.ReactNode }) {
  return (
    <div style={{
      display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 3,
      padding: '8px 4px', borderRadius: 9, background: 'rgba(255,255,255,0.03)',
    }}>
      <span style={{ color: '#625850', display: 'flex' }}>{icon}</span>
      <span style={{ fontSize: 16, fontWeight: 600, color: '#F0EDE7', lineHeight: 1 }}>{value ?? 0}</span>
      <span style={{ fontSize: 10, color: '#625850', lineHeight: 1 }}>{label}</span>
    </div>
  )
}

export default function VMCard({ vm, onDisconnect }: VMCardProps) {
  const router = useRouter()

  const isConnected   = vm.status === 'connected'
  const isConnecting  = vm.status === 'connecting' || vm.status === 'paired'
  const isError       = vm.status === 'error'
  const isDisconnected = vm.status === 'disconnected'

  const hostNode    = vm.graph?.nodes?.find((n) => n.type === 'host')
  const clusterNode = vm.graph?.nodes?.find((n) => n.type === 'cluster')

  const cpu           = parseFloat(hostNode?.metadata?.cpu    ?? '0')
  const mem           = parseFloat(hostNode?.metadata?.memory ?? '0')
  const cloudProvider = hostNode?.metadata?.cloudProvider || clusterNode?.metadata?.platform || null
  const stats         = vm.graph?.stats?.nodesByType ?? {}
  const snapshot      = vm.graph?.snapshot

  const canOpen = isConnected && !!vm.graph

  /* status color */
  const dotColor = isConnected ? '#4DB88A' : isConnecting ? '#DA7756' : isError ? '#D95555' : '#625850'
  const statusLabel = isConnecting ? 'Connecting…' : isError ? 'Error' : isDisconnected ? 'Disconnected' : vm.status

  return (
    <div style={{
      background: '#191817',
      border: `1px solid ${isError ? 'rgba(217,85,85,0.2)' : 'rgba(255,255,255,0.08)'}`,
      borderRadius: 14,
      display: 'flex', flexDirection: 'column',
      transition: 'border-color 0.2s, box-shadow 0.2s',
      overflow: 'hidden',
    }}
    onMouseEnter={(e) => {
      if (!isError) (e.currentTarget as HTMLDivElement).style.borderColor = 'rgba(255,255,255,0.14)'
      ;(e.currentTarget as HTMLDivElement).style.boxShadow = '0 8px 32px rgba(0,0,0,0.4)'
    }}
    onMouseLeave={(e) => {
      ;(e.currentTarget as HTMLDivElement).style.borderColor = isError ? 'rgba(217,85,85,0.2)' : 'rgba(255,255,255,0.08)'
      ;(e.currentTarget as HTMLDivElement).style.boxShadow = 'none'
    }}
    >

      {/* ── Header ────────────────────────────────────────────────── */}
      <div style={{ padding: '16px 18px 12px', display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 10 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
          {/* status dot / spinner */}
          {isConnecting
            ? <Loader2 size={14} style={{ color: '#DA7756', flexShrink: 0 }} className="animate-spin" />
            : <span style={{ width: 8, height: 8, borderRadius: '50%', background: dotColor, flexShrink: 0, display: 'block' }}
                    className={isConnected ? 'status-dot-pulse' : ''} />
          }
          <div style={{ minWidth: 0 }}>
            <p style={{ fontSize: 14, fontWeight: 600, color: '#F0EDE7', margin: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {vm.hostname ?? vm.code}
            </p>
            <p style={{ fontSize: 11, color: '#625850', margin: '2px 0 0', fontFamily: 'JetBrains Mono, monospace' }}>
              {vm.code}
            </p>
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 7, flexShrink: 0 }}>
          {cloudProvider && (
            <span style={{
              fontSize: 11, padding: '2px 9px', borderRadius: 20,
              background: 'rgba(218,119,86,0.1)', color: '#DA7756',
              border: '1px solid rgba(218,119,86,0.18)', fontWeight: 500,
            }}>{cloudProvider}</span>
          )}
          <span style={{
            fontSize: 11, padding: '2px 9px', borderRadius: 20, fontWeight: 500,
            background: isConnected ? 'rgba(77,184,138,0.1)' : isConnecting ? 'rgba(218,119,86,0.1)' : isError ? 'rgba(217,85,85,0.1)' : 'rgba(255,255,255,0.06)',
            color: isConnected ? '#4DB88A' : isConnecting ? '#DA7756' : isError ? '#D95555' : '#625850',
          }}>{statusLabel}</span>
        </div>
      </div>

      {/* ── Error banner ──────────────────────────────────────────── */}
      {isError && vm.error && (
        <div style={{
          margin: '0 14px 10px',
          padding: '10px 12px', borderRadius: 9,
          background: 'rgba(217,85,85,0.07)', border: '1px solid rgba(217,85,85,0.15)',
          display: 'flex', alignItems: 'flex-start', gap: 9,
        }}>
          <AlertTriangle size={13} style={{ color: '#D95555', flexShrink: 0, marginTop: 1 }} />
          <p style={{ fontSize: 12, color: '#F8A0A0', margin: 0, lineHeight: 1.5 }}>{vm.error}</p>
        </div>
      )}

      {/* ── Disconnected ──────────────────────────────────────────── */}
      {isDisconnected && (
        <div style={{
          margin: '0 14px 10px', padding: '10px 12px', borderRadius: 9,
          background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.07)',
          display: 'flex', alignItems: 'center', gap: 9,
        }}>
          <WifiOff size={13} style={{ color: '#625850' }} />
          <p style={{ fontSize: 12, color: '#625850', margin: 0 }}>Agent disconnected</p>
        </div>
      )}

      {/* ── Metrics ───────────────────────────────────────────────── */}
      {isConnected && vm.graph && (
        <>
          {(cpu > 0 || mem > 0) && (
            <div style={{ padding: '0 18px 12px', display: 'flex', flexDirection: 'column', gap: 7 }}>
              <div style={{ display: 'flex', gap: 14 }}>
                {[
                  { label: 'CPU', value: cpu, color: '#DA7756', icon: <Cpu size={11} /> },
                  { label: 'Mem', value: mem, color: '#8B6CF6', icon: <MemoryStick size={11} /> },
                ].map((m) => (
                  <div key={m.label} style={{ flex: 1 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 5 }}>
                      <span style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 11, color: '#625850' }}>
                        {m.icon} {m.label}
                      </span>
                      <span style={{ fontSize: 11, fontWeight: 500, color: '#A09890', fontFamily: 'JetBrains Mono, monospace' }}>
                        {m.value.toFixed(1)}%
                      </span>
                    </div>
                    <MiniBar value={m.value} color={m.color} />
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Stats */}
          <div style={{ padding: '0 14px 12px', display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 6 }}>
            <Stat label="Pods"       value={stats.pod}         icon={<span style={{ fontSize: 13 }}>◉</span>} />
            <Stat label="Deploys"    value={stats.deployment}  icon={<Layers size={13} />} />
            <Stat label="Services"   value={stats.k8s_service} icon={<Network size={13} />} />
            <Stat label="Containers" value={stats.container}   icon={<Box size={13} />} />
            <Stat label="Namespaces" value={stats.namespace}   icon={<span style={{ fontSize: 13 }}>⬡</span>} />
            <Stat label="Nodes"      value={stats.node}        icon={<Server size={13} />} />
          </div>

          {/* Snapshot meta */}
          {snapshot && (
            <div style={{
              margin: '0 14px 12px', padding: '8px 12px', borderRadius: 8,
              background: 'rgba(255,255,255,0.03)',
              display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            }}>
              <span style={{ fontSize: 11, color: '#625850' }}>
                {vm.graph.stats.totalNodes} nodes · {vm.graph.stats.totalEdges} edges
              </span>
              <span style={{ fontSize: 11, color: '#625850', fontFamily: 'JetBrains Mono, monospace' }}>
                {new Date(snapshot.timestamp).toLocaleTimeString()}
              </span>
            </div>
          )}
        </>
      )}

      {/* ── Loading shimmer ────────────────────────────────────────── */}
      {(isConnecting || (isConnected && !vm.graph)) && (
        <div style={{ padding: '0 18px 14px', display: 'flex', alignItems: 'center', gap: 10 }}>
          <div style={{ flex: 1, height: 3, borderRadius: 2, background: 'rgba(255,255,255,0.06)', overflow: 'hidden', position: 'relative' }}>
            <div style={{
              position: 'absolute', inset: 0,
              background: 'linear-gradient(90deg, transparent, rgba(218,119,86,0.5), transparent)',
              animation: 'shimmer 1.8s infinite',
            }} />
          </div>
          <span style={{ fontSize: 11, color: '#625850', whiteSpace: 'nowrap' }}>
            {isConnecting ? 'Pairing…' : 'Loading graph…'}
          </span>
        </div>
      )}

      {/* ── Actions ───────────────────────────────────────────────── */}
      <div style={{
        padding: '12px 14px 14px',
        borderTop: '1px solid rgba(255,255,255,0.05)',
        display: 'flex', gap: 8, marginTop: 'auto',
      }}>
        <button
          onClick={() => router.push(`/vm/${vm.code}`)}
          disabled={!canOpen}
          style={{
            flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center',
            gap: 6, padding: '8px 12px', borderRadius: 9, border: 'none',
            fontSize: 12, fontWeight: 500, cursor: canOpen ? 'pointer' : 'not-allowed',
            background: canOpen ? '#DA7756' : 'rgba(255,255,255,0.05)',
            color: canOpen ? '#fff' : '#625850',
            transition: 'background 0.15s, opacity 0.15s',
          }}
          onMouseEnter={(e) => { if (canOpen) e.currentTarget.style.background = '#E88A68' }}
          onMouseLeave={(e) => { if (canOpen) e.currentTarget.style.background = '#DA7756' }}
        >
          <ExternalLink size={12} />
          View Canvas
        </button>

        <button
          onClick={() => sendCommand(vm.code, 'refresh')}
          disabled={!isConnected}
          title="Refresh graph"
          style={{
            padding: '8px 11px', borderRadius: 9, border: '1px solid rgba(255,255,255,0.08)',
            background: 'transparent', color: isConnected ? '#A09890' : '#625850',
            cursor: isConnected ? 'pointer' : 'not-allowed', transition: 'all 0.15s',
            display: 'flex', alignItems: 'center',
          }}
          onMouseEnter={(e) => {
            if (isConnected) {
              e.currentTarget.style.borderColor = 'rgba(255,255,255,0.18)'
              e.currentTarget.style.color = '#F0EDE7'
            }
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.borderColor = 'rgba(255,255,255,0.08)'
            e.currentTarget.style.color = isConnected ? '#A09890' : '#625850'
          }}
        >
          <RefreshCw size={13} />
        </button>

        <button
          onClick={onDisconnect}
          title="Disconnect"
          style={{
            padding: '8px 11px', borderRadius: 9, border: '1px solid rgba(255,255,255,0.08)',
            background: 'transparent', color: '#625850',
            cursor: 'pointer', transition: 'all 0.15s',
            display: 'flex', alignItems: 'center',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.borderColor = 'rgba(217,85,85,0.3)'
            e.currentTarget.style.color = '#D95555'
            e.currentTarget.style.background = 'rgba(217,85,85,0.07)'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.borderColor = 'rgba(255,255,255,0.08)'
            e.currentTarget.style.color = '#625850'
            e.currentTarget.style.background = 'transparent'
          }}
        >
          <X size={13} />
        </button>
      </div>
    </div>
  )
}
