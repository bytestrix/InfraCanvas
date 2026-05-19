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

const MONO = 'var(--font-geist-mono,"Geist Mono","JetBrains Mono",ui-monospace,monospace)'

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
          background:          '#0A0A0A',
          foreground:          '#FAFAFA',
          cursor:              '#A1A1A1',
          cursorAccent:        '#0A0A0A',
          selectionBackground: 'rgba(250,250,250,0.15)',
          black:               '#1E1E1E',
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
        fontFamily: '"Geist Mono", "JetBrains Mono", "Fira Code", "Cascadia Code", Consolas, "Courier New", monospace',
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
      position: 'absolute', left: 0, right: 0, bottom: 0, height: 380,
      background: '#0A0A0A', borderTop: '1px solid #1E1E1E',
      display: 'flex', flexDirection: 'column', zIndex: 25,
      boxShadow: '0 -8px 32px rgba(0,0,0,0.5)',
    }}>
      <style>{`
        .xterm { height: 100%; }
        .xterm-viewport { border-radius: 0; overflow-y: scroll !important; }
        .xterm-viewport::-webkit-scrollbar { width: 6px; }
        .xterm-viewport::-webkit-scrollbar-track { background: transparent; }
        .xterm-viewport::-webkit-scrollbar-thumb { background: #2A2A2A; border-radius: 3px; }
        .xterm-viewport::-webkit-scrollbar-thumb:hover { background: #383838; }
      `}</style>

      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '8px 14px', borderBottom: '1px solid #1E1E1E', flexShrink: 0,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 11, fontWeight: 600, color: '#A1A1A1', letterSpacing: '0.06em', fontFamily: MONO }}>TERMINAL</span>
          <span style={{ fontSize: 11, color: '#6E6E6E', fontFamily: MONO }}>{node.label}</span>
          <span style={{ fontSize: 10, padding: '1px 6px', borderRadius: 4, background: '#1E1E1E', color: '#A1A1A1', border: '1px solid #2A2A2A', fontFamily: MONO }}>
            {layer === 'host' ? 'VM shell' : 'container exec'}
          </span>
          {status === 'connecting' && (
            <span style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 10, color: '#6E6E6E', fontFamily: MONO }}>
              <Loader2 size={10} style={{ animation: 'spin 1s linear infinite' }} /> connecting
            </span>
          )}
          {status === 'closed' && (
            <span style={{ fontSize: 10, color: '#ef4444', fontFamily: MONO }}>session ended</span>
          )}
        </div>
        <button
          onClick={() => { cleanup(); onClose() }}
          style={{ background: 'transparent', border: 'none', color: '#6E6E6E', cursor: 'pointer', padding: '3px 5px', borderRadius: 4 }}
          onMouseEnter={(e) => { e.currentTarget.style.color = '#A1A1A1'; (e.currentTarget as HTMLButtonElement).style.background = '#161616' }}
          onMouseLeave={(e) => { e.currentTarget.style.color = '#6E6E6E'; (e.currentTarget as HTMLButtonElement).style.background = 'transparent' }}
        >
          <X size={12} />
        </button>
      </div>

      {/* Terminal area */}
      <div ref={termDivRef} style={{ flex: 1, overflow: 'hidden', padding: '6px 8px' }} />
    </div>
  )
}
