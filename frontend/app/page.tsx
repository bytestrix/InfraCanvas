'use client'

import { useState } from 'react'
import { useVMStore } from '@/store/vmStore'
import { connectVM, disconnectVM } from '@/lib/wsManager'
import ConnectModal from '@/components/ConnectModal'
import VMCard from '@/components/VMCard'
import { Plus, Activity, Network } from 'lucide-react'

export default function Dashboard() {
  const { vms } = useVMStore()
  const [showConnect, setShowConnect] = useState(false)

  const vmList = Object.values(vms)
  const connectedCount = vmList.filter((v) => v.status === 'connected').length

  function handleConnect(code: string) {
    connectVM(code)
    setShowConnect(false)
  }

  return (
    <div className="min-h-screen" style={{ background: '#08080E' }}>

      {/* ── Header ────────────────────────────────────────────────── */}
      <header style={{
        position: 'sticky', top: 0, zIndex: 40,
        borderBottom: '1px solid rgba(138,92,246,0.12)',
        background: 'rgba(8,8,14,0.92)',
        backdropFilter: 'blur(16px)',
        WebkitBackdropFilter: 'blur(16px)',
      }}>
        <div style={{ maxWidth: 1200, margin: '0 auto', padding: '0 24px', height: 60, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>

          {/* Logo */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{
              width: 34, height: 34, borderRadius: 10,
              background: 'linear-gradient(135deg, #C026D3, #7C3AED)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 16, color: '#fff', fontWeight: 700, flexShrink: 0,
              boxShadow: '0 2px 14px rgba(192,38,211,0.4)',
            }}>
              ⬡
            </div>
            <div>
              <p style={{ fontSize: 14, fontWeight: 600, color: '#EEE8FF', lineHeight: 1, margin: 0 }}>
                InfraCanvas
              </p>
              <p style={{ fontSize: 11, color: '#52496E', margin: '2px 0 0', lineHeight: 1 }}>
                Infrastructure at a glance
              </p>
            </div>
          </div>

          {/* Right */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            {connectedCount > 0 && (
              <div style={{
                display: 'flex', alignItems: 'center', gap: 7,
                padding: '5px 12px', borderRadius: 20,
                background: 'rgba(74,222,128,0.08)',
                border: '1px solid rgba(74,222,128,0.18)',
                color: '#4ADE80', fontSize: 12, fontWeight: 500,
              }}>
                <span className="status-dot-pulse" style={{ display: 'block', width: 6, height: 6, borderRadius: '50%', background: '#4ADE80', flexShrink: 0 }} />
                {connectedCount} live
              </div>
            )}
            <button
              onClick={() => setShowConnect(true)}
              style={{
                display: 'flex', alignItems: 'center', gap: 7,
                padding: '8px 16px', borderRadius: 9,
                background: 'linear-gradient(135deg, #C026D3, #7C3AED)',
                color: '#fff', border: 'none', fontSize: 13, fontWeight: 500,
                cursor: 'pointer', transition: 'opacity 0.15s, transform 0.15s, box-shadow 0.15s',
                boxShadow: '0 2px 12px rgba(192,38,211,0.3)',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.opacity = '0.88'
                e.currentTarget.style.transform = 'translateY(-1px)'
                e.currentTarget.style.boxShadow = '0 6px 20px rgba(192,38,211,0.45)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.opacity = '1'
                e.currentTarget.style.transform = 'none'
                e.currentTarget.style.boxShadow = '0 2px 12px rgba(192,38,211,0.3)'
              }}
            >
              <Plus size={15} />
              Connect VM
            </button>
          </div>
        </div>
      </header>

      {/* ── Main ──────────────────────────────────────────────────── */}
      <main style={{ maxWidth: 1200, margin: '0 auto', padding: '40px 24px' }}>

        {vmList.length === 0 ? (
          /* ── Empty state ── */
          <div className="animate-slide-up" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', paddingTop: 80, paddingBottom: 80 }}>

            {/* Icon */}
            <div style={{
              width: 80, height: 80, borderRadius: 22,
              background: 'rgba(192,38,211,0.08)',
              border: '1px solid rgba(192,38,211,0.18)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 36, marginBottom: 28,
              boxShadow: '0 0 40px rgba(192,38,211,0.12)',
            }}>
              ⬡
            </div>

            <h2 style={{ fontSize: 26, fontWeight: 600, color: '#EEE8FF', margin: '0 0 10px', letterSpacing: '-0.3px' }}>
              No VMs connected
            </h2>
            <p style={{ fontSize: 14, color: '#8B82B0', margin: '0 0 36px', textAlign: 'center', maxWidth: 380, lineHeight: 1.6 }}>
              Connect a VM to start visualizing your infrastructure. Run the agent on any Linux server — no inbound ports required.
            </p>

            <button
              onClick={() => setShowConnect(true)}
              style={{
                display: 'flex', alignItems: 'center', gap: 8,
                padding: '12px 24px', borderRadius: 10,
                background: 'linear-gradient(135deg, #C026D3, #7C3AED)',
                color: '#fff', border: 'none', fontSize: 14, fontWeight: 500,
                cursor: 'pointer', transition: 'opacity 0.15s, transform 0.15s, box-shadow 0.15s',
                boxShadow: '0 4px 24px rgba(192,38,211,0.35)',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.opacity = '0.88'
                e.currentTarget.style.transform = 'translateY(-2px)'
                e.currentTarget.style.boxShadow = '0 8px 32px rgba(192,38,211,0.45)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.opacity = '1'
                e.currentTarget.style.transform = 'none'
                e.currentTarget.style.boxShadow = '0 4px 24px rgba(192,38,211,0.35)'
              }}
            >
              <Plus size={17} />
              Connect your first VM
            </button>

            {/* Feature hints */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 14, marginTop: 60, width: '100%', maxWidth: 560 }}>
              {[
                { icon: <Activity size={16} />, title: 'Live metrics', desc: 'CPU, memory, and health in real-time' },
                { icon: <span style={{ fontSize: 16 }}>⬡</span>, title: 'Visual canvas', desc: 'Interactive graph of every resource' },
                { icon: <Network size={16} />, title: 'Outbound only', desc: 'No inbound ports or VPN needed' },
              ].map((f) => (
                <div key={f.title} style={{
                  padding: '18px 16px',
                  background: '#0E0E1C',
                  border: '1px solid rgba(138,92,246,0.12)',
                  borderRadius: 12, textAlign: 'center',
                }}>
                  <div style={{
                    width: 36, height: 36, borderRadius: 9,
                    background: 'rgba(192,38,211,0.1)',
                    color: '#C026D3',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    margin: '0 auto 12px',
                  }}>{f.icon}</div>
                  <p style={{ fontSize: 12, fontWeight: 600, color: '#EEE8FF', margin: '0 0 4px' }}>{f.title}</p>
                  <p style={{ fontSize: 11, color: '#52496E', margin: 0, lineHeight: 1.5 }}>{f.desc}</p>
                </div>
              ))}
            </div>
          </div>

        ) : (
          /* ── VM Grid ── */
          <div>
            <div style={{ marginBottom: 24 }}>
              <h2 style={{ fontSize: 18, fontWeight: 600, color: '#EEE8FF', margin: '0 0 4px', letterSpacing: '-0.2px' }}>
                Connected VMs
              </h2>
              <p style={{ fontSize: 12, color: '#52496E', margin: 0 }}>
                {vmList.length} VM{vmList.length !== 1 ? 's' : ''} registered
              </p>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: 14 }}>
              {vmList.map((vm) => (
                <VMCard key={vm.code} vm={vm} onDisconnect={() => disconnectVM(vm.code)} />
              ))}

              {/* Add more */}
              <button
                onClick={() => setShowConnect(true)}
                style={{
                  minHeight: 200, borderRadius: 14,
                  border: '1.5px dashed rgba(138,92,246,0.18)',
                  background: 'transparent',
                  display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
                  gap: 10, cursor: 'pointer', color: '#52496E',
                  transition: 'border-color 0.15s, color 0.15s, background 0.15s',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.borderColor = 'rgba(192,38,211,0.4)'
                  e.currentTarget.style.color = '#C026D3'
                  e.currentTarget.style.background = 'rgba(192,38,211,0.05)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.borderColor = 'rgba(138,92,246,0.18)'
                  e.currentTarget.style.color = '#52496E'
                  e.currentTarget.style.background = 'transparent'
                }}
              >
                <div style={{
                  width: 38, height: 38, borderRadius: 9,
                  background: 'rgba(138,92,246,0.08)',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                  <Plus size={18} />
                </div>
                <span style={{ fontSize: 13, fontWeight: 500 }}>Add VM</span>
              </button>
            </div>
          </div>
        )}
      </main>

      {showConnect && (
        <ConnectModal onConnect={handleConnect} onClose={() => setShowConnect(false)} />
      )}
    </div>
  )
}
