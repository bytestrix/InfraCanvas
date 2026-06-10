'use client'

import { useEffect, useState } from 'react'
import { useVMStore } from '@/store/vmStore'
import { connectVM } from '@/lib/wsManager'
import InfraCanvas from '@/components/canvas/InfraCanvas'
import AgentOverview from '@/components/agent/AgentOverview'
import { LogoMark } from '@/components/Logo'
import { useTheme } from '@/hooks/useTheme'
import { AlertCircle } from 'lucide-react'

const LOCAL_KEY = 'local'

// All values are CSS variables — update automatically when data-theme changes
const T = {
  bg:       'var(--bg)',
  surface:  'var(--surface)',
  surface2: 'var(--surface-2)',
  line:     'var(--line)',
  line2:    'var(--line2)',
  line3:    'var(--line3)',
  ink:      'var(--ink)',
  ink2:     'var(--ink2)',
  ink3:     'var(--ink3)',
  ink4:     'var(--ink4)',
}
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
const IcSun = () => (
  <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
    <circle cx="8" cy="8" r="3"/>
    <path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41"/>
  </svg>
)
const IcMoon = () => (
  <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
    <path d="M13 9.5A6 6 0 0 1 6.5 3a6 6 0 1 0 6.5 6.5z"/>
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
  const { theme, toggle } = useTheme()

  const navItems: { id: View; label: string; Icon: () => JSX.Element }[] = [
    { id: 'overview', label: 'Overview', Icon: IcOverview },
    { id: 'canvas',   label: 'Canvas',   Icon: IcCanvas   },
  ]

  return (
    <aside style={{ borderRight:`1px solid ${T.line}`, display:'flex', flexDirection:'column', padding:'16px 10px 12px', background:T.bg }}>
      {/* Brand */}
      <div style={{ display:'flex', alignItems:'center', gap:10, padding:'0 6px 20px' }}>
        <LogoMark size={40} />
        <div>
          <div style={{ fontSize:14, fontWeight:600, letterSpacing:'-0.025em', color:T.ink }}>InfraCanvas</div>
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

      {/* Theme toggle */}
      <div style={{ padding:'0 2px 8px' }}>
        <button onClick={toggle} title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`} style={{
          display:'flex', alignItems:'center', gap:8, width:'100%',
          height:30, padding:'0 8px', borderRadius:7,
          fontSize:12, border:'none', cursor:'pointer',
          color:T.ink3, background:'transparent', fontFamily:SANS, transition:'all 0.1s',
        }}
          onMouseEnter={e=>{ (e.currentTarget as any).style.color=T.ink; (e.currentTarget as any).style.background=T.surface }}
          onMouseLeave={e=>{ (e.currentTarget as any).style.color=T.ink3; (e.currentTarget as any).style.background='transparent' }}
        >
          {theme === 'dark' ? <IcSun /> : <IcMoon />}
          <span>{theme === 'dark' ? 'Light mode' : 'Dark mode'}</span>
        </button>
      </div>

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
        {vm?.readOnly && (
          <div style={{
            margin:'4px 8px 0', padding:'3px 8px', borderRadius:6,
            fontSize:10.5, fontFamily:MONO, textAlign:'center',
            color:H.degraded, background:'rgba(245,158,11,0.08)',
            border:'1px solid rgba(245,158,11,0.2)',
          }}>
            read-only demo
          </div>
        )}
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
        <div className="animate-spin" style={{ width:38, height:38, borderRadius:'50%', border:'2.5px solid var(--spinner-track)', borderTopColor:'var(--spinner-tip)' }} />
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
