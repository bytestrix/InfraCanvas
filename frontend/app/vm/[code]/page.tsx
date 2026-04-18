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
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#08080E' }}>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16 }}>
          <div style={{
            width: 38, height: 38, borderRadius: '50%',
            border: '2.5px solid rgba(192,38,211,0.2)',
            borderTopColor: '#C026D3',
            animation: 'spin 0.85s linear infinite',
          }} className="animate-spin" />
          <p style={{ fontSize: 13, color: '#52496E' }}>Connecting to {code}…</p>
        </div>
      </div>
    )
  }

  if (vm.status === 'error') {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#08080E' }}>
        <div style={{
          maxWidth: 420, width: '100%', margin: '0 16px',
          background: '#0E0E1C', border: '1px solid rgba(138,92,246,0.12)',
          borderRadius: 16, padding: '28px 28px 24px',
          display: 'flex', flexDirection: 'column', gap: 18,
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
            <div style={{
              width: 42, height: 42, borderRadius: 11,
              background: 'rgba(248,113,113,0.1)', border: '1px solid rgba(248,113,113,0.2)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              color: '#F87171', flexShrink: 0,
            }}>
              <AlertCircle size={20} />
            </div>
            <div>
              <p style={{ fontSize: 15, fontWeight: 600, color: '#EEE8FF', margin: 0 }}>Connection failed</p>
              <p style={{ fontSize: 12, color: '#52496E', margin: '3px 0 0' }}>Could not connect to {code}</p>
            </div>
          </div>

          <p style={{
            fontSize: 12, color: '#8B82B0', margin: 0,
            padding: '12px 14px', borderRadius: 9,
            background: '#08080E', border: '1px solid rgba(138,92,246,0.1)',
            fontFamily: 'JetBrains Mono, monospace', lineHeight: 1.6,
          }}>
            {vm.error}
          </p>

          <button
            onClick={() => router.push('/')}
            style={{
              display: 'flex', alignItems: 'center', gap: 8,
              padding: '10px 16px', borderRadius: 9,
              background: 'transparent', border: '1px solid rgba(138,92,246,0.15)',
              color: '#8B82B0', fontSize: 13, fontWeight: 500,
              cursor: 'pointer', transition: 'all 0.15s',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.borderColor = 'rgba(138,92,246,0.3)'
              e.currentTarget.style.color = '#EEE8FF'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.borderColor = 'rgba(138,92,246,0.15)'
              e.currentTarget.style.color = '#8B82B0'
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
