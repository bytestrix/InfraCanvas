'use client'

import { useState } from 'react'
import { useVMStore } from '@/store/vmStore'
import { connectVM, disconnectVM } from '@/lib/wsManager'
import ConnectModal from '@/components/ConnectModal'
import VMCard from '@/components/VMCard'
import { Plus, Activity, Network, Terminal } from 'lucide-react'

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
    <div className="min-h-screen" style={{ background: '#111110' }}>

      {/* ── Header ────────────────────────────────────────────────── */}
      <header style={{
        position: 'sticky', top: 0, zIndex: 40,
        borderBottom: '1px solid rgba(255,255,255,0.07)',
        background: 'rgba(17,17,16,0.92)',
        backdropFilter: 'blur(16px)',
        WebkitBackdropFilter: 'blur(16px)',
      }}>
        <div style={{ maxWidth: 1200, margin: '0 auto', padding: '0 24px', height: 60, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>

          {/* Logo */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{
              width: 34, height: 34, borderRadius: 10,
              background: '#DA7756',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 16, color: '#fff', fontWeight: 700, flexShrink: 0,
              boxShadow: '0 2px 12px rgba(218,119,86,0.35)',
            }}>
              ⬡
            </div>
            <div>
              <p style={{ fontSize: 14, fontWeight: 600, color: '#F0EDE7', lineHeight: 1, margin: 0 }}>
                InfraCanvas
              </p>
              <p style={{ fontSize: 11, color: '#625850', margin: '2px 0 0', lineHeight: 1 }}>
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
                background: 'rgba(77,184,138,0.1)',
                border: '1px solid rgba(77,184,138,0.2)',
                color: '#4DB88A', fontSize: 12, fontWeight: 500,
              }}>
                <span className="status-dot-pulse" style={{ display: 'block', width: 6, height: 6, borderRadius: '50%', background: '#4DB88A', flexShrink: 0 }} />
                {connectedCount} live
              </div>
            )}
            <button
              onClick={() => setShowConnect(true)}
              style={{
                display: 'flex', alignItems: 'center', gap: 7,
                padding: '8px 16px', borderRadius: 9,
                background: '#DA7756', color: '#fff',
                border: 'none', fontSize: 13, fontWeight: 500,
                cursor: 'pointer', transition: 'background 0.15s, transform 0.15s, box-shadow 0.15s',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = '#E88A68'
                e.currentTarget.style.transform = 'translateY(-1px)'
                e.currentTarget.style.boxShadow = '0 4px 16px rgba(218,119,86,0.35)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = '#DA7756'
                e.currentTarget.style.transform = 'none'
                e.currentTarget.style.boxShadow = 'none'
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
              background: 'rgba(218,119,86,0.08)',
              border: '1px solid rgba(218,119,86,0.14)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 36, marginBottom: 28,
            }}>
              ⬡
            </div>

            <h2 style={{ fontSize: 26, fontWeight: 600, color: '#F0EDE7', margin: '0 0 10px', letterSpacing: '-0.3px' }}>
              No VMs connected
            </h2>
            <p style={{ fontSize: 14, color: '#A09890', margin: '0 0 36px', textAlign: 'center', maxWidth: 380, lineHeight: 1.6 }}>
              Connect a VM to start visualizing your infrastructure. Run the agent on any Linux server — no inbound ports required.
            </p>

            <button
              onClick={() => setShowConnect(true)}
              style={{
                display: 'flex', alignItems: 'center', gap: 8,
                padding: '12px 24px', borderRadius: 10,
                background: '#DA7756', color: '#fff',
                border: 'none', fontSize: 14, fontWeight: 500,
                cursor: 'pointer', transition: 'all 0.15s',
                boxShadow: '0 4px 20px rgba(218,119,86,0.25)',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = '#E88A68'
                e.currentTarget.style.transform = 'translateY(-2px)'
                e.currentTarget.style.boxShadow = '0 8px 28px rgba(218,119,86,0.35)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = '#DA7756'
                e.currentTarget.style.transform = 'none'
                e.currentTarget.style.boxShadow = '0 4px 20px rgba(218,119,86,0.25)'
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
                  background: '#191817',
                  border: '1px solid rgba(255,255,255,0.07)',
                  borderRadius: 12, textAlign: 'center',
                }}>
                  <div style={{
                    width: 36, height: 36, borderRadius: 9,
                    background: 'rgba(218,119,86,0.1)',
                    color: '#DA7756',
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    margin: '0 auto 12px',
                  }}>{f.icon}</div>
                  <p style={{ fontSize: 12, fontWeight: 600, color: '#F0EDE7', margin: '0 0 4px' }}>{f.title}</p>
                  <p style={{ fontSize: 11, color: '#625850', margin: 0, lineHeight: 1.5 }}>{f.desc}</p>
                </div>
              ))}
            </div>
          </div>

        ) : (
          /* ── VM Grid ── */
          <div>
            <div style={{ marginBottom: 24 }}>
              <h2 style={{ fontSize: 18, fontWeight: 600, color: '#F0EDE7', margin: '0 0 4px', letterSpacing: '-0.2px' }}>
                Connected VMs
              </h2>
              <p style={{ fontSize: 12, color: '#625850', margin: 0 }}>
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
                  border: '1.5px dashed rgba(255,255,255,0.1)',
                  background: 'transparent',
                  display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
                  gap: 10, cursor: 'pointer', color: '#625850',
                  transition: 'border-color 0.15s, color 0.15s, background 0.15s',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.borderColor = 'rgba(218,119,86,0.35)'
                  e.currentTarget.style.color = '#DA7756'
                  e.currentTarget.style.background = 'rgba(218,119,86,0.04)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.borderColor = 'rgba(255,255,255,0.1)'
                  e.currentTarget.style.color = '#625850'
                  e.currentTarget.style.background = 'transparent'
                }}
              >
                <div style={{
                  width: 38, height: 38, borderRadius: 9,
                  background: 'rgba(255,255,255,0.04)',
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
