'use client'

import { useState, useEffect } from 'react'
import { useVMStore } from '@/store/vmStore'
import { connectVM, disconnectVM } from '@/lib/wsManager'
import ConnectModal from '@/components/ConnectModal'
import VMCard from '@/components/VMCard'
import { Plus, Hexagon, Activity, Wifi } from 'lucide-react'

export default function Dashboard() {
  const { vms } = useVMStore()
  const [showConnect, setShowConnect] = useState(false)

  const vmList = Object.values(vms)
  const connectedCount = vmList.filter((v) => v.status === 'connected').length

  function handleConnect(code: string) {
    connectVM(code)
    setShowConnect(false)
  }

  function handleDisconnect(code: string) {
    disconnectVM(code)
  }

  return (
    <div className="min-h-screen" style={{ background: '#070711' }}>
      {/* ─── Header ─────────────────────────────────────────────── */}
      <header
        className="sticky top-0 z-40 border-b"
        style={{
          background: 'rgba(7, 7, 17, 0.9)',
          backdropFilter: 'blur(12px)',
          borderColor: '#1e1e3a',
        }}
      >
        <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
          {/* Logo */}
          <div className="flex items-center gap-3">
            <div
              className="w-9 h-9 rounded-lg flex items-center justify-center text-lg"
              style={{ background: 'linear-gradient(135deg, #6366f1, #8b5cf6)' }}
            >
              ⬡
            </div>
            <div>
              <h1 className="font-semibold text-base leading-none" style={{ color: '#e2e8f0' }}>
                InfraCanvas
              </h1>
              <p className="text-xs mt-0.5" style={{ color: '#475569' }}>
                Infrastructure at a glance
              </p>
            </div>
          </div>

          {/* Right side */}
          <div className="flex items-center gap-4">
            {connectedCount > 0 && (
              <div
                className="flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-medium"
                style={{
                  background: 'rgba(16, 185, 129, 0.1)',
                  border: '1px solid rgba(16, 185, 129, 0.2)',
                  color: '#10b981',
                }}
              >
                <span
                  className="w-1.5 h-1.5 rounded-full status-dot-pulse"
                  style={{ background: '#10b981' }}
                />
                {connectedCount} VM{connectedCount !== 1 ? 's' : ''} connected
              </div>
            )}
            <button
              onClick={() => setShowConnect(true)}
              className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all duration-150"
              style={{
                background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
                color: '#fff',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.opacity = '0.9'
                e.currentTarget.style.transform = 'translateY(-1px)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.opacity = '1'
                e.currentTarget.style.transform = 'translateY(0)'
              }}
            >
              <Plus size={16} />
              Connect VM
            </button>
          </div>
        </div>
      </header>

      {/* ─── Main Content ───────────────────────────────────────── */}
      <main className="max-w-7xl mx-auto px-6 py-10">
        {vmList.length === 0 ? (
          /* Empty state */
          <div className="flex flex-col items-center justify-center py-32 animate-slide-up">
            <div
              className="w-24 h-24 rounded-2xl flex items-center justify-center text-4xl mb-6"
              style={{
                background: 'rgba(99, 102, 241, 0.08)',
                border: '1px solid rgba(99, 102, 241, 0.15)',
              }}
            >
              ⬡
            </div>
            <h2 className="text-2xl font-semibold mb-2" style={{ color: '#e2e8f0' }}>
              No VMs connected
            </h2>
            <p className="text-sm mb-8 text-center max-w-md" style={{ color: '#64748b' }}>
              Connect your first VM to start visualizing your infrastructure. Run the InfraCanvas
              agent on any VM or Kubernetes cluster.
            </p>
            <button
              onClick={() => setShowConnect(true)}
              className="flex items-center gap-2 px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-150"
              style={{
                background: 'linear-gradient(135deg, #6366f1, #8b5cf6)',
                color: '#fff',
                boxShadow: '0 0 24px rgba(99, 102, 241, 0.25)',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.transform = 'translateY(-2px)'
                e.currentTarget.style.boxShadow = '0 0 32px rgba(99, 102, 241, 0.4)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.transform = 'translateY(0)'
                e.currentTarget.style.boxShadow = '0 0 24px rgba(99, 102, 241, 0.25)'
              }}
            >
              <Plus size={18} />
              Connect VM
            </button>

            {/* Feature hints */}
            <div className="grid grid-cols-3 gap-4 mt-16 w-full max-w-2xl">
              {[
                {
                  icon: <Activity size={18} />,
                  title: 'Real-time monitoring',
                  desc: 'Live CPU, memory, and health metrics',
                },
                {
                  icon: <span className="text-base">⬡</span>,
                  title: 'Visual canvas',
                  desc: 'Interactive graph of your infrastructure',
                },
                {
                  icon: <Wifi size={18} />,
                  title: 'WebSocket streaming',
                  desc: 'Instant updates via persistent connection',
                },
              ].map((f) => (
                <div
                  key={f.title}
                  className="p-4 rounded-xl text-center"
                  style={{
                    background: '#0e0e1a',
                    border: '1px solid #1e1e3a',
                  }}
                >
                  <div
                    className="w-9 h-9 rounded-lg flex items-center justify-center mx-auto mb-3"
                    style={{
                      background: 'rgba(99, 102, 241, 0.1)',
                      color: '#6366f1',
                    }}
                  >
                    {f.icon}
                  </div>
                  <p className="text-xs font-semibold mb-1" style={{ color: '#e2e8f0' }}>
                    {f.title}
                  </p>
                  <p className="text-xs" style={{ color: '#475569' }}>
                    {f.desc}
                  </p>
                </div>
              ))}
            </div>
          </div>
        ) : (
          /* VM Grid */
          <div>
            <div className="flex items-center justify-between mb-6">
              <div>
                <h2 className="text-lg font-semibold" style={{ color: '#e2e8f0' }}>
                  Connected VMs
                </h2>
                <p className="text-xs mt-0.5" style={{ color: '#64748b' }}>
                  {vmList.length} VM{vmList.length !== 1 ? 's' : ''} registered
                </p>
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {vmList.map((vm) => (
                <VMCard key={vm.code} vm={vm} onDisconnect={() => handleDisconnect(vm.code)} />
              ))}
              {/* Add more card */}
              <button
                onClick={() => setShowConnect(true)}
                className="rounded-xl border-2 border-dashed flex flex-col items-center justify-center gap-3 p-8 transition-all duration-150 min-h-[220px]"
                style={{
                  borderColor: '#1e1e3a',
                  color: '#475569',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.borderColor = '#6366f1'
                  e.currentTarget.style.color = '#6366f1'
                  e.currentTarget.style.background = 'rgba(99, 102, 241, 0.04)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.borderColor = '#1e1e3a'
                  e.currentTarget.style.color = '#475569'
                  e.currentTarget.style.background = 'transparent'
                }}
              >
                <div
                  className="w-10 h-10 rounded-lg flex items-center justify-center"
                  style={{ background: 'rgba(71, 85, 105, 0.1)' }}
                >
                  <Plus size={20} />
                </div>
                <p className="text-sm font-medium">Add VM</p>
              </button>
            </div>
          </div>
        )}
      </main>

      {/* ─── Connect Modal ──────────────────────────────────────── */}
      {showConnect && (
        <ConnectModal onConnect={handleConnect} onClose={() => setShowConnect(false)} />
      )}
    </div>
  )
}
