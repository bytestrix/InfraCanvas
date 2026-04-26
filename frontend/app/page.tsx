'use client'

import { useEffect } from 'react'
import { useVMStore } from '@/store/vmStore'
import { connectVM } from '@/lib/wsManager'
import InfraCanvas from '@/components/canvas/InfraCanvas'
import { AlertCircle } from 'lucide-react'

const LOCAL_KEY = 'local'

export default function Dashboard() {
  const { vms } = useVMStore()
  const vm = vms[LOCAL_KEY]

  useEffect(() => {
    // Auto-connect to the in-process relay (same origin). The server is in
    // local-mode and auto-pairs the first browser to the only agent.
    if (!vms[LOCAL_KEY]) connectVM(LOCAL_KEY)
  }, [vms])

  if (!vm || vm.status === 'connecting' || vm.status === 'paired') {
    return <LoadingScreen />
  }

  if (vm.status === 'error') {
    return <ErrorScreen message={vm.error ?? 'Connection failed'} />
  }

  return <InfraCanvas vm={vm} />
}

function LoadingScreen() {
  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#08080E' }}>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16 }}>
        <div
          className="animate-spin"
          style={{
            width: 38, height: 38, borderRadius: '50%',
            border: '2.5px solid rgba(192,38,211,0.2)',
            borderTopColor: '#C026D3',
          }}
        />
        <p style={{ fontSize: 13, color: '#8B82B0' }}>Discovering infrastructure…</p>
      </div>
    </div>
  )
}

function ErrorScreen({ message }: { message: string }) {
  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#08080E', padding: 16 }}>
      <div style={{
        maxWidth: 460, width: '100%',
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
            <p style={{ fontSize: 12, color: '#52496E', margin: '3px 0 0' }}>The dashboard couldn&apos;t reach the local agent</p>
          </div>
        </div>
        <p style={{
          fontSize: 12, color: '#8B82B0', margin: 0,
          padding: '12px 14px', borderRadius: 9,
          background: '#08080E', border: '1px solid rgba(138,92,246,0.1)',
          fontFamily: 'JetBrains Mono, monospace', lineHeight: 1.6,
        }}>
          {message}
        </p>
        <p style={{ fontSize: 12, color: '#52496E', margin: 0, lineHeight: 1.6 }}>
          Check that the InfraCanvas service is running:
          <br /><code style={{ color: '#C026D3' }}>sudo systemctl status infracanvas</code>
        </p>
      </div>
    </div>
  )
}
