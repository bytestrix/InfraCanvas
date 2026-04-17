'use client'

import { useState, useEffect, useRef } from 'react'
import { X, Terminal, ArrowRight, Wifi, Check, AlertCircle } from 'lucide-react'
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

    // Fetch active sessions
    fetch(`${API_URL}/api/sessions`)
      .then((r) => r.json())
      .then((data: SessionInfo[]) => {
        setSessions(Array.isArray(data) ? data : [])
        setLoadingSessions(false)
      })
      .catch(() => {
        setSessions([])
        setLoadingSessions(false)
      })
  }, [])

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = code.trim().toUpperCase()
    if (!trimmed) {
      setError('Please enter a pair code')
      return
    }
    // Basic format validation: WORD-1234
    if (!/^[A-Z]+-\d+$/.test(trimmed)) {
      setError('Code format should be WORD-1234 (e.g. WOLF-1234)')
      return
    }
    onConnect(trimmed)
  }

  function handleSelectSession(sessionCode: string) {
    onConnect(sessionCode)
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Escape') onClose()
  }

  const steps = [
    {
      num: 1,
      icon: <Terminal size={14} />,
      title: 'Start the server on your VM',
      code: 'infracanvas-server &',
      desc: 'Run on the target VM (once per machine)',
    },
    {
      num: 2,
      icon: <Wifi size={14} />,
      title: 'Start the agent on your VM',
      code: 'infracanvas start',
      desc: 'Run on the same VM — displays your pair code',
    },
    {
      num: 3,
      icon: <ArrowRight size={14} />,
      title: 'Enter the pair code',
      desc: 'The agent will display a code like WOLF-1234',
    },
  ]

  return (
    <div className="modal-backdrop" onClick={onClose} onKeyDown={handleKeyDown}>
      <div
        className="w-full max-w-lg mx-4 rounded-2xl animate-slide-up overflow-hidden"
        style={{
          background: '#0e0e1a',
          border: '1px solid #1e1e3a',
          boxShadow: '0 24px 80px rgba(0,0,0,0.6), 0 0 0 1px rgba(99,102,241,0.05)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          className="flex items-center justify-between px-6 py-4 border-b"
          style={{ borderColor: '#1e1e3a' }}
        >
          <div className="flex items-center gap-3">
            <div
              className="w-8 h-8 rounded-lg flex items-center justify-center"
              style={{ background: 'linear-gradient(135deg, #6366f1, #8b5cf6)' }}
            >
              ⬡
            </div>
            <div>
              <h2 className="font-semibold text-sm" style={{ color: '#e2e8f0' }}>
                Connect a VM
              </h2>
              <p className="text-xs" style={{ color: '#475569' }}>
                Pair with an InfraCanvas agent
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-lg flex items-center justify-center transition-colors"
            style={{ color: '#475569' }}
            onMouseEnter={(e) => {
              e.currentTarget.style.background = '#13131f'
              e.currentTarget.style.color = '#94a3b8'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.background = 'transparent'
              e.currentTarget.style.color = '#475569'
            }}
          >
            <X size={16} />
          </button>
        </div>

        {/* Body */}
        <div className="px-6 py-5 flex flex-col gap-5">
          {/* Steps */}
          <div className="flex flex-col gap-3">
            {steps.map((step, i) => (
              <div key={step.num} className="flex gap-3">
                {/* Step indicator */}
                <div className="flex flex-col items-center gap-1 flex-shrink-0">
                  <div
                    className="w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold flex-shrink-0"
                    style={{
                      background: i < 2 ? 'rgba(99,102,241,0.15)' : 'rgba(99,102,241,0.15)',
                      color: '#6366f1',
                      border: '1px solid rgba(99,102,241,0.2)',
                    }}
                  >
                    {step.num}
                  </div>
                  {i < steps.length - 1 && (
                    <div className="w-px flex-1 min-h-[8px]" style={{ background: '#1e1e3a' }} />
                  )}
                </div>

                {/* Content */}
                <div className="flex-1 pb-1">
                  <p className="text-xs font-semibold mb-0.5" style={{ color: '#e2e8f0' }}>
                    {step.title}
                  </p>
                  {step.desc && (
                    <p className="text-xs mb-1.5" style={{ color: '#475569' }}>
                      {step.desc}
                    </p>
                  )}
                  {step.code && (
                    <div
                      className="flex items-center gap-2 px-3 py-2 rounded-lg code-text text-xs"
                      style={{
                        background: '#070711',
                        border: '1px solid #1e1e3a',
                        color: '#a78bfa',
                      }}
                    >
                      <span style={{ color: '#475569' }}>$</span>
                      <span className="select-all">{step.code}</span>
                    </div>
                  )}
                  {step.num === 3 && (
                    <form onSubmit={handleSubmit} className="mt-2">
                      <div className="flex gap-2">
                        <div className="flex-1 relative">
                          <input
                            ref={inputRef}
                            type="text"
                            value={code}
                            onChange={(e) => {
                              setCode(e.target.value.toUpperCase())
                              setError('')
                            }}
                            placeholder="WOLF-1234"
                            className="w-full px-3 py-2.5 rounded-lg text-sm font-mono outline-none transition-all"
                            style={{
                              background: '#070711',
                              border: `1px solid ${error ? '#ef4444' : '#2d2d52'}`,
                              color: '#e2e8f0',
                              letterSpacing: '0.05em',
                            }}
                            onFocus={(e) => {
                              if (!error) e.target.style.borderColor = '#6366f1'
                            }}
                            onBlur={(e) => {
                              if (!error) e.target.style.borderColor = '#2d2d52'
                            }}
                          />
                        </div>
                        <button
                          type="submit"
                          className="px-4 py-2.5 rounded-lg text-sm font-semibold transition-all flex items-center gap-2"
                          style={{
                            background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
                            color: '#fff',
                          }}
                          onMouseEnter={(e) => { e.currentTarget.style.opacity = '0.85' }}
                          onMouseLeave={(e) => { e.currentTarget.style.opacity = '1' }}
                        >
                          Connect
                          <ArrowRight size={14} />
                        </button>
                      </div>
                      {error && (
                        <div className="flex items-center gap-1.5 mt-2">
                          <AlertCircle size={12} style={{ color: '#ef4444' }} />
                          <p className="text-xs" style={{ color: '#ef4444' }}>
                            {error}
                          </p>
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
            <div>
              <div
                className="border-t pt-4"
                style={{ borderColor: '#1e1e3a' }}
              >
                <p className="text-xs font-semibold mb-2" style={{ color: '#475569' }}>
                  ACTIVE SESSIONS
                </p>
                <div className="flex flex-col gap-2">
                  {sessions.map((session) => (
                    <button
                      key={session.code}
                      onClick={() => handleSelectSession(session.code)}
                      className="flex items-center justify-between px-3 py-2.5 rounded-lg text-left transition-all"
                      style={{
                        background: '#070711',
                        border: '1px solid #1e1e3a',
                      }}
                      onMouseEnter={(e) => {
                        e.currentTarget.style.borderColor = '#6366f1'
                        e.currentTarget.style.background = 'rgba(99,102,241,0.04)'
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.borderColor = '#1e1e3a'
                        e.currentTarget.style.background = '#070711'
                      }}
                    >
                      <div className="flex items-center gap-3">
                        <div
                          className="w-2 h-2 rounded-full flex-shrink-0"
                          style={{ background: session.paired ? '#f59e0b' : '#10b981' }}
                        />
                        <div>
                          <p className="text-xs font-semibold" style={{ color: '#e2e8f0' }}>
                            {session.hostname || 'Unknown host'}
                          </p>
                          <p className="text-xs font-mono" style={{ color: '#475569' }}>
                            {session.code}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <span
                          className="text-xs px-2 py-0.5 rounded-full"
                          style={{
                            background: session.paired
                              ? 'rgba(245,158,11,0.1)'
                              : 'rgba(16,185,129,0.1)',
                            color: session.paired ? '#f59e0b' : '#10b981',
                          }}
                        >
                          {session.paired ? 'In use' : 'Available'}
                        </span>
                        <ArrowRight size={14} style={{ color: '#475569' }} />
                      </div>
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
