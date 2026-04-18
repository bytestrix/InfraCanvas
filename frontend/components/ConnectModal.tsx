'use client'

import { useState, useEffect, useRef } from 'react'
import { X, Terminal, ArrowRight, Wifi, AlertCircle } from 'lucide-react'
import { SessionInfo } from '@/types'

interface ConnectModalProps {
  onConnect: (code: string) => void
  onClose: () => void
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'

export default function ConnectModal({ onConnect, onClose }: ConnectModalProps) {
  const [code, setCode] = useState('')
  const [error, setError] = useState('')
  const [sessions, setSessions] = useState<SessionInfo[]>([])
  const [loadingSessions, setLoadingSessions] = useState(true)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
    fetch(`${API_URL}/api/sessions`)
      .then((r) => r.json())
      .then((d: SessionInfo[]) => { setSessions(Array.isArray(d) ? d : []); setLoadingSessions(false) })
      .catch(() => { setSessions([]); setLoadingSessions(false) })
  }, [])

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = code.trim().toUpperCase()
    if (!trimmed) { setError('Enter a pair code'); return }
    if (!/^[A-Z]+-\d+$/.test(trimmed)) {
      setError('Format: WORD-1234 (e.g. APEX-1483)')
      return
    }
    onConnect(trimmed)
  }

  const steps = [
    {
      icon: <Terminal size={13} />,
      title: 'Install the agent on your VM',
      code: 'curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | bash',
      desc: 'One command — installs and starts automatically',
    },
    {
      icon: <Wifi size={13} />,
      title: 'Get your pair code',
      code: 'sudo journalctl -u infracanvas-agent -n 20 | grep "Pair code"',
      desc: 'The agent prints a code like APEX-1483',
    },
    {
      icon: <ArrowRight size={13} />,
      title: 'Enter the code below',
      desc: null,
      code: null,
    },
  ]

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        onClick={(e) => e.stopPropagation()}
        className="animate-slide-up"
        style={{
          width: '100%', maxWidth: 500, margin: '0 16px',
          background: '#0E0E1C',
          border: '1px solid rgba(138,92,246,0.18)',
          borderRadius: 18,
          overflow: 'hidden',
          boxShadow: '0 32px 80px rgba(0,0,0,0.8), 0 0 0 1px rgba(192,38,211,0.06)',
        }}
      >
        {/* Header */}
        <div style={{ padding: '20px 22px 18px', borderBottom: '1px solid rgba(138,92,246,0.1)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{
              width: 34, height: 34, borderRadius: 10,
              background: 'linear-gradient(135deg, #C026D3, #7C3AED)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 16, boxShadow: '0 2px 12px rgba(192,38,211,0.35)',
            }}>⬡</div>
            <div>
              <p style={{ fontSize: 14, fontWeight: 600, color: '#EEE8FF', margin: 0 }}>Connect a VM</p>
              <p style={{ fontSize: 11, color: '#52496E', margin: '2px 0 0' }}>Pair with an InfraCanvas agent</p>
            </div>
          </div>
          <button
            onClick={onClose}
            style={{
              width: 30, height: 30, borderRadius: 8, border: 'none',
              background: 'transparent', color: '#52496E',
              cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center',
              transition: 'background 0.15s, color 0.15s',
            }}
            onMouseEnter={(e) => { e.currentTarget.style.background = 'rgba(138,92,246,0.1)'; e.currentTarget.style.color = '#8B82B0' }}
            onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = '#52496E' }}
          >
            <X size={15} />
          </button>
        </div>

        {/* Body */}
        <div style={{ padding: '22px 22px 20px' }}>

          {/* Steps */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
            {steps.map((step, i) => (
              <div key={i} style={{ display: 'flex', gap: 14 }}>
                {/* Step column */}
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', flexShrink: 0 }}>
                  <div style={{
                    width: 26, height: 26, borderRadius: '50%',
                    background: 'rgba(192,38,211,0.12)',
                    border: '1px solid rgba(192,38,211,0.25)',
                    color: '#C026D3',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: 11, fontWeight: 600, flexShrink: 0,
                  }}>
                    {i + 1}
                  </div>
                  {i < steps.length - 1 && (
                    <div style={{ width: 1, flex: 1, minHeight: 16, background: 'rgba(138,92,246,0.1)', margin: '4px 0' }} />
                  )}
                </div>

                {/* Content */}
                <div style={{ paddingBottom: i < steps.length - 1 ? 18 : 0, flex: 1 }}>
                  <p style={{ fontSize: 12, fontWeight: 600, color: '#EEE8FF', margin: '2px 0 4px' }}>
                    {step.title}
                  </p>
                  {step.desc && (
                    <p style={{ fontSize: 11, color: '#52496E', margin: '0 0 8px', lineHeight: 1.5 }}>
                      {step.desc}
                    </p>
                  )}
                  {step.code && (
                    <div style={{
                      padding: '9px 12px', borderRadius: 9,
                      background: '#08080E', border: '1px solid rgba(138,92,246,0.12)',
                      fontFamily: 'JetBrains Mono, monospace', fontSize: 11,
                      color: '#A78BFA', wordBreak: 'break-all',
                    }}>
                      <span style={{ color: '#52496E', marginRight: 8 }}>$</span>
                      <span className="select-all">{step.code}</span>
                    </div>
                  )}

                  {/* Input form on last step */}
                  {i === steps.length - 1 && (
                    <form onSubmit={handleSubmit} style={{ marginTop: 8 }}>
                      <div style={{ display: 'flex', gap: 8 }}>
                        <div style={{ flex: 1, position: 'relative' }}>
                          <input
                            ref={inputRef}
                            type="text"
                            value={code}
                            onChange={(e) => { setCode(e.target.value.toUpperCase()); setError('') }}
                            placeholder="APEX-1483"
                            style={{
                              width: '100%', padding: '10px 14px',
                              borderRadius: 9, background: '#08080E',
                              border: `1px solid ${error ? 'rgba(248,113,113,0.5)' : 'rgba(138,92,246,0.18)'}`,
                              color: '#EEE8FF', fontSize: 14, fontWeight: 500,
                              fontFamily: 'JetBrains Mono, monospace', outline: 'none',
                              letterSpacing: '0.06em', transition: 'border-color 0.15s',
                            }}
                            onFocus={(e) => { if (!error) e.target.style.borderColor = 'rgba(192,38,211,0.5)' }}
                            onBlur={(e) => { if (!error) e.target.style.borderColor = 'rgba(138,92,246,0.18)' }}
                          />
                        </div>
                        <button
                          type="submit"
                          style={{
                            padding: '10px 18px', borderRadius: 9, border: 'none',
                            background: 'linear-gradient(135deg, #C026D3, #7C3AED)',
                            color: '#fff', fontSize: 13, fontWeight: 500, cursor: 'pointer',
                            display: 'flex', alignItems: 'center', gap: 6,
                            transition: 'opacity 0.15s',
                            whiteSpace: 'nowrap',
                            boxShadow: '0 2px 12px rgba(192,38,211,0.3)',
                          }}
                          onMouseEnter={(e) => { e.currentTarget.style.opacity = '0.85' }}
                          onMouseLeave={(e) => { e.currentTarget.style.opacity = '1' }}
                        >
                          Connect <ArrowRight size={13} />
                        </button>
                      </div>
                      {error && (
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 8 }}>
                          <AlertCircle size={12} style={{ color: '#F87171', flexShrink: 0 }} />
                          <p style={{ fontSize: 11, color: '#F87171', margin: 0 }}>{error}</p>
                        </div>
                      )}
                    </form>
                  )}
                </div>
              </div>
            ))}
          </div>

          {/* Active sessions */}
          {!loadingSessions && sessions.length > 0 && (
            <div style={{ marginTop: 20, paddingTop: 18, borderTop: '1px solid rgba(138,92,246,0.08)' }}>
              <p style={{ fontSize: 11, fontWeight: 600, color: '#52496E', margin: '0 0 10px', letterSpacing: '0.08em', textTransform: 'uppercase' }}>
                Active sessions
              </p>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 7 }}>
                {sessions.map((s) => (
                  <button
                    key={s.code}
                    onClick={() => onConnect(s.code)}
                    style={{
                      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                      padding: '10px 14px', borderRadius: 10, border: '1px solid rgba(138,92,246,0.12)',
                      background: '#08080E', cursor: 'pointer', textAlign: 'left',
                      transition: 'border-color 0.15s, background 0.15s',
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.borderColor = 'rgba(192,38,211,0.3)'
                      e.currentTarget.style.background = 'rgba(192,38,211,0.05)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.borderColor = 'rgba(138,92,246,0.12)'
                      e.currentTarget.style.background = '#08080E'
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                      <span style={{ width: 7, height: 7, borderRadius: '50%', background: s.paired ? '#FBBF24' : '#4ADE80', flexShrink: 0, display: 'block' }} />
                      <div>
                        <p style={{ fontSize: 13, fontWeight: 500, color: '#EEE8FF', margin: 0 }}>
                          {s.hostname || 'Unknown host'}
                        </p>
                        <p style={{ fontSize: 11, color: '#52496E', margin: '2px 0 0', fontFamily: 'JetBrains Mono, monospace' }}>
                          {s.code}
                        </p>
                      </div>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span style={{
                        fontSize: 11, padding: '2px 9px', borderRadius: 20, fontWeight: 500,
                        background: s.paired ? 'rgba(251,191,36,0.1)' : 'rgba(74,222,128,0.1)',
                        color: s.paired ? '#FBBF24' : '#4ADE80',
                      }}>
                        {s.paired ? 'In use' : 'Available'}
                      </span>
                      <ArrowRight size={13} style={{ color: '#52496E' }} />
                    </div>
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
