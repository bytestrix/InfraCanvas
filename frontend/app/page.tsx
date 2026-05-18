'use client'

import { useEffect, useState, useMemo } from 'react'
import { useVMStore } from '@/store/vmStore'
import { connectVM } from '@/lib/wsManager'
import InfraCanvas from '@/components/canvas/InfraCanvas'
import { LogoMark } from '@/components/Logo'
import { AlertCircle, AlertTriangle, CheckCircle2 } from 'lucide-react'

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
        <MainContent vm={vm} view={view} />
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
function MainContent({ vm, view }: { vm: any; view: View }) {
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

  // overview — show even while connecting so UI isn't blank
  return <OverviewContent vm={vm} isLoading={isLoading} />
}

/* ── Overview ── */
function OverviewContent({ vm, isLoading }: { vm: any; isLoading: boolean }) {
  const graph    = vm?.graph ?? null
  const hostname = vm?.hostname ?? 'local'

  const stats = useMemo(() => {
    if (!graph) return { nodes:0, pods:0, degraded:0, unhealthy:0 }
    return {
      nodes:     graph.stats?.totalNodes ?? graph.nodes.length,
      pods:      graph.nodes.filter((n: any) => n.type === 'pod' && n.health === 'healthy').length,
      degraded:  graph.nodes.filter((n: any) => n.health === 'degraded').length,
      unhealthy: graph.nodes.filter((n: any) => n.health === 'unhealthy').length,
    }
  }, [graph])

  const hostMeta = graph?.nodes?.find((n: any) => n.type === 'host')?.metadata ?? {}
  const cpuPct   = typeof hostMeta.cpu_percent    === 'number' ? Math.round(hostMeta.cpu_percent)    : null
  const memPct   = typeof hostMeta.memory_percent === 'number' ? Math.round(hostMeta.memory_percent) : null
  const alertCount = graph ? (stats.degraded + stats.unhealthy) : null
  const displayIp = (() => {
    const mi = hostMeta?.ip ?? hostMeta?.ip_address ?? hostMeta?.ipAddress
    if (mi && mi !== '127.0.0.1' && !String(mi).startsWith('127.')) return String(mi)
    return null
  })()

  const connected = vm?.status === 'connected'

  const metrics = [
    { k:'Status',    v: connected ? 'online' : (vm?.status ?? '—'), c: connected ? H.healthy : T.ink4 },
    { k:'Resources', v: graph ? String(stats.nodes)    : '—', c: T.ink },
    { k:'Pods',      v: graph ? String(stats.pods)     : '—', c: T.ink },
    { k:'Degraded',  v: graph ? String(stats.degraded) : '—', c: stats.degraded  > 0 ? H.degraded  : T.ink4 },
    { k:'Unhealthy', v: graph ? String(stats.unhealthy): '—', c: stats.unhealthy > 0 ? H.unhealthy : T.ink4 },
  ]

  return (
    <div style={{ height:'100%', overflowY:'auto', padding:'28px 28px 48px', fontSize:13 }}>
      {/* Topbar */}
      <div style={{ display:'flex', alignItems:'center', gap:10, marginBottom:28 }}>
        <h1 style={{ margin:0, fontSize:14, fontWeight:500, color:T.ink, letterSpacing:'-0.015em' }}>Overview</h1>
        {connected && (
          <span style={{ display:'inline-flex', alignItems:'center', gap:5, fontFamily:MONO, fontSize:10.5, color:T.ink3 }}>
            <span style={{ width:5, height:5, borderRadius:'50%', background:H.healthy, animation:'pulse 1.6s ease-in-out infinite' }} />
            live
          </span>
        )}
      </div>

      {/* Metric tiles */}
      <div style={{ display:'grid', gridTemplateColumns:'repeat(auto-fill,minmax(130px,1fr))', gap:1, background:T.line, border:`1px solid ${T.line}`, borderRadius:10, overflow:'hidden', marginBottom:32 }}>
        {metrics.map(m => (
          <div key={m.k} style={{ padding:'20px 22px', background:T.bg }}>
            <div style={{ fontSize:10.5, color:T.ink3, textTransform:'uppercase', letterSpacing:'0.06em', marginBottom:6 }}>{m.k}</div>
            <div style={{ fontSize:26, fontWeight:500, letterSpacing:'-0.03em', lineHeight:1, color:m.c, fontFamily:MONO }}>{m.v}</div>
          </div>
        ))}
      </div>

      {/* Machine section */}
      <div style={{ display:'flex', alignItems:'center', gap:8, marginBottom:12 }}>
        <span style={{ fontSize:11, color:T.ink3, textTransform:'uppercase', letterSpacing:'0.08em', fontWeight:500 }}>Machine</span>
        <span style={{ flex:1, height:1, background:T.line }} />
      </div>

      {isLoading ? (
        <div style={{ display:'flex', justifyContent:'center', padding:48 }}>
          <div className="animate-spin" style={{ width:22, height:22, borderRadius:'50%', border:`1.5px solid ${T.line2}`, borderTopColor:T.ink }} />
        </div>
      ) : (
        <div style={{ maxWidth:400 }}>
          {/* Machine card */}
          <div style={{ background:T.surface, border:`1px solid ${T.line}`, borderRadius:10, padding:'16px 18px', position:'relative' }}>
            <div style={{ position:'absolute', top:0, left:0, right:0, height:2, borderRadius:'10px 10px 0 0', background: connected ? H.healthy : T.line3 }} />

            {/* Header */}
            <div style={{ display:'flex', alignItems:'center', gap:10, marginBottom:12 }}>
              <div style={{ flex:1, minWidth:0 }}>
                <div style={{ fontSize:14, fontWeight:600, color:T.ink, overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap', letterSpacing:'-0.015em' }}>
                  {hostname}
                </div>
                <div style={{ display:'flex', gap:8, flexWrap:'wrap', marginTop:2 }}>
                  {hostMeta.os    && <span style={{ fontSize:11, color:T.ink3, fontFamily:MONO }}>{hostMeta.os}</span>}
                  {hostMeta.arch  && <span style={{ fontSize:11, color:T.ink3, fontFamily:MONO }}>{hostMeta.arch}</span>}
                  {displayIp      && <span style={{ fontSize:11, color:T.ink2, fontFamily:MONO }}>{displayIp}</span>}
                </div>
              </div>
              <span style={{
                display:'inline-flex', alignItems:'center', gap:5,
                fontSize:10.5, fontFamily:MONO, padding:'2px 8px', borderRadius:999,
                background: connected ? 'rgba(34,197,94,0.1)' : T.bg,
                border: `1px solid ${connected ? 'rgba(34,197,94,0.2)' : T.line}`,
                color: connected ? H.healthy : T.ink4,
              }}>
                <span style={{ width:4, height:4, borderRadius:'50%', background: connected ? H.healthy : T.ink4, ...(connected?{animation:'pulse 2s infinite'}:{}) }} />
                {connected ? 'online' : 'offline'}
              </span>
            </div>

            {/* CPU / MEM bars */}
            {(cpuPct !== null || memPct !== null) && (
              <div style={{ display:'flex', flexDirection:'column', gap:6, marginBottom:12 }}>
                {cpuPct !== null && <MiniBar label="CPU" pct={cpuPct} />}
                {memPct !== null && <MiniBar label="MEM" pct={memPct} />}
              </div>
            )}

            {/* Footer stats */}
            {graph && (
              <div style={{ display:'flex', gap:14, fontSize:11, fontFamily:MONO, color:T.ink4 }}>
                <span>{stats.nodes} nodes</span>
                <span>{stats.pods} pods</span>
                {alertCount !== null && alertCount > 0
                  ? <span style={{ color:H.degraded, marginLeft:'auto', display:'flex', alignItems:'center', gap:4 }}><AlertTriangle size={11} />{alertCount} alerts</span>
                  : <span style={{ color:H.healthy, marginLeft:'auto', display:'flex', alignItems:'center', gap:4 }}><CheckCircle2 size={11} />healthy</span>
                }
              </div>
            )}
            {!graph && connected && (
              <div style={{ fontSize:11, color:T.ink4, fontFamily:MONO }}>Waiting for graph data…</div>
            )}
          </div>

          {/* Unhealthy nodes list */}
          {graph && alertCount !== null && alertCount > 0 && (
            <div style={{ marginTop:28 }}>
              <div style={{ display:'flex', alignItems:'center', gap:8, marginBottom:12 }}>
                <span style={{ fontSize:11, color:T.ink3, textTransform:'uppercase', letterSpacing:'0.08em', fontWeight:500 }}>Alerts</span>
                <span style={{ fontFamily:MONO, fontSize:11, padding:'1px 7px', borderRadius:999, background:'rgba(239,68,68,0.1)', border:'1px solid rgba(239,68,68,0.25)', color:'#ef4444' }}>{alertCount}</span>
                <span style={{ flex:1, height:1, background:T.line }} />
              </div>
              <div style={{ border:`1px solid ${T.line}`, borderRadius:9, overflow:'hidden' }}>
                {graph.nodes
                  .filter((n: any) => (n.health === 'degraded' || n.health === 'unhealthy') && n.type !== 'host')
                  .map((node: any, i: number, arr: any[]) => {
                    const color = node.health === 'unhealthy' ? H.unhealthy : H.degraded
                    return (
                      <div key={node.id} style={{ padding:'11px 16px', display:'flex', gap:12, alignItems:'center', borderBottom:i<arr.length-1?`1px solid ${T.line}`:undefined, transition:'background 0.1s' }}
                        onMouseEnter={e=>(e.currentTarget as any).style.background=T.surface}
                        onMouseLeave={e=>(e.currentTarget as any).style.background='transparent'}
                      >
                        <span style={{ width:6, height:6, borderRadius:'50%', background:color, flexShrink:0, ...(node.health==='unhealthy'?{animation:'pulse 1.6s ease-in-out infinite'}:{}) }} />
                        <div style={{ flex:1, minWidth:0 }}>
                          <div style={{ fontSize:12.5, fontWeight:500, color:T.ink, overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap' }}>{node.label}</div>
                          <div style={{ fontSize:11, color:T.ink3, marginTop:2, display:'flex', gap:10, fontFamily:MONO }}>
                            <span>{node.type}</span>
                            {node.metadata?.namespace && <span>ns: {node.metadata.namespace}</span>}
                          </div>
                        </div>
                        <span style={{ height:20, padding:'0 7px', borderRadius:999, border:`1px solid ${color}40`, background:`${color}12`, color, fontSize:10, fontFamily:MONO, display:'flex', alignItems:'center' }}>
                          {node.health}
                        </span>
                      </div>
                    )
                  })}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
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

/* ── Mini resource bar ── */
function MiniBar({ label, pct }: { label: string; pct: number }) {
  const color = pct > 85 ? H.unhealthy : pct > 70 ? H.degraded : H.healthy
  return (
    <div style={{ display:'flex', alignItems:'center', gap:8 }}>
      <span style={{ fontSize:10, color:T.ink4, fontFamily:MONO, width:26, flexShrink:0 }}>{label}</span>
      <div style={{ flex:1, height:3, background:T.line2, borderRadius:2, overflow:'hidden' }}>
        <div style={{ width:`${Math.min(pct, 100)}%`, height:'100%', background:color, borderRadius:2 }} />
      </div>
      <span style={{ fontSize:10, color:T.ink3, fontFamily:MONO, width:28, textAlign:'right', flexShrink:0 }}>{pct}%</span>
    </div>
  )
}
