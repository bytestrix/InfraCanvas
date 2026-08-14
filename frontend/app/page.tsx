'use client'

import { useEffect, useState } from 'react'
import { useVMStore } from '@/store/vmStore'
import { connectVM } from '@/lib/wsManager'
import { fetchSessions, fetchJoinInfo, type JoinInfo, addCluster, type ClusterContextOption, previewClusterPermissions, type PermissionPreview, setClusterReadOnly, fetchAuditLog, type AuditEntry } from '@/lib/api'
import InfraCanvas from '@/components/canvas/InfraCanvas'
import AgentOverview from '@/components/agent/AgentOverview'
import { LogoMark } from '@/components/Logo'
import { useTheme } from '@/hooks/useTheme'
import { AlertCircle } from 'lucide-react'
import type { SessionInfo } from '@/types'

const LOCAL_KEY = 'local'
const SESSIONS_POLL_MS = 10_000

// The websocket/store key for a machine: the hub's own agent uses the
// 'local' alias, remote agents their session id.
const machineKey = (m: SessionInfo) => (m.local ? LOCAL_KEY : m.id)

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

type View = 'overview' | 'canvas' | 'audit'

const IcAudit = () => (
  <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4">
    <path d="M3 2h7l3 3v9a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V3a1 1 0 0 1 1-1Z"/>
    <path d="M10 2v3h3M5 8h6M5 11h4" strokeLinecap="round"/>
  </svg>
)
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
const IcClusters = () => (
  <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4">
    <path d="M8 1.5l6 3.2v6.6L8 14.5l-6-3.2V4.7z"/>
    <path d="M8 8v6.5M8 8L2 4.8M8 8l6-3.2" strokeLinecap="round"/>
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
  const { vms, machines, activeKey, setMachines, setActiveKey } = useVMStore()
  const vm = vms[activeKey]
  const [view, setView] = useState<View>('overview')

  useEffect(() => {
    if (!vms[LOCAL_KEY]) connectVM(LOCAL_KEY)
  }, [vms])

  // Keep the machine list fresh; failures (e.g. old server) leave it empty,
  // which hides the section entirely.
  useEffect(() => {
    let stop = false
    const poll = () => fetchSessions().then(m => { if (!stop) setMachines(m) }).catch(() => {})
    poll()
    const t = setInterval(poll, SESSIONS_POLL_MS)
    return () => { stop = true; clearInterval(t) }
  }, [setMachines])

  const selectMachine = (m: SessionInfo) => {
    const key = machineKey(m)
    if (!vms[key]) connectVM(key)
    setActiveKey(key)
  }

  const [showAddMachine, setShowAddMachine] = useState(false)
  const [showAddCluster, setShowAddCluster] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(false)

  const machineList = machines.filter(m => m.kind !== 'cluster')
  const clusterList = machines.filter(m => m.kind === 'cluster')

  return (
    <div className="app-shell" style={{ display:'grid', gridTemplateColumns:'200px 1fr', height:'100vh', background:T.bg, fontFamily:SANS, overflow:'hidden', position:'relative' }}>
      <button
        className="sidebar-toggle"
        onClick={() => setSidebarOpen(v => !v)}
        aria-label="Toggle sidebar"
        style={{ width:34, height:34, borderRadius:8, border:`1px solid ${T.line2}`, background:T.surface, color:T.ink2, alignItems:'center', justifyContent:'center', cursor:'pointer' }}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/></svg>
      </button>
      <div className={`sidebar-backdrop${sidebarOpen ? ' sidebar-open' : ''}`} onClick={() => setSidebarOpen(false)} />
      <div className={`app-sidebar${sidebarOpen ? ' sidebar-open' : ''}`} style={{ display:'flex', background:T.bg }}>
        <Sidebar vm={vm} view={view} onViewChange={(v) => { setView(v); setSidebarOpen(false) }}
          machines={machineList} clusters={clusterList} activeKey={activeKey}
          onSelectMachine={(m) => { selectMachine(m); setSidebarOpen(false) }}
          onAddMachine={() => { setShowAddMachine(true); setSidebarOpen(false) }}
          onAddCluster={() => { setShowAddCluster(true); setSidebarOpen(false) }} />
      </div>
      <main style={{ overflow:'hidden', minWidth:0, height:'100vh', display:'flex', flexDirection:'column' }}>
        <MainContent vm={vm} vmKey={activeKey} view={view} onSwitchToCanvas={() => setView('canvas')} />
      </main>
      {showAddMachine && <AddMachineModal onClose={() => setShowAddMachine(false)} />}
      {showAddCluster && <AddClusterModal onClose={() => setShowAddCluster(false)} />}
      <style>{`@keyframes pulse{0%,100%{opacity:1}50%{opacity:.4}}`}</style>
    </div>
  )
}

/* ── Add machine modal ── */
function AddMachineModal({ onClose }: { onClose: () => void }) {
  const [info, setInfo] = useState<JoinInfo | null>(null)
  const [failed, setFailed] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    fetchJoinInfo().then(setInfo).catch(() => setFailed(true))
  }, [])

  const reachable = !!info?.joinUrl
  const cmd = reachable
    ? `curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | sudo bash -s -- --join ${info!.joinUrl} --token ${info!.token}`
    : ''

  const copy = () => {
    navigator.clipboard?.writeText(cmd).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1800)
    })
  }

  return (
    <div onClick={onClose} style={{
      position:'fixed', inset:0, background:'var(--overlay, rgba(0,0,0,0.55))', zIndex:100,
      display:'flex', alignItems:'center', justifyContent:'center', padding:20,
    }}>
      <div onClick={e => e.stopPropagation()} style={{
        maxWidth:560, width:'100%', background:T.surface, border:`1px solid ${T.line}`,
        borderRadius:14, padding:'22px 24px 20px', display:'flex', flexDirection:'column', gap:14,
      }}>
        <div>
          <div style={{ fontSize:15, fontWeight:600, color:T.ink }}>Add a machine</div>
          <div style={{ fontSize:12.5, color:T.ink3, marginTop:4, lineHeight:1.5 }}>
            Run this on the VM you want on the canvas. It connects <strong>outbound-only</strong> to
            this dashboard — no port is opened on that VM.
          </div>
        </div>

        {failed && (
          <div style={{ fontSize:12.5, color:T.ink2, padding:'10px 12px', borderRadius:8, background:T.bg, border:`1px solid ${T.line}` }}>
            Couldn&apos;t load join info — the server may be an older version. Run{' '}
            <code style={{ fontFamily:MONO }}>infracanvas url</code> on the hub VM to see the join command.
          </div>
        )}

        {info && !reachable && (
          <div style={{ fontSize:12.5, color:T.ink2, padding:'10px 12px', borderRadius:8, background:T.bg, border:`1px solid ${T.line}`, lineHeight:1.6 }}>
            This dashboard isn&apos;t reachable from other machines (it&apos;s bound privately).
            Restart without <code style={{ fontFamily:MONO }}>--private</code>, or put a reverse proxy
            in front, then on each VM run:{' '}
            <code style={{ fontFamily:MONO }}>infracanvas start --backend &lt;this-hub-url&gt; --token {info.token}</code>
          </div>
        )}

        {reachable && (
          <>
            <div style={{ position:'relative' }}>
              <pre style={{
                margin:0, padding:'12px 14px', borderRadius:9, background:T.bg,
                border:`1px solid ${T.line}`, fontSize:11.5, fontFamily:MONO, color:T.ink2,
                whiteSpace:'pre-wrap', wordBreak:'break-all', lineHeight:1.6,
              }}>{cmd}</pre>
              <button onClick={copy} style={{
                position:'absolute', top:8, right:8, fontSize:11, fontFamily:MONO,
                padding:'3px 9px', borderRadius:6, cursor:'pointer',
                border:`1px solid ${T.line2}`, background:T.surface, color: copied ? H.healthy : T.ink2,
              }}>{copied ? 'copied ✓' : 'copy'}</button>
            </div>
            {info!.caveat && (
              <div style={{ fontSize:11.5, color:T.ink4, lineHeight:1.5 }}>Note: {info!.caveat}</div>
            )}
            <div style={{ fontSize:11.5, color:T.ink4, lineHeight:1.5 }}>
              Binary already installed on that VM? Run{' '}
              <code style={{ fontFamily:MONO }}>infracanvas start --backend {info!.joinUrl} --token {info!.token}</code>
            </div>
          </>
        )}

        <div style={{ display:'flex', justifyContent:'flex-end' }}>
          <button onClick={onClose} style={{
            fontSize:12.5, padding:'6px 14px', borderRadius:7, cursor:'pointer',
            border:`1px solid ${T.line2}`, background:'transparent', color:T.ink2, fontFamily:SANS,
          }}>Close</button>
        </div>
      </div>
    </div>
  )
}

/* ── Add cluster modal ── */
function AddClusterModal({ onClose }: { onClose: () => void }) {
  const [kubeconfig, setKubeconfig] = useState('')
  const [contexts, setContexts] = useState<ClusterContextOption[] | null>(null)
  const [selected, setSelected] = useState('')
  const [readOnly, setReadOnly] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [preview, setPreview] = useState<PermissionPreview | null>(null)
  const [previewFor, setPreviewFor] = useState('') // which context the current preview is for
  const [previewBusy, setPreviewBusy] = useState(false)

  const onFile = async (file: File) => {
    setKubeconfig(await file.text())
    setError('')
  }

  const listContexts = async () => {
    if (!kubeconfig.trim()) return
    setBusy(true)
    setError('')
    try {
      const res = await addCluster(kubeconfig)
      if ('contexts' in res) {
        setContexts(res.contexts)
        setSelected(res.contexts.find(c => c.current)?.name ?? res.contexts[0]?.name ?? '')
      } else {
        onClose() // shouldn't happen (server always returns contexts first), but handle gracefully
      }
    } catch (e: any) {
      setError(e.message ?? 'Failed to parse kubeconfig')
    } finally {
      setBusy(false)
    }
  }

  const loadPreview = async (contextName: string) => {
    setPreviewBusy(true)
    setPreview(null)
    try {
      const p = await previewClusterPermissions(kubeconfig, contextName)
      setPreview(p)
      setPreviewFor(contextName)
    } catch {
      // Non-fatal — connecting still works without a preview, just skip showing one.
    } finally {
      setPreviewBusy(false)
    }
  }

  const connect = async () => {
    if (!selected) return
    setBusy(true)
    setError('')
    try {
      await addCluster(kubeconfig, selected, selected, readOnly)
      onClose()
    } catch (e: any) {
      setError(e.message ?? 'Failed to connect cluster')
      setBusy(false)
    }
  }

  useEffect(() => {
    if (selected && selected !== previewFor) loadPreview(selected)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected])

  return (
    <div onClick={onClose} style={{
      position:'fixed', inset:0, background:'var(--overlay, rgba(0,0,0,0.55))', zIndex:100,
      display:'flex', alignItems:'center', justifyContent:'center', padding:20,
    }}>
      <div onClick={e => e.stopPropagation()} style={{
        maxWidth:560, width:'100%', background:T.surface, border:`1px solid ${T.line}`,
        borderRadius:14, padding:'22px 24px 20px', display:'flex', flexDirection:'column', gap:14,
      }}>
        <div>
          <div style={{ fontSize:15, fontWeight:600, color:T.ink }}>Connect a cluster</div>
          <div style={{ fontSize:12.5, color:T.ink3, marginTop:4, lineHeight:1.5 }}>
            Drop or paste a kubeconfig. It stays on this machine and is never sent
            anywhere — the dashboard talks to your cluster&apos;s API server directly,
            the same way <code style={{ fontFamily:MONO }}>kubectl</code> does.
          </div>
        </div>

        {!contexts && (
          <>
            <label style={{
              display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center', gap:6,
              padding:'22px 14px', borderRadius:10, border:`1.5px dashed ${T.line2}`, cursor:'pointer',
              background:T.bg, textAlign:'center',
            }}>
              <span style={{ fontSize:12.5, color:T.ink2 }}>Click to choose a kubeconfig file</span>
              <span style={{ fontSize:11, color:T.ink4 }}>or paste its contents below</span>
              <input type="file" accept=".yaml,.yml,text/yaml,text/plain" style={{ display:'none' }}
                onChange={e => { const f = e.target.files?.[0]; if (f) onFile(f) }} />
            </label>
            <textarea
              value={kubeconfig}
              onChange={e => setKubeconfig(e.target.value)}
              placeholder="apiVersion: v1&#10;kind: Config&#10;clusters: ..."
              rows={6}
              style={{
                width:'100%', padding:'10px 12px', borderRadius:9, background:T.bg,
                border:`1px solid ${T.line}`, fontSize:11.5, fontFamily:MONO, color:T.ink2,
                resize:'vertical', boxSizing:'border-box',
              }}
            />
          </>
        )}

        {contexts && (
          <div style={{ display:'flex', flexDirection:'column', gap:6 }}>
            <div style={{ fontSize:12, color:T.ink3 }}>
              {contexts.length > 1 ? 'This kubeconfig has multiple contexts — pick the one to connect:' : 'Confirm the cluster to connect:'}
            </div>
            {contexts.map(c => (
              <label key={c.name} style={{
                display:'flex', alignItems:'center', gap:9, padding:'8px 10px', borderRadius:8,
                border:`1px solid ${selected === c.name ? T.line2 : T.line}`,
                background: selected === c.name ? T.bg : 'transparent', cursor:'pointer',
              }}>
                <input type="radio" name="context" checked={selected === c.name} onChange={() => setSelected(c.name)} />
                <div style={{ minWidth:0 }}>
                  <div style={{ fontSize:12.5, color:T.ink, fontFamily:MONO }}>{c.name}</div>
                  <div style={{ fontSize:11, color:T.ink4, overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap' }}>{c.serverUrl}</div>
                </div>
              </label>
            ))}

            {previewBusy && (
              <div style={{ fontSize:11.5, color:T.ink4, padding:'6px 2px' }}>Checking what this credential can do…</div>
            )}
            {preview && previewFor === selected && (
              <div style={{ borderRadius:9, border:`1px solid ${T.line}`, background:T.bg, padding:'10px 12px', display:'flex', flexDirection:'column', gap:6 }}>
                <div style={{ fontSize:11, color:T.ink3, textTransform:'uppercase', letterSpacing:'0.06em', fontWeight:600 }}>What this credential can do</div>
                <div style={{ display:'flex', flexWrap:'wrap', gap:6 }}>
                  {[
                    ['View pods/deployments', preview.canView],
                    ['Open a terminal (exec)', preview.canExec],
                    ['Restart / kill pods', preview.canRestartOrKill],
                    ['Scale / edit deployments', preview.canScaleOrEdit],
                    ['Read Secret contents', preview.canViewSecrets],
                  ].map(([label, allowed]) => (
                    <span key={label as string} style={{
                      fontSize:10.5, fontFamily:MONO, padding:'2px 8px', borderRadius:20,
                      background: allowed ? 'rgba(34,197,94,0.1)' : T.bg,
                      border: `1px solid ${allowed ? 'rgba(34,197,94,0.3)' : T.line2}`,
                      color: allowed ? '#22c55e' : T.ink4,
                    }}>{allowed ? '✓' : '—'} {label}</span>
                  ))}
                </div>
                {preview.warnings?.map(w => (
                  <div key={w} style={{ fontSize:11, color:'#f59e0b', lineHeight:1.4 }}>⚠ {w}</div>
                ))}
              </div>
            )}

            <label style={{ display:'flex', alignItems:'center', gap:8, fontSize:12, color:T.ink2, cursor:'pointer', padding:'2px 2px' }}>
              <input type="checkbox" checked={readOnly} onChange={e => setReadOnly(e.target.checked)} />
              Connect as read-only — blocks every action and terminal session on this cluster, even outside global read-only mode. Can be changed later.
            </label>
          </div>
        )}

        {error && (
          <div style={{ fontSize:12, color:'#ef4444', padding:'8px 10px', borderRadius:8, background:'rgba(239,68,68,0.08)', border:'1px solid rgba(239,68,68,0.2)' }}>
            {error}
          </div>
        )}

        <div style={{ display:'flex', justifyContent:'flex-end', gap:8 }}>
          <button onClick={onClose} style={{
            fontSize:12.5, padding:'6px 14px', borderRadius:7, cursor:'pointer',
            border:`1px solid ${T.line2}`, background:'transparent', color:T.ink2, fontFamily:SANS,
          }}>Cancel</button>
          {!contexts ? (
            <button onClick={listContexts} disabled={!kubeconfig.trim() || busy} style={{
              fontSize:12.5, padding:'6px 14px', borderRadius:7, cursor: kubeconfig.trim() ? 'pointer' : 'default',
              border:'none', background:T.ink, color:T.bg, fontFamily:SANS, opacity: busy ? 0.6 : 1,
            }}>{busy ? 'Reading…' : 'Continue'}</button>
          ) : (
            <button onClick={connect} disabled={!selected || busy} style={{
              fontSize:12.5, padding:'6px 14px', borderRadius:7, cursor: selected ? 'pointer' : 'default',
              border:'none', background:T.ink, color:T.bg, fontFamily:SANS, opacity: busy ? 0.6 : 1,
            }}>{busy ? 'Connecting…' : 'Connect'}</button>
          )}
        </div>
      </div>
    </div>
  )
}

/* ── Sidebar ── */
function Sidebar({ vm, view, onViewChange, machines, clusters, activeKey, onSelectMachine, onAddMachine, onAddCluster }: {
  vm: any; view: View; onViewChange: (v: View) => void
  machines: SessionInfo[]; clusters: SessionInfo[]; activeKey: string; onSelectMachine: (m: SessionInfo) => void
  onAddMachine: () => void; onAddCluster: () => void
}) {
  const connected = vm?.status === 'connected'
  const hostname  = vm?.hostname ?? null
  const { theme, toggle } = useTheme()

  const navItems: { id: View; label: string; Icon: () => JSX.Element }[] = [
    { id: 'overview', label: 'Overview', Icon: IcOverview },
    { id: 'canvas',   label: 'Canvas',   Icon: IcCanvas   },
    { id: 'audit',    label: 'Audit',    Icon: IcAudit    },
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

      {/* Machines — hub mode: hidden until the server reports sessions */}
      {machines.length > 0 && (
        <div style={{ marginTop:18 }}>
          <div style={{ display:'flex', alignItems:'center', justifyContent:'space-between', padding:'0 8px 6px' }}>
            <div style={{ fontSize:10, fontWeight:600, letterSpacing:'0.08em', textTransform:'uppercase', color:T.ink4, fontFamily:MONO }}>
              Machines
            </div>
            <button onClick={onAddMachine} title="Add a VM to this dashboard" style={{
              border:'none', background:'transparent', cursor:'pointer', color:T.ink3,
              fontSize:14, lineHeight:1, padding:'2px 4px', borderRadius:5, fontFamily:MONO,
            }}
              onMouseEnter={e=>{ (e.currentTarget as any).style.color=T.ink; (e.currentTarget as any).style.background=T.surface }}
              onMouseLeave={e=>{ (e.currentTarget as any).style.color=T.ink3; (e.currentTarget as any).style.background='transparent' }}
            >+</button>
          </div>
          <div style={{ display:'flex', flexDirection:'column', gap:1 }}>
            {machines.map((m) => {
              const key = machineKey(m)
              const active = key === activeKey
              return (
                <button key={m.id} onClick={() => onSelectMachine(m)} title={m.hostname} style={{
                  display:'flex', alignItems:'center', gap:8,
                  height:28, padding:'0 8px', borderRadius:7,
                  fontSize:12.5, border:'none', cursor:'pointer', width:'100%', textAlign:'left',
                  color: active ? T.ink : T.ink2,
                  background: active ? T.surface : 'transparent',
                  fontFamily: SANS, transition:'all 0.1s',
                }}
                  onMouseEnter={e=>{ if(!active){(e.currentTarget as any).style.color=T.ink;(e.currentTarget as any).style.background=T.surface} }}
                  onMouseLeave={e=>{ if(!active){(e.currentTarget as any).style.color=T.ink2;(e.currentTarget as any).style.background='transparent'} }}
                >
                  <span style={{ width:6, height:6, borderRadius:'50%', flexShrink:0, background: m.online ? H.healthy : T.ink4 }} />
                  <span style={{ overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap', flex:1 }}>
                    {m.hostname || m.id}
                  </span>
                  {m.local && <span style={{ fontSize:9.5, color:T.ink4, fontFamily:MONO, flexShrink:0 }}>local</span>}
                </button>
              )
            })}
          </div>
        </div>
      )}

      {/* Clusters — kubeconfig direct-connect / relay pod, no VM agent needed */}
      {clusters.length > 0 && (
        <div style={{ marginTop:18 }}>
          <div style={{ display:'flex', alignItems:'center', justifyContent:'space-between', padding:'0 8px 6px' }}>
            <div style={{ fontSize:10, fontWeight:600, letterSpacing:'0.08em', textTransform:'uppercase', color:T.ink4, fontFamily:MONO }}>
              Clusters
            </div>
            <button onClick={onAddCluster} title="Connect a Kubernetes cluster via kubeconfig" style={{
              border:'none', background:'transparent', cursor:'pointer', color:T.ink3,
              fontSize:14, lineHeight:1, padding:'2px 4px', borderRadius:5, fontFamily:MONO,
            }}
              onMouseEnter={e=>{ (e.currentTarget as any).style.color=T.ink; (e.currentTarget as any).style.background=T.surface }}
              onMouseLeave={e=>{ (e.currentTarget as any).style.color=T.ink3; (e.currentTarget as any).style.background='transparent' }}
            >+</button>
          </div>
          <div style={{ display:'flex', flexDirection:'column', gap:1 }}>
            {clusters.map((m) => {
              const key = machineKey(m)
              const active = key === activeKey
              return (
                <button key={m.id} onClick={() => onSelectMachine(m)} title={m.hostname} style={{
                  display:'flex', alignItems:'center', gap:8,
                  height:28, padding:'0 8px', borderRadius:7,
                  fontSize:12.5, border:'none', cursor:'pointer', width:'100%', textAlign:'left',
                  color: active ? T.ink : T.ink2,
                  background: active ? T.surface : 'transparent',
                  fontFamily: SANS, transition:'all 0.1s',
                }}
                  onMouseEnter={e=>{ if(!active){(e.currentTarget as any).style.color=T.ink;(e.currentTarget as any).style.background=T.surface} }}
                  onMouseLeave={e=>{ if(!active){(e.currentTarget as any).style.color=T.ink2;(e.currentTarget as any).style.background='transparent'} }}
                >
                  <span style={{ width:6, height:6, borderRadius:'50%', flexShrink:0, background: m.online ? H.healthy : T.ink4 }} />
                  <span style={{ overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap', flex:1 }}>
                    {m.hostname || m.id}
                  </span>
                  {m.machineId?.startsWith('cluster-') && (
                    <span
                      role="button"
                      title={m.readOnly ? 'Read-only — click to allow writes again' : 'Click to make this cluster read-only'}
                      onClick={e => {
                        e.stopPropagation()
                        const clusterId = m.machineId!.slice('cluster-'.length)
                        setClusterReadOnly(clusterId, !m.readOnly).catch(() => {})
                      }}
                      style={{
                        fontSize:9, fontFamily:MONO, padding:'1px 5px', borderRadius:20, flexShrink:0, cursor:'pointer',
                        background: m.readOnly ? T.bg : 'transparent',
                        border:`1px solid ${m.readOnly ? T.line2 : 'transparent'}`,
                        color: m.readOnly ? T.ink4 : T.ink4,
                        opacity: m.readOnly ? 1 : 0.5,
                      }}
                    >{m.readOnly ? 'RO' : 'RW'}</span>
                  )}
                </button>
              )
            })}
          </div>
        </div>
      )}
      {clusters.length === 0 && (
        <div style={{ marginTop:18, padding:'0 8px' }}>
          <button onClick={onAddCluster} title="Connect a Kubernetes cluster via kubeconfig" style={{
            display:'flex', alignItems:'center', gap:8, width:'100%',
            height:28, padding:'0 8px', borderRadius:7, border:'none', cursor:'pointer',
            fontSize:12, color:T.ink3, background:'transparent', fontFamily:SANS,
          }}
            onMouseEnter={e=>{ (e.currentTarget as any).style.color=T.ink; (e.currentTarget as any).style.background=T.surface }}
            onMouseLeave={e=>{ (e.currentTarget as any).style.color=T.ink3; (e.currentTarget as any).style.background='transparent' }}
          >
            <IcClusters />
            <span>Connect a cluster</span>
          </button>
        </div>
      )}

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
function MainContent({ vm, vmKey, view, onSwitchToCanvas }: { vm: any; vmKey: string; view: View; onSwitchToCanvas: () => void }) {
  const isLoading = !vm || vm.status === 'connecting' || vm.status === 'paired'
  const isError   = vm?.status === 'error'

  if (isError) {
    return <ErrorContent message={vm.error ?? 'Connection failed'} />
  }

  if (view === 'audit') {
    return <AuditView />
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
      vmCode={vmKey}
      onSwitchToCanvas={onSwitchToCanvas}
    />
  )
}

/* ── Audit log ──
   Write actions and terminal sessions requested through this dashboard.
   Entries are attributed by session/machine, not by user — OSS has one
   shared UI token authenticating the whole dashboard, no per-user login,
   so that's the real ceiling on what can be logged here. */
function AuditView() {
  const [entries, setEntries] = useState<AuditEntry[] | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let stop = false
    const poll = () => fetchAuditLog(300).then(e => { if (!stop) { setEntries(e); setError('') } }).catch(e => { if (!stop) setError(e.message ?? 'Failed to load audit log') })
    poll()
    const t = setInterval(poll, 5000)
    return () => { stop = true; clearInterval(t) }
  }, [])

  const eventLabel = (e: AuditEntry) => {
    if (e.event === 'exec_requested') return 'Terminal opened'
    if (e.event === 'action_requested') return `Requested: ${e.type ?? 'action'}`
    if (e.event === 'action_completed') return e.success ? 'Completed' : 'Failed'
    return e.event
  }

  return (
    <div style={{ flex:1, overflow:'auto', padding:'28px 36px' }}>
      <div style={{ fontSize:20, fontWeight:600, color:T.ink, marginBottom:4 }}>Audit log</div>
      <div style={{ fontSize:12.5, color:T.ink3, marginBottom:20, maxWidth:640, lineHeight:1.5 }}>
        Every write action and terminal session requested through this dashboard, most recent first.
        Attributed by machine, not by user — there&apos;s no per-user login in the self-hosted version,
        just the one dashboard token.
      </div>

      {error && (
        <div style={{ fontSize:12, color:'#ef4444', padding:'8px 10px', borderRadius:8, background:'rgba(239,68,68,0.08)', border:'1px solid rgba(239,68,68,0.2)', marginBottom:14 }}>{error}</div>
      )}

      {entries === null ? (
        <div style={{ fontSize:12.5, color:T.ink4 }}>Loading…</div>
      ) : entries.length === 0 ? (
        <div style={{ fontSize:12.5, color:T.ink4 }}>No write actions or terminal sessions recorded yet.</div>
      ) : (
        <div style={{ display:'flex', flexDirection:'column', gap:1, border:`1px solid ${T.line}`, borderRadius:10, overflow:'hidden' }}>
          {entries.map((e, i) => (
            <div key={i} style={{
              display:'flex', alignItems:'center', gap:12, padding:'9px 14px', fontSize:12, background:T.surface,
              borderBottom: i === entries.length - 1 ? 'none' : `1px solid ${T.line}`,
            }}>
              <span style={{ fontFamily:MONO, fontSize:10.5, color:T.ink4, width:150, flexShrink:0 }}>
                {new Date(e.timestamp).toLocaleString()}
              </span>
              <span style={{
                fontSize:10, fontFamily:MONO, padding:'1px 7px', borderRadius:20, flexShrink:0, width:110, textAlign:'center',
                background: e.event === 'action_completed' ? (e.success ? 'rgba(34,197,94,0.1)' : 'rgba(239,68,68,0.1)') : T.bg,
                border: `1px solid ${e.event === 'action_completed' ? (e.success ? 'rgba(34,197,94,0.3)' : 'rgba(239,68,68,0.3)') : T.line2}`,
                color: e.event === 'action_completed' ? (e.success ? '#22c55e' : '#ef4444') : T.ink3,
              }}>{eventLabel(e)}</span>
              <span style={{ color:T.ink2, flexShrink:0 }}>{e.hostname || e.machine_id || '—'}</span>
              {e.entity_id && (
                <span style={{ fontFamily:MONO, color:T.ink4, overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap', flex:1 }}>
                  {e.namespace ? `${e.namespace}/` : ''}{e.entity_id}
                </span>
              )}
              {e.message && (
                <span style={{ color:T.ink4, overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap', flex: e.entity_id ? 0 : 1, maxWidth:280 }} title={e.message}>
                  {e.message}
                </span>
              )}
            </div>
          ))}
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
