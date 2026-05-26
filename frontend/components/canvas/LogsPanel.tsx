'use client'

import { useState, useEffect, useRef, useCallback } from 'react'
import { X, Download, RefreshCw, Loader2, ChevronDown } from 'lucide-react'
import { sendAction, subscribeLogData } from '@/lib/wsManager'
import type { GraphNode, GraphEdge } from '@/types'

interface LogsPanelProps {
  node: GraphNode
  vmCode: string
  onClose: () => void
  allNodes?: GraphNode[]
  allEdges?: GraphEdge[]
}

const MONO = 'var(--font-geist-mono,"Geist Mono","JetBrains Mono",ui-monospace,monospace)'
const WORKLOAD_TYPES = new Set(['deployment', 'statefulset', 'daemonset'])

export default function LogsPanel({ node, vmCode, onClose, allNodes = [], allEdges = [] }: LogsPanelProps) {
  const isWorkload = WORKLOAD_TYPES.has(node.type)

  // For workload nodes: find owned pods via OWNS edges
  const ownedPods = isWorkload
    ? allEdges
        .filter((e) => e.source === node.id && e.type === 'OWNS')
        .map((e) => allNodes.find((n) => n.id === e.target && n.type === 'pod'))
        .filter((n): n is GraphNode => n !== undefined)
    : []

  const [selectedPod, setSelectedPod] = useState<GraphNode | null>(
    isWorkload ? (ownedPods[0] ?? null) : null
  )
  const [pickerOpen, setPickerOpen] = useState(false)
  const [lines, setLines] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const requestIDRef = useRef<string>('')
  const unsubRef = useRef<(() => void) | null>(null)

  // The actual node whose logs we stream
  const targetNode = isWorkload ? selectedPod : node

  const fetchLogs = useCallback(() => {
    if (!targetNode) return
    if (unsubRef.current) { unsubRef.current(); unsubRef.current = null }

    const requestID = `logs-${Date.now()}`
    requestIDRef.current = requestID
    setLines([])
    setLoading(true)
    setError(null)

    const unsub = subscribeLogData(requestID, (data) => {
      if (data.error) {
        setError(data.error)
        setLoading(false)
        unsub()
        return
      }
      if (data.lines?.length > 0) {
        setLines((prev) => [...prev, ...data.lines])
      }
      if (data.done) {
        setLoading(false)
        unsub()
      }
    })
    unsubRef.current = unsub

    const isK8sPod = targetNode.type === 'pod'
    const isHost = targetNode.type === 'host'
    sendAction(vmCode, {
      action_id: requestID,
      type: isHost ? 'host_logs' : isK8sPod ? 'k8s_logs' : 'docker_logs',
      target: {
        layer: isHost ? 'host' : isK8sPod ? 'kubernetes' : 'docker',
        entity_type: isHost ? 'host' : isK8sPod ? 'pod' : 'container',
        entity_id: targetNode.id,
        namespace: isK8sPod ? ((targetNode.metadata?.namespace as string) || 'default') : undefined,
      },
      parameters: { tail: '200' },
    })

    setTimeout(() => { setLoading(false); unsub() }, 35_000)
  }, [targetNode, vmCode])

  useEffect(() => {
    if (targetNode) fetchLogs()
    return () => { if (unsubRef.current) unsubRef.current() }
  }, [fetchLogs])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines])

  function downloadLogs() {
    const text = lines.join('\n')
    const blob = new Blob([text], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${targetNode?.label ?? node.label}-logs.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div style={{
      position: 'absolute', left: 0, right: 0, bottom: 0,
      height: isWorkload ? 352 : 320,
      background: 'var(--bg)', borderTop: '1px solid var(--line)',
      display: 'flex', flexDirection: 'column', zIndex: 25,
      boxShadow: '0 -8px 32px rgba(0,0,0,0.5)',
    }}>
      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '8px 14px', borderBottom: '1px solid var(--line)', flexShrink: 0,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--ink2)', letterSpacing: '0.06em', fontFamily: MONO }}>
            LOGS
          </span>
          <span style={{ fontSize: 11, color: 'var(--ink3)', fontFamily: MONO }}>
            {node.label}
          </span>
          {loading && <Loader2 size={11} color="var(--ink2)" style={{ animation: 'spin 1s linear infinite' }} />}
        </div>
        <div style={{ display: 'flex', gap: 4 }}>
          {targetNode && <IconBtn onClick={fetchLogs} title="Refresh logs"><RefreshCw size={12} /></IconBtn>}
          {targetNode && <IconBtn onClick={downloadLogs} title="Download"><Download size={12} /></IconBtn>}
          <IconBtn onClick={onClose} title="Close"><X size={12} /></IconBtn>
        </div>
      </div>

      {/* Pod picker (workload nodes only) */}
      {isWorkload && (
        <div style={{
          padding: '6px 14px', borderBottom: '1px solid var(--line)',
          display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0,
          background: 'var(--surface)',
        }}>
          <span style={{ fontSize: 10, color: 'var(--ink4)', fontFamily: MONO, whiteSpace: 'nowrap' }}>POD</span>
          {ownedPods.length === 0 ? (
            <span style={{ fontSize: 11, color: 'var(--ink4)', fontFamily: MONO }}>No pods found for this {node.type}</span>
          ) : (
            <div style={{ position: 'relative' }}>
              <button
                onClick={() => setPickerOpen((p) => !p)}
                style={{
                  display: 'flex', alignItems: 'center', gap: 4,
                  background: 'var(--bg)', border: '1px solid var(--line)',
                  borderRadius: 5, padding: '3px 8px', cursor: 'pointer',
                  color: 'var(--ink2)', fontFamily: MONO, fontSize: 11,
                }}
              >
                <span style={{
                  display: 'inline-block', width: 6, height: 6, borderRadius: '50%',
                  background: selectedPod?.health === 'healthy' ? '#22c55e' :
                              selectedPod?.health === 'unhealthy' ? '#ef4444' : '#f59e0b',
                  flexShrink: 0,
                }} />
                {selectedPod?.label ?? 'select pod'}
                <ChevronDown size={10} />
              </button>
              {pickerOpen && (
                <div style={{
                  position: 'absolute', top: '100%', left: 0, zIndex: 50,
                  background: 'var(--bg)', border: '1px solid var(--line)',
                  borderRadius: 6, marginTop: 2, minWidth: 220,
                  boxShadow: '0 4px 16px rgba(0,0,0,0.3)', overflow: 'hidden',
                }}>
                  {ownedPods.map((pod) => (
                    <button
                      key={pod.id}
                      onClick={() => { setSelectedPod(pod); setPickerOpen(false) }}
                      style={{
                        display: 'flex', alignItems: 'center', gap: 6,
                        width: '100%', padding: '6px 10px',
                        background: pod.id === selectedPod?.id ? 'var(--surface-2)' : 'transparent',
                        border: 'none', cursor: 'pointer', textAlign: 'left',
                        color: pod.id === selectedPod?.id ? 'var(--ink)' : 'var(--ink2)',
                        fontFamily: MONO, fontSize: 11,
                      }}
                      onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--surface-2)' }}
                      onMouseLeave={(e) => { e.currentTarget.style.background = pod.id === selectedPod?.id ? 'var(--surface-2)' : 'transparent' }}
                    >
                      <span style={{
                        display: 'inline-block', width: 6, height: 6, borderRadius: '50%', flexShrink: 0,
                        background: pod.health === 'healthy' ? '#22c55e' :
                                    pod.health === 'unhealthy' ? '#ef4444' : '#f59e0b',
                      }} />
                      {pod.label}
                      <span style={{ marginLeft: 'auto', fontSize: 10, color: 'var(--ink4)' }}>
                        {pod.metadata?.namespace as string ?? ''}
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Log output */}
      <div style={{
        flex: 1, overflowY: 'auto', padding: '8px 14px',
        fontFamily: MONO, fontSize: 11, lineHeight: '1.6',
        color: 'var(--ink2)',
      }}>
        {!targetNode && (
          <div style={{ color: 'var(--ink4)', fontStyle: 'italic' }}>Select a pod above to view logs.</div>
        )}
        {error && <div style={{ color: '#ef4444', marginBottom: 6 }}>Error: {error}</div>}
        {targetNode && lines.length === 0 && !loading && !error && (
          <div style={{ color: 'var(--ink4)', fontStyle: 'italic' }}>No log output.</div>
        )}
        {lines.map((line, i) => (
          <div key={i} style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
            {colorizeLog(line)}
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  )
}

function IconBtn({ children, onClick, title }: { children: React.ReactNode; onClick: () => void; title?: string }) {
  return (
    <button
      onClick={onClick}
      title={title}
      style={{
        background: 'transparent', border: 'none', color: 'var(--ink3)',
        cursor: 'pointer', padding: '3px 5px', borderRadius: 4,
        display: 'flex', alignItems: 'center',
      }}
      onMouseEnter={(e) => { e.currentTarget.style.color = 'var(--ink2)'; e.currentTarget.style.background = 'var(--surface-2)' }}
      onMouseLeave={(e) => { e.currentTarget.style.color = 'var(--ink3)'; e.currentTarget.style.background = 'transparent' }}
    >
      {children}
    </button>
  )
}

function colorizeLog(line: string): React.ReactNode {
  const trimmed = line.trim()
  if (/\berror\b|exception|fatal|ERRO/i.test(trimmed)) {
    return <span style={{ color: '#ef4444' }}>{line}</span>
  }
  if (/\bwarn(ing)?\b|WARN/i.test(trimmed)) {
    return <span style={{ color: '#f59e0b' }}>{line}</span>
  }
  if (/\binfo\b|INFO/i.test(trimmed)) {
    return <span style={{ color: '#60a5fa' }}>{line}</span>
  }
  if (/^\d{4}-\d{2}-\d{2}/.test(trimmed)) {
    const dateEnd = trimmed.indexOf(' ', 20)
    if (dateEnd > 0) {
      return (
        <>
          <span style={{ color: 'var(--ink4)' }}>{line.slice(0, line.indexOf(trimmed) + dateEnd + 1)}</span>
          <span>{line.slice(line.indexOf(trimmed) + dateEnd + 1)}</span>
        </>
      )
    }
  }
  return line
}
