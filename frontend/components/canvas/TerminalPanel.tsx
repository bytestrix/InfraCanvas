'use client'

import { useEffect, useRef, useState, useCallback } from 'react'
import { X, Loader2 } from 'lucide-react'
import {
  sendExecStart,
  sendExecInput,
  sendExecResize,
  sendExecEnd,
  subscribeExecData,
  subscribeExecEnd,
} from '@/lib/wsManager'
import type { GraphNode } from '@/types'

interface TerminalPanelProps {
  node: GraphNode
  vmCode: string
  layer?: 'docker' | 'host'
  onClose: () => void
}

function generateID(): string {
  return Math.random().toString(36).slice(2) + Date.now().toString(36)
}

export default function TerminalPanel({ node, vmCode, layer = 'docker', onClose }: TerminalPanelProps) {
  const termDivRef = useRef<HTMLDivElement>(null)
  const termRef    = useRef<any>(null)
  const fitRef     = useRef<any>(null)
  const sessionID  = useRef<string>('')
  const [status, setStatus] = useState<'connecting' | 'connected' | 'closed'>('connecting')

  const containerID = node.id // e.g. "container/abc123" — agent normalizes

  // Cleanup on unmount or when node changes
  const cleanup = useCallback(() => {
    if (sessionID.current) {
      sendExecEnd(vmCode, sessionID.current)
      sessionID.current = ''
    }
    termRef.current?.dispose()
    termRef.current = null
    fitRef.current = null
  }, [vmCode])

  useEffect(() => {
    if (!termDivRef.current) return

    let unmounted = false
    let unsubData: (() => void) | null = null
    let unsubEnd:  (() => void) | null = null
    const sid = generateID()
    sessionID.current = sid

    // Dynamically import xterm (SSR-safe)
    Promise.all([
      import('@xterm/xterm'),
      import('@xterm/addon-fit'),
    ]).then(([{ Terminal }, { FitAddon }]) => {
      if (unmounted || !termDivRef.current) return

      const term = new Terminal({
        theme: {
          background: '#070711',
          foreground: '#e2e8f0',
          cursor:     '#6366f1',
          black:      '#0a0a16',
          red:        '#ef4444',
          green:      '#10b981',
          yellow:     '#f59e0b',
          blue:       '#3b82f6',
          magenta:    '#8b5cf6',
          cyan:       '#06b6d4',
          white:      '#cbd5e1',
        },
        fontFamily: 'JetBrains Mono, Consolas, monospace',
        fontSize: 12,
        lineHeight: 1.4,
        cursorBlink: true,
        scrollback: 2000,
      })

      const fit = new FitAddon()
      term.loadAddon(fit)
      term.open(termDivRef.current)
      fit.fit()

      termRef.current = term
      fitRef.current  = fit

      // Forward keystrokes to agent
      term.onData((data) => {
        sendExecInput(vmCode, sid, btoa(data))
      })

      // Subscribe to output from agent
      unsubData = subscribeExecData(sid, (d) => {
        if (d.error) {
          term.writeln(`\r\n\x1b[31mError: ${d.error}\x1b[0m`)
          setStatus('closed')
          return
        }
        if (d.data) {
          try {
            term.write(atob(d.data))
          } catch {}
        }
      })

      unsubEnd = subscribeExecEnd(sid, () => {
        term.writeln('\r\n\x1b[90m[session ended]\x1b[0m')
        setStatus('closed')
      })

      // Start exec on the agent
      const cmd = layer === 'host' ? ['/bin/bash'] : ['/bin/sh']
      sendExecStart(vmCode, sid, containerID, cmd, term.rows, term.cols, layer)
      setStatus('connected')

      // Resize observer
      const ro = new ResizeObserver(() => {
        if (fitRef.current && termRef.current) {
          fitRef.current.fit()
          sendExecResize(vmCode, sid, termRef.current.rows, termRef.current.cols)
        }
      })
      if (termDivRef.current) ro.observe(termDivRef.current)

      return () => ro.disconnect()
    }).catch((err) => {
      console.error('[TerminalPanel] xterm load failed:', err)
    })

    return () => {
      unmounted = true
      unsubData?.()
      unsubEnd?.()
      cleanup()
    }
  }, [containerID, vmCode, cleanup])

  return (
    <div style={{
      position: 'absolute', left: 0, right: 340, bottom: 0, height: 320,
      background: '#070711', borderTop: '1px solid #1e1e3a',
      display: 'flex', flexDirection: 'column', zIndex: 25,
      boxShadow: '0 -8px 32px rgba(0,0,0,0.5)',
    }}>
      {/* xterm CSS */}
      <style>{`.xterm { height: 100%; padding: 0; } .xterm-viewport { border-radius: 0; }`}</style>

      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '8px 14px', borderBottom: '1px solid #1e1e3a', flexShrink: 0,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 11, fontWeight: 600, color: '#94a3b8', letterSpacing: '0.06em' }}>TERMINAL</span>
          <span style={{ fontSize: 11, color: '#475569', fontFamily: 'monospace' }}>{node.label}</span>
          <span style={{ fontSize: 10, padding: '1px 6px', borderRadius: 4, background: layer === 'host' ? '#6366f120' : '#10b98120', color: layer === 'host' ? '#818cf8' : '#34d399', border: `1px solid ${layer === 'host' ? '#6366f130' : '#10b98130'}` }}>
            {layer === 'host' ? 'VM shell' : 'container exec'}
          </span>
          {status === 'connecting' && (
            <span style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 10, color: '#475569' }}>
              <Loader2 size={10} style={{ animation: 'spin 1s linear infinite' }} /> connecting
            </span>
          )}
          {status === 'closed' && (
            <span style={{ fontSize: 10, color: '#ef4444' }}>session ended</span>
          )}
        </div>
        <button
          onClick={() => { cleanup(); onClose() }}
          style={{ background: 'transparent', border: 'none', color: '#475569', cursor: 'pointer', padding: '3px 5px', borderRadius: 4 }}
          onMouseEnter={(e) => { e.currentTarget.style.color = '#94a3b8' }}
          onMouseLeave={(e) => { e.currentTarget.style.color = '#475569' }}
        >
          <X size={12} />
        </button>
      </div>

      {/* Terminal area */}
      <div ref={termDivRef} style={{ flex: 1, overflow: 'hidden', padding: '4px 4px 4px 8px' }} />
    </div>
  )
}
