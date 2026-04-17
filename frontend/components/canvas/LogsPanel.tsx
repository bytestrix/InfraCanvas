'use client'

import { useState, useEffect, useRef, useCallback } from 'react'
import { X, Download, RefreshCw, Loader2 } from 'lucide-react'
import { sendAction, subscribeLogData } from '@/lib/wsManager'
import type { GraphNode } from '@/types'

interface LogsPanelProps {
  node: GraphNode
  vmCode: string
  onClose: () => void
}

export default function LogsPanel({ node, vmCode, onClose }: LogsPanelProps) {
  const [lines, setLines] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const requestIDRef = useRef<string>('')

  const fetchLogs = useCallback(() => {
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

    sendAction(vmCode, {
      action_id: requestID,
      type: 'docker_logs',
      target: {
        layer: 'docker',
        entity_type: 'container',
        entity_id: node.id,
      },
      parameters: { tail: '200' },
    })

    // Safety timeout
    setTimeout(() => {
      setLoading(false)
      unsub()
    }, 35_000)
  }, [node.id, vmCode])

  useEffect(() => {
    fetchLogs()
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
    a.download = `${node.label}-logs.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div style={{
      position: 'absolute', left: 0, right: 340, bottom: 0, height: 320,
      background: '#080812', borderTop: '1px solid #1e1e3a',
      display: 'flex', flexDirection: 'column', zIndex: 25,
      boxShadow: '0 -8px 32px rgba(0,0,0,0.5)',
    }}>
      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '8px 14px', borderBottom: '1px solid #1e1e3a', flexShrink: 0,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 11, fontWeight: 600, color: '#94a3b8', letterSpacing: '0.06em' }}>
            LOGS
          </span>
          <span style={{ fontSize: 11, color: '#475569', fontFamily: 'monospace' }}>
            {node.label}
          </span>
          {loading && <Loader2 size={11} color="#6366f1" style={{ animation: 'spin 1s linear infinite' }} />}
        </div>
        <div style={{ display: 'flex', gap: 4 }}>
          <IconBtn onClick={fetchLogs} title="Refresh logs"><RefreshCw size={12} /></IconBtn>
          <IconBtn onClick={downloadLogs} title="Download"><Download size={12} /></IconBtn>
          <IconBtn onClick={onClose} title="Close"><X size={12} /></IconBtn>
        </div>
      </div>

      {/* Log output */}
      <div style={{
        flex: 1, overflowY: 'auto', padding: '8px 14px',
        fontFamily: 'JetBrains Mono, Consolas, monospace', fontSize: 11, lineHeight: '1.6',
        color: '#94a3b8',
      }}>
        {error && (
          <div style={{ color: '#ef4444', marginBottom: 6 }}>Error: {error}</div>
        )}
        {lines.length === 0 && !loading && !error && (
          <div style={{ color: '#334155', fontStyle: 'italic' }}>No log output.</div>
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
        background: 'transparent', border: 'none', color: '#475569',
        cursor: 'pointer', padding: '3px 5px', borderRadius: 4,
        display: 'flex', alignItems: 'center',
      }}
      onMouseEnter={(e) => { e.currentTarget.style.color = '#94a3b8'; e.currentTarget.style.background = '#13131f' }}
      onMouseLeave={(e) => { e.currentTarget.style.color = '#475569'; e.currentTarget.style.background = 'transparent' }}
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
  // Highlight timestamps (ISO-like patterns)
  if (/^\d{4}-\d{2}-\d{2}/.test(trimmed)) {
    const dateEnd = trimmed.indexOf(' ', 20)
    if (dateEnd > 0) {
      return (
        <>
          <span style={{ color: '#475569' }}>{line.slice(0, line.indexOf(trimmed) + dateEnd + 1)}</span>
          <span>{line.slice(line.indexOf(trimmed) + dateEnd + 1)}</span>
        </>
      )
    }
  }
  return line
}
