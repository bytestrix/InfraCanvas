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

  const containerID = node.id

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

    const sid = generateID()
    sessionID.current = sid
    let active = true
    let unsubData: (() => void) | null = null
    let unsubEnd:  (() => void) | null = null
    let ro: ResizeObserver | null = null

    Promise.all([
      import('@xterm/xterm'),
      import('@xterm/addon-fit'),
    ]).then(([{ Terminal }, { FitAddon }]) => {
      if (!active || !termDivRef.current) return

      const term = new Terminal({
        theme: {
          background:          '#0d0d17',
          foreground:          '#cdd6f4',
          cursor:              '#c026d3',
          cursorAccent:        '#0d0d17',
          selectionBackground: 'rgba(192,38,211,0.3)',
          black:               '#1e1e2e',
          red:                 '#f38ba8',
          green:               '#a6e3a1',
          yellow:              '#f9e2af',
          blue:                '#89b4fa',
          magenta:             '#cba6f7',
          cyan:                '#89dceb',
          white:               '#cdd6f4',
          brightBlack:         '#585b70',
          brightRed:           '#f38ba8',
          brightGreen:         '#a6e3a1',
          brightYellow:        '#f9e2af',
          brightBlue:          '#89b4fa',
          brightMagenta:       '#cba6f7',
          brightCyan:          '#89dceb',
          brightWhite:         '#ffffff',
        },
        fontFamily: '"JetBrains Mono", "Fira Code", "Cascadia Code", Consolas, "Courier New", monospace',
        fontSize: 13,
        lineHeight: 1.5,
        cursorBlink: true,
        cursorStyle: 'block',
        scrollback: 5000,
        allowTransparency: false,
      })

      const fit = new FitAddon()
      term.loadAddon(fit)
      term.open(termDivRef.current)
      // Delay fit to let fonts render
      requestAnimationFrame(() => { if (active) fit.fit() })

      termRef.current = term
      fitRef.current  = fit

      term.onData((data) => {
        sendExecInput(vmCode, sid, btoa(data))
      })

      // Subscribe BEFORE sending exec start so no data is missed
      unsubData = subscribeExecData(sid, (d) => {
        if (d.error) {
          term.writeln(`\r\n\x1b[31mError: ${d.error}\x1b[0m`)
          setStatus('closed')
          return
        }
        if (d.data) {
          try { term.write(atob(d.data)) } catch {}
        }
      })

      unsubEnd = subscribeExecEnd(sid, () => {
        term.writeln('\r\n\x1b[90m[session ended]\x1b[0m')
        setStatus('closed')
      })

      const shellCmd = layer === 'host' ? ['/bin/bash'] : ['/bin/sh']
      sendExecStart(vmCode, sid, containerID, shellCmd, term.rows, term.cols, layer)
      setStatus('connected')

      ro = new ResizeObserver(() => {
        if (fitRef.current && termRef.current) {
          fitRef.current.fit()
          sendExecResize(vmCode, sid, termRef.current.rows, termRef.current.cols)
        }
      })
      ro.observe(termDivRef.current)
    }).catch((err) => {
      console.error('[TerminalPanel] xterm load failed:', err)
    })

    return () => {
      active = false
      unsubData?.()
      unsubEnd?.()
      ro?.disconnect()
      cleanup()
    }
  }, [containerID, vmCode, cleanup, layer])

  return (
    <div style={{
      position: 'absolute', left: 0, right: 340, bottom: 0, height: 380,
      background: '#0d0d17', borderTop: '1px solid rgba(192,38,211,0.2)',
      display: 'flex', flexDirection: 'column', zIndex: 25,
      boxShadow: '0 -8px 32px rgba(0,0,0,0.5)',
    }}>
      <style>{`
        .xterm { height: 100%; }
        .xterm-viewport { border-radius: 0; overflow-y: scroll !important; }
        .xterm-viewport::-webkit-scrollbar { width: 6px; }
        .xterm-viewport::-webkit-scrollbar-track { background: transparent; }
        .xterm-viewport::-webkit-scrollbar-thumb { background: rgba(192,38,211,0.3); border-radius: 3px; }
      `}</style>

      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '8px 14px', borderBottom: '1px solid rgba(192,38,211,0.12)', flexShrink: 0,
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
      <div ref={termDivRef} style={{ flex: 1, overflow: 'hidden', padding: '6px 8px' }} />
    </div>
  )
}
