'use client'

import { useParams, useRouter } from 'next/navigation'
import { useEffect } from 'react'
import { useVMStore } from '@/store/vmStore'
import { connectVM } from '@/lib/wsManager'
import InfraCanvas from '@/components/canvas/InfraCanvas'
import { ArrowLeft, AlertCircle } from 'lucide-react'

export default function VMCanvasPage() {
  const params = useParams()
  const router = useRouter()
  const code = params.code as string
  const { vms } = useVMStore()
  const vm = vms[code]

  useEffect(() => {
    if (!vm) connectVM(code)
  }, [code, vm])

  if (!vm) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#111110' }}>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16 }}>
          <div style={{
            width: 38, height: 38, borderRadius: '50%',
            border: '2.5px solid rgba(218,119,86,0.25)',
            borderTopColor: '#DA7756',
            animation: 'spin 0.85s linear infinite',
          }} className="animate-spin" />
          <p style={{ fontSize: 13, color: '#625850' }}>Connecting to {code}…</p>
        </div>
      </div>
    )
  }

  if (vm.status === 'error') {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#111110' }}>
        <div style={{
          maxWidth: 420, width: '100%', margin: '0 16px',
          background: '#191817', border: '1px solid rgba(255,255,255,0.08)',
          borderRadius: 16, padding: '28px 28px 24px',
          display: 'flex', flexDirection: 'column', gap: 18,
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
            <div style={{
              width: 42, height: 42, borderRadius: 11,
              background: 'rgba(217,85,85,0.1)', border: '1px solid rgba(217,85,85,0.2)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              color: '#D95555', flexShrink: 0,
            }}>
              <AlertCircle size={20} />
            </div>
            <div>
              <p style={{ fontSize: 15, fontWeight: 600, color: '#F0EDE7', margin: 0 }}>Connection failed</p>
              <p style={{ fontSize: 12, color: '#625850', margin: '3px 0 0' }}>Could not connect to {code}</p>
            </div>
          </div>

          <p style={{
            fontSize: 12, color: '#A09890', margin: 0,
            padding: '12px 14px', borderRadius: 9,
            background: '#111110', border: '1px solid rgba(255,255,255,0.07)',
            fontFamily: 'JetBrains Mono, monospace', lineHeight: 1.6,
          }}>
            {vm.error}
          </p>

          <button
            onClick={() => router.push('/')}
            style={{
              display: 'flex', alignItems: 'center', gap: 8,
              padding: '10px 16px', borderRadius: 9,
              background: 'transparent', border: '1px solid rgba(255,255,255,0.1)',
              color: '#A09890', fontSize: 13, fontWeight: 500,
              cursor: 'pointer', transition: 'all 0.15s',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.borderColor = 'rgba(255,255,255,0.2)'
              e.currentTarget.style.color = '#F0EDE7'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.borderColor = 'rgba(255,255,255,0.1)'
              e.currentTarget.style.color = '#A09890'
            }}
          >
            <ArrowLeft size={15} />
            Back to Dashboard
          </button>
        </div>
      </div>
    )
  }

  return <InfraCanvas vm={vm} onBack={() => router.push('/')} />
}
