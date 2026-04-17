'use client'

import { useParams, useRouter } from 'next/navigation'
import { useEffect } from 'react'
import { useVMStore } from '@/store/vmStore'
import { connectVM } from '@/lib/wsManager'
import InfraCanvas from '@/components/canvas/InfraCanvas'
import { ArrowLeft, AlertCircle, Loader2 } from 'lucide-react'

export default function VMCanvasPage() {
  const params = useParams()
  const router = useRouter()
  const code = params.code as string
  const { vms } = useVMStore()
  const vm = vms[code]

  // If VM not in store (e.g. direct URL access), try to connect
  useEffect(() => {
    if (!vm) {
      connectVM(code)
    }
  }, [code, vm])

  if (!vm) {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ background: '#070711' }}>
        <div className="flex flex-col items-center gap-4">
          <Loader2 size={32} className="animate-spin" style={{ color: '#6366f1' }} />
          <p style={{ color: '#64748b' }}>Connecting to {code}…</p>
        </div>
      </div>
    )
  }

  if (vm.status === 'error') {
    return (
      <div className="min-h-screen flex items-center justify-center" style={{ background: '#070711' }}>
        <div
          className="max-w-md w-full mx-4 p-6 rounded-2xl flex flex-col gap-4"
          style={{ background: '#0e0e1a', border: '1px solid #1e1e3a' }}
        >
          <div className="flex items-center gap-3">
            <div
              className="w-10 h-10 rounded-lg flex items-center justify-center"
              style={{ background: 'rgba(239, 68, 68, 0.1)', color: '#ef4444' }}
            >
              <AlertCircle size={20} />
            </div>
            <div>
              <p className="font-semibold" style={{ color: '#e2e8f0' }}>Connection Error</p>
              <p className="text-xs" style={{ color: '#64748b' }}>Could not connect to {code}</p>
            </div>
          </div>
          <p className="text-sm p-3 rounded-lg" style={{ background: '#13131f', color: '#94a3b8' }}>
            {vm.error}
          </p>
          <button
            onClick={() => router.push('/')}
            className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all"
            style={{ background: '#1a1a2e', color: '#94a3b8', border: '1px solid #1e1e3a' }}
            onMouseEnter={(e) => { e.currentTarget.style.color = '#e2e8f0' }}
            onMouseLeave={(e) => { e.currentTarget.style.color = '#94a3b8' }}
          >
            <ArrowLeft size={16} />
            Back to Dashboard
          </button>
        </div>
      </div>
    )
  }

  return <InfraCanvas vm={vm} onBack={() => router.push('/')} />
}
