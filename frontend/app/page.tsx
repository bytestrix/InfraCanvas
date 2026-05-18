'use client'

import { useEffect, useState } from 'react'
import { useVMStore } from '@/store/vmStore'
import { connectVM } from '@/lib/wsManager'
import InfraCanvas from '@/components/canvas/InfraCanvas'
import AgentOverview from '@/components/agent/AgentOverview'
import { LogoMark } from '@/components/Logo'
import { AlertCircle } from 'lucide-react'

const LOCAL_KEY = 'local'

const T = { bg:'#0A0A0A', surface:'#111111', surface2:'#161616', line:'#1E1E1E', line2:'#2A2A2A', line3:'#383838', ink:'#FAFAFA', ink2:'#A1A1A1', ink3:'#6E6E6E', ink4:'#454545' }
const H = { healthy:'#22c55e', degraded:'#f59e0b', unhealthy:'#ef4444' }
const MONO = "var(--font-geist-mono,'Geist Mono','JetBrains Mono',ui-monospace,monospace)"
const SANS = "var(--font-geist,'Geist',ui-sans-serif,system-ui,sans-serif)"

type View = 'overview' | 'canvas'

const IcOverview = () => (
  <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4">
    <rect x="1" y="1" width="6" height="6" rx="1.2"/><rect x="9" y="1" width="6" height="6" rx="1.2"/>
    <rect x="1" y="9" width="6" height="6" rx="1.2"/><rect x="9" y="9" width="6" height="6" rx="1.2"/>
  </svg>
)
const IcCanvas = () => (
  <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4">
    <rect x="1" y="2" width="14" height="10" rx="1.5"/>
    <path d="M5 12v2M11 12v2M3 14h10"/>
    <path d="M5 7h6M8 5v4" strokeLinecap="round"/>
  </svg>
)

export default function App() {
  const { vms } = useVMStore()
  const vm = vms[LOCAL_KEY]
  const [view, setView] = useState<View>('overview')

  useEffect(() => {
    if (!vms[LOCAL_KEY]) connectVM(LOCAL_KEY)
  }, [vms])

  return (
    <div style={{ display:'grid', gridTemplateColumns:'200px 1fr', height:'100vh', background:T.bg, fontFamily:SANS, overflow:'hidden' }}>
      <Sidebar vm={vm} view={view} onViewChange={setView} />
      <main style={{ overflow:'hidden', minWidth:0, height:'100vh', display:'flex', flexDirection:'column' }}>
        <MainContent vm={vm} view={view} onSwitchToCanvas={() => setView('canvas')} />
      </main>
      <style>{`@keyframes pulse{0%,100%{opacity:1}50%{opacity:.4}}`}</style>
    </div>
  )
}

/* ── Sidebar ── */
function Sidebar({ vm, view, onViewChange }: { vm: any; view: View; onViewChange: (v: View) => void }) {
  const connected = vm?.status === 'connected'
  const hostname  = vm?.hostname ?? null

  const navItems: { id: View; label: string; Icon: () => JSX.Element }[] = [
    { id: 'overview', label: 'Overview', Icon: IcOverview },
    { id: 'canvas',   label: 'Canvas',   Icon: IcCanvas   },
  ]

  return (
    <aside style={{ borderRight:`1px solid ${T.line}`, display:'flex', flexDirection:'column', padding:'16px 10px 12px', background:T.bg }}>
      {/* Brand */}
      <div style={{ display:'flex', alignItems:'center', gap:9, padding:'0 6px 20px' }}>
        <LogoMark size={20} color={T.ink} />
        <div>
          <div style={{ fontSize:13.5, fontWeight:600, letterSpacing:'-0.025em', color:T.ink }}>InfraCanvas</div>
          <div style={{ fontSize:10, color:T.ink4, fontFamily:MONO }}>open source</div>
        </div>
      </div>

      {/* Nav */}
      <div style={{ display:'flex', flexDirection:'column', gap:1 }}>
        {navItems.map(({ id, label, Icon }) => {
          const active = view === id
          return (
            <button key={id} onClick={() => onViewChange(id)} style={{
              display:'flex', alignItems:'center', gap:9,
              height:30, padding:'0 8px', borderRadius:7,
              fontSize:13, border:'none', cursor:'pointer', width:'100%', textAlign:'left',
              color: active ? T.ink : T.ink2,
              background: active ? T.surface : 'transparent',
              fontFamily: SANS, transition:'all 0.1s',
            }}
              onMouseEnter={e=>{ if(!active){(e.currentTarget as any).style.color=T.ink;(e.currentTarget as any).style.background=T.surface} }}
              onMouseLeave={e=>{ if(!active){(e.currentTarget as any).style.color=T.ink2;(e.currentTarget as any).style.background='transparent'} }}
            >
              <Icon />
              <span>{label}</span>
            </button>
          )
        })}
      </div>

      {/* Spacer */}
      <div style={{ flex:1 }} />

      {/* Connection status */}
      <div style={{ borderTop:`1px solid ${T.line}`, paddingTop:10 }}>
        <div style={{ display:'flex', alignItems:'center', gap:9, padding:'6px 8px', borderRadius:7 }}>
          <div style={{
            width:26, height:26, borderRadius:'50%',
            background: connected ? 'rgba(34,197,94,0.1)' : T.surface,
            border: `1px solid ${connected ? 'rgba(34,197,94,0.2)' : T.line2}`,
            display:'grid', placeItems:'center', flexShrink:0,
          }}>
            <span style={{ width:8, height:8, borderRadius:'50%', background: connected ? H.healthy : T.ink4, ...(connected ? { animation:'pulse 2s infinite' } : {}) }} />
          </div>
          <div style={{ flex:1, minWidth:0 }}>
            <div style={{ fontSize:12.5, fontWeight:500, color:T.ink, overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap' }}>
              {hostname ?? 'local'}
            </div>
            <div style={{ fontSize:10.5, color: connected ? H.healthy : T.ink4, fontFamily:MONO }}>
              {connected ? 'connected' : vm?.status ?? 'connecting…'}
            </div>
          </div>
        </div>
      </div>
    </aside>
  )
}

/* ── Main content router ── */
function MainContent({ vm, view, onSwitchToCanvas }: { vm: any; view: View; onSwitchToCanvas: () => void }) {
  const isLoading = !vm || vm.status === 'connecting' || vm.status === 'paired'
  const isError   = vm?.status === 'error'

  if (isError) {
    return <ErrorContent message={vm.error ?? 'Connection failed'} />
  }

  if (view === 'canvas') {
    if (isLoading) return <LoadingContent />
    return (
      <div style={{ flex:1, position:'relative', height:'100%', overflow:'hidden' }}>
        <InfraCanvas vm={vm} />
      </div>
    )
  }

  // overview — loading state while connecting, full overview once graph arrives
  if (isLoading || !vm?.graph) return <LoadingContent />

  return (
    <AgentOverview
      graph={vm.graph}
      hostname={vm.hostname ?? null}
      vmCode={LOCAL_KEY}
      onSwitchToCanvas={onSwitchToCanvas}
    />
  )
}

/* ── Loading ── */
function LoadingContent() {
  return (
    <div style={{ flex:1, display:'flex', alignItems:'center', justifyContent:'center', background:T.bg }}>
      <div style={{ display:'flex', flexDirection:'column', alignItems:'center', gap:16 }}>
        <div className="animate-spin" style={{ width:38, height:38, borderRadius:'50%', border:'2.5px solid rgba(250,250,250,0.08)', borderTopColor:'#A1A1A1' }} />
        <p style={{ fontSize:13, color:T.ink3, fontFamily:MONO, margin:0 }}>Discovering infrastructure…</p>
      </div>
    </div>
  )
}

/* ── Error ── */
function ErrorContent({ message }: { message: string }) {
  return (
    <div style={{ flex:1, display:'flex', alignItems:'center', justifyContent:'center', background:T.bg, padding:16 }}>
      <div style={{ maxWidth:460, width:'100%', background:T.surface, border:`1px solid ${T.line}`, borderRadius:16, padding:'28px 28px 24px', display:'flex', flexDirection:'column', gap:18 }}>
        <div style={{ display:'flex', alignItems:'center', gap:14 }}>
          <div style={{ width:42, height:42, borderRadius:11, background:'rgba(248,113,113,0.1)', border:'1px solid rgba(248,113,113,0.2)', display:'flex', alignItems:'center', justifyContent:'center', color:'#F87171', flexShrink:0 }}>
            <AlertCircle size={20} />
          </div>
          <div>
            <p style={{ fontSize:15, fontWeight:600, color:T.ink, margin:0 }}>Connection failed</p>
            <p style={{ fontSize:12, color:T.ink3, margin:'3px 0 0' }}>The dashboard couldn&apos;t reach the local agent</p>
          </div>
        </div>
        <p style={{ fontSize:12, color:T.ink2, margin:0, padding:'12px 14px', borderRadius:9, background:T.bg, border:`1px solid ${T.line}`, fontFamily:MONO, lineHeight:1.6 }}>
          {message}
        </p>
        <p style={{ fontSize:12, color:T.ink3, margin:0, lineHeight:1.6 }}>
          Check that the InfraCanvas service is running:<br />
          <code style={{ color:T.ink2, fontFamily:MONO }}>sudo systemctl status infracanvas</code>
        </p>
      </div>
    </div>
  )
}

