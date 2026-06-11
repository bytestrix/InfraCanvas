'use client'

import { useState, useMemo } from 'react'
import { Terminal, LayoutGrid, AlertTriangle } from 'lucide-react'
import { GraphNode, GraphOutput } from '@/types'
import NodeSvgIcon from '@/components/canvas/NodeSvgIcon'
import GroupDrawer from '@/components/canvas/GroupDrawer'
import NodeDetailPanel from '@/components/canvas/NodeDetailPanel'
import LogsPanel from '@/components/canvas/LogsPanel'
import TerminalPanel from '@/components/canvas/TerminalPanel'
import { type GroupInfo } from '@/lib/graphPreprocess'

const T = {
  bg:'var(--bg)', surface:'var(--surface)', surface2:'var(--surface-2)',
  line:'var(--line)', line2:'var(--line2)', line3:'var(--line3)',
  ink:'var(--ink)', ink2:'var(--ink2)', ink3:'var(--ink3)', ink4:'var(--ink4)',
}
const H = { healthy:'#22c55e', degraded:'#f59e0b', unhealthy:'#ef4444', unknown:'#6b7280' }
const MONO = "var(--font-geist-mono,'Geist Mono',ui-monospace,monospace)"
const SANS = "var(--font-geist,'Geist',ui-sans-serif,system-ui,sans-serif)"

interface Props {
  graph: GraphOutput
  hostname: string | null
  vmCode: string
  onSwitchToCanvas?: () => void
}

export default function AgentOverview({ graph, hostname, vmCode, onSwitchToCanvas }: Props) {
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [nsFilter]                  = useState<string>('all')
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null)
  const [showLogs, setShowLogs]         = useState(false)
  const [showTerminal, setShowTerminal] = useState(false)
  const [terminalLayer, setTerminalLayer] = useState<'docker'|'host'|'kubernetes'>('docker')
  const [showHostTerminal, setShowHostTerminal] = useState(false)

  const nodes = graph.nodes

  const d = useMemo(() => {
    const byType = (t: string) => nodes.filter(n => n.type === t)
    const host       = byType('host')[0] ?? null
    const clusters   = byType('cluster')
    const runtimes   = byType('container_runtime')
    const namespaces = byType('namespace')
    const pods       = byType('pod')
    const containers = byType('container')
    const k8sNodes   = byType('node')
    const jobs       = byType('job')
    const cronjobs   = byType('cronjob')
    const pvcs       = byType('pvc')
    const pvs        = byType('pv')
    const deployments  = byType('deployment')
    const statefulsets = byType('statefulset')
    const daemonsets   = byType('daemonset')
    const services     = byType('k8s_service')
    const ingresses    = byType('ingress')
    const volumes      = byType('volume')
    const sysServices  = byType('service')
    const processes    = byType('process')
    const hc = (arr: GraphNode[], h: string) => arr.filter(n => n.health === h).length
    const alerts = nodes.filter(n => (n.health === 'degraded' || n.health === 'unhealthy') && n.type !== 'host')
    return {
      host, clusters, runtimes, namespaces, pods, containers, k8sNodes,
      jobs, cronjobs, pvcs, pvs, deployments, statefulsets, daemonsets,
      services, ingresses, volumes, sysServices, processes, alerts,
      totalDegraded:  nodes.filter(n => n.health === 'degraded').length,
      totalUnhealthy: nodes.filter(n => n.health === 'unhealthy').length,
      podsRunning: hc(pods, 'healthy'),
    }
  }, [nodes])

  const hostMeta = d.host?.metadata ?? {}
  const cpuPct  = typeof hostMeta.cpu_percent    === 'number' ? Math.round(hostMeta.cpu_percent)    : null
  const memPct  = typeof hostMeta.memory_percent === 'number' ? Math.round(hostMeta.memory_percent) : null
  const diskPct = typeof hostMeta.disk_percent   === 'number' ? Math.round(hostMeta.disk_percent)   : null

  const displayIp = useMemo(() => {
    const mi = (hostMeta as any).ip ?? (hostMeta as any).ipAddress ?? (hostMeta as any).ip_address
    if (mi && mi !== '127.0.0.1' && mi !== '::1') return String(mi)
    return null
  }, [hostMeta])

  const tiers = useMemo(() => {
    const all: any[] = []

    all.push({
      id: 'host', label: 'Host', count: 1,
      tiles: [{
        id: 'host-vm', typeLabel: 'VM HOST', icon: 'host',
        name: hostname ?? d.host?.label ?? 'VM Host',
        count: 1,
        stats: [
          ['cpu',  cpuPct  !== null ? `${cpuPct}%`  : '—'],
          ['mem',  memPct  !== null ? `${memPct}%`  : '—'],
          ['disk', diskPct !== null ? `${diskPct}%` : '—'],
        ] as [string,string][],
        health: null, state: '', raw: d.host,
      }],
    })

    const runtimeTiles = [
      ...d.clusters.map(c => ({
        id: c.id, typeLabel: 'CLUSTER · KUBERNETES', icon: 'k8s',
        name: c.label, count: 1,
        stats: [['version', c.metadata?.version ?? '—'], ['nodes', String(d.k8sNodes.length)], ['pods', String(d.pods.length)]] as [string,string][],
        health: null, state: '', raw: c,
      })),
      ...d.runtimes.map(r => ({
        id: r.id, typeLabel: 'RUNTIME · DOCKER', icon: 'docker',
        name: r.label, count: 1,
        stats: [['version', r.metadata?.version ?? '—'], ['containers', String(d.containers.length)], ['volumes', String(d.volumes.length + d.pvcs.length)]] as [string,string][],
        health: null, state: '', raw: r,
      })),
    ]
    if (runtimeTiles.length > 0) all.push({ id:'runtimes', label:'Runtimes', count:runtimeTiles.length, tiles:runtimeTiles })

    const mkH = (arr: GraphNode[]) => ({ h:arr.filter(n=>n.health==='healthy').length, d:arr.filter(n=>n.health==='degraded').length, u:arr.filter(n=>n.health==='unhealthy').length })

    const coreTiles: any[] = []
    if (d.k8sNodes.length > 0)   coreTiles.push({ id:'tile-k8snodes',   typeLabel:'K8S NODES',   icon:'k8s-node',  name:'cluster nodes',    count:d.k8sNodes.length,   stats:[['ready',`${d.k8sNodes.filter(n=>n.health==='healthy').length}/${d.k8sNodes.length}`]] as [string,string][], health:mkH(d.k8sNodes),   state:d.k8sNodes.some(n=>n.health!=='healthy')?'warn':'' })
    if (d.namespaces.length > 0)  coreTiles.push({ id:'tile-namespaces', typeLabel:'NAMESPACES',  icon:'namespace', name:'namespaces',        count:d.namespaces.length,  stats:[['active',String(d.namespaces.filter(n=>n.health==='healthy').length)]] as [string,string][], health:mkH(d.namespaces),  state:'' })
    if (d.containers.length > 0)  coreTiles.push({ id:'tile-containers', typeLabel:'CONTAINERS',  icon:'docker',    name:'docker containers', count:d.containers.length,  stats:[['running',String(d.containers.filter(n=>n.health==='healthy').length)]] as [string,string][], health:mkH(d.containers),  state:'' })
    if (d.pvcs.length+d.pvs.length+d.volumes.length > 0) coreTiles.push({ id:'tile-volumes', typeLabel:'VOLUMES', icon:'volume', name:'storage volumes', count:d.pvcs.length+d.pvs.length+d.volumes.length, stats:[['total',String(d.pvcs.length+d.pvs.length+d.volumes.length)],['pvcs',String(d.pvcs.length)]] as [string,string][], health:null, state:'' })
    if (coreTiles.length > 0) all.push({ id:'core', label:'Core resources', count:coreTiles.reduce((s:number,t:any)=>s+t.count,0), tiles:coreTiles })

    const workTiles: any[] = []
    if (d.deployments.length > 0)  workTiles.push({ id:'tile-deployments',  typeLabel:'DEPLOYMENTS',  icon:'deployment',  name:'deployments',  count:d.deployments.length,  stats:[['ready',`${d.deployments.filter(n=>n.health==='healthy').length}/${d.deployments.length}`],['degraded',String(d.deployments.filter(n=>n.health==='degraded').length)]] as [string,string][], health:mkH(d.deployments),  state:d.deployments.some(n=>n.health==='degraded'||n.health==='unhealthy')?'warn':'' })
    if (d.statefulsets.length > 0) workTiles.push({ id:'tile-statefulsets', typeLabel:'STATEFULSETS', icon:'statefulset', name:'statefulsets', count:d.statefulsets.length, stats:[['ready',`${d.statefulsets.filter(n=>n.health==='healthy').length}/${d.statefulsets.length}`]] as [string,string][], health:mkH(d.statefulsets), state:d.statefulsets.some(n=>n.health!=='healthy')?'warn':'' })
    if (d.daemonsets.length > 0)   workTiles.push({ id:'tile-daemonsets',   typeLabel:'DAEMONSETS',   icon:'daemonset',   name:'daemonsets',   count:d.daemonsets.length,   stats:[['ready',`${d.daemonsets.filter(n=>n.health==='healthy').length}/${d.daemonsets.length}`]] as [string,string][], health:mkH(d.daemonsets),   state:'' })
    if (d.services.length > 0)     workTiles.push({ id:'tile-services',     typeLabel:'SERVICES',     icon:'service',     name:'k8s services', count:d.services.length,     stats:[['total',String(d.services.length)]] as [string,string][], health:mkH(d.services),     state:'' })
    if (d.ingresses.length > 0)    workTiles.push({ id:'tile-ingresses',    typeLabel:'INGRESSES',    icon:'ingress',     name:'ingresses',    count:d.ingresses.length,    stats:[['active',String(d.ingresses.length)]] as [string,string][], health:null, state:'' })
    if (d.sysServices.length > 0) workTiles.push({ id:'tile-sysservices', typeLabel:'SYSTEM SERVICES', icon:'sysservice', name:'systemd services', count:d.sysServices.length, stats:[['running',String(d.sysServices.filter(n=>n.health==='healthy').length)],['failed',String(d.sysServices.filter(n=>n.health==='unhealthy').length)]] as [string,string][], health:mkH(d.sysServices), state:d.sysServices.some(n=>n.health==='unhealthy')?'alert':'' })
    if (d.processes.length > 0)   workTiles.push({ id:'tile-processes',   typeLabel:'PROCESSES',       icon:'process', name:'listening processes', count:d.processes.length, stats:[['total',String(d.processes.length)]] as [string,string][], health:mkH(d.processes), state:'' })
    if (workTiles.length > 0) all.push({ id:'workloads', label:'Workloads', count:workTiles.reduce((s:number,t:any)=>s+t.count,0), tiles:workTiles })

    const jobTiles: any[] = []
    if (d.jobs.length > 0)     jobTiles.push({ id:'tile-jobs',     typeLabel:'JOBS',     icon:'job',     name:'jobs',     count:d.jobs.length,     stats:[['ok',String(d.jobs.filter(n=>n.health==='healthy').length)],['failed',String(d.jobs.filter(n=>n.health==='unhealthy').length)]] as [string,string][], health:mkH(d.jobs),     state:d.jobs.some(n=>n.health==='unhealthy')?'alert':'' })
    if (d.cronjobs.length > 0) jobTiles.push({ id:'tile-cronjobs', typeLabel:'CRONJOBS', icon:'cronjob', name:'cronjobs', count:d.cronjobs.length, stats:[['active',String(d.cronjobs.length)]] as [string,string][], health:null, state:'' })
    if (d.pods.length > 0)     jobTiles.push({ id:'tile-pods',     typeLabel:'PODS',     icon:'pod',     name:'pods',     count:d.pods.length,     stats:[['running',String(d.pods.filter(n=>n.health==='healthy').length)],['failed',String(d.pods.filter(n=>n.health==='unhealthy').length)]] as [string,string][], health:mkH(d.pods), state:d.pods.some(n=>n.health==='unhealthy')?'alert':'' })
    if (jobTiles.length > 0) all.push({ id:'jobs', label:'Jobs & Pods', count:jobTiles.reduce((s:number,t:any)=>s+t.count,0), tiles:jobTiles })

    return all
  }, [d, hostname, cpuPct, memPct, diskPct])

  const detailTile = tiers.flatMap((t:any) => t.tiles).find((t: any) => t.id === selectedId)

  const detailNodes = useMemo((): GraphNode[] => {
    if (!selectedId) return []
    const typeMap: Record<string, string[]> = {
      'host-vm':          ['host'],
      'tile-k8snodes':    ['node'],
      'tile-namespaces':  ['namespace'],
      'tile-containers':  ['container'],
      'tile-volumes':     ['pvc','pv','volume'],
      'tile-deployments': ['deployment'],
      'tile-statefulsets':['statefulset'],
      'tile-daemonsets':  ['daemonset'],
      'tile-services':    ['k8s_service'],
      'tile-sysservices': ['service'],
      'tile-processes':   ['process'],
      'tile-ingresses':   ['ingress'],
      'tile-jobs':        ['job'],
      'tile-cronjobs':    ['cronjob'],
      'tile-pods':        ['pod'],
    }
    const types = typeMap[selectedId]
    let base = types ? nodes.filter(n => types.includes(n.type)) : nodes.filter(n => n.id === selectedId)
    if (nsFilter !== 'all' && selectedId !== 'host-vm' && selectedId !== 'tile-namespaces' && selectedId !== 'tile-k8snodes') {
      base = base.filter(n => n.metadata?.namespace === nsFilter)
    }
    return base
  }, [selectedId, nodes, nsFilter])

  const groupInfo = useMemo((): GroupInfo | null => {
    if (!detailTile || !detailNodes.length) return null
    const iconToType: Record<string, string> = {
      'host':'host', 'k8s':'cluster', 'k8s-node':'node', 'docker':'container_runtime',
      'namespace':'namespace', 'deployment':'deployment', 'statefulset':'statefulset',
      'daemonset':'daemonset', 'service':'k8s_service', 'ingress':'ingress',
      'pod':'pod', 'job':'job', 'cronjob':'cronjob', 'volume':'pvc',
      'sysservice':'service', 'process':'process',
    }
    const nodeType = iconToType[detailTile.icon] ?? detailTile.icon
    const hc = (h: string) => detailNodes.filter(n => n.health === h).length
    return {
      id: `group:${nodeType}`,
      type: nodeType,
      label: detailTile.name ?? detailTile.typeLabel,
      count: detailNodes.length,
      healthCounts: { healthy: hc('healthy'), degraded: hc('degraded'), unhealthy: hc('unhealthy'), unknown: hc('unknown') },
      nodes: detailNodes,
    }
  }, [detailTile, detailNodes])

  function handleTileClick(id: string) {
    // Host tile has exactly 1 node — skip the drawer, open detail directly
    if (id === 'host-vm' && d.host) {
      setSelectedNode(d.host)
      setSelectedId(null)
      setShowLogs(false)
      setShowTerminal(false)
      setShowHostTerminal(false)
      return
    }
    setSelectedId(id)
    setSelectedNode(null)
    setShowLogs(false)
    setShowTerminal(false)
    setShowHostTerminal(false)
  }

  function closeAll() {
    setSelectedId(null)
    setSelectedNode(null)
    setShowLogs(false)
    setShowTerminal(false)
    setShowHostTerminal(false)
  }

  const anyPanelOpen = !!(selectedId || selectedNode || showLogs || showTerminal)

  return (
    <div style={{ display:'flex', height:'100%', fontFamily:SANS, fontSize:13, overflow:'hidden', position:'relative' }}>
      {/* ── Main pane ── */}
      <div style={{ flex:1, overflowY:'auto', minWidth:0 }}>
        {/* Page header */}
        <div style={{ padding:'28px 36px 20px', borderBottom:`1px solid ${T.line}` }}>
          <h1 style={{ margin:0, fontSize:22, fontWeight:500, letterSpacing:'-0.025em', color:T.ink }}>
            {hostname ?? 'Local machine'}
          </h1>
          <div style={{ marginTop:7, display:'flex', alignItems:'center', gap:12, fontSize:12, color:T.ink3, fontFamily:MONO, flexWrap:'wrap' }}>
            {hostMeta.os   && <span>{hostMeta.os}</span>}
            {hostMeta.arch && <><DotSep /><span>{hostMeta.arch}</span></>}
            {displayIp     && <><DotSep /><span style={{ color:T.ink }}>{displayIp}</span></>}
            {hostMeta.kernel_version && <><DotSep /><span>{hostMeta.kernel_version}</span></>}
          </div>
          <div style={{ marginTop:10, display:'flex', gap:6, flexWrap:'wrap', alignItems:'center' }}>
            {d.host && (
              <button onClick={() => { setShowHostTerminal(true); setSelectedId(null); setSelectedNode(null); setShowLogs(false); setShowTerminal(false) }} style={actionBtn}>
                <Terminal size={12} strokeWidth={1.5} />
                Terminal
              </button>
            )}
            {onSwitchToCanvas && (
              <button onClick={onSwitchToCanvas} style={actionBtn}>
                <LayoutGrid size={12} strokeWidth={1.5} />
                Canvas
              </button>
            )}
            {d.alerts.length > 0 && (
              <button style={{ display:'inline-flex', alignItems:'center', gap:5, height:28, padding:'0 10px', borderRadius:6, border:`1px solid ${(d.totalUnhealthy>0?H.unhealthy:H.degraded)+'40'}`, background:`${d.totalUnhealthy>0?H.unhealthy:H.degraded}12`, color:d.totalUnhealthy>0?H.unhealthy:H.degraded, fontSize:12, cursor:'default', fontFamily:SANS }}>
                <AlertTriangle size={12} strokeWidth={1.5} />
                {d.alerts.length} issues
              </button>
            )}
          </div>
        </div>

        {/* Alert summary bar */}
        {d.alerts.length > 0 && (
          <div style={{ margin:'12px 36px 0', padding:'10px 16px', background:T.surface, border:`1px solid ${T.line2}`, borderRadius:10, display:'flex', alignItems:'center', gap:10 }}>
            <AlertTriangle size={14} color={d.totalUnhealthy>0?H.unhealthy:H.degraded} strokeWidth={1.5} style={{ flexShrink:0 }} />
            <span style={{ fontSize:12.5, color:T.ink, fontWeight:500 }}>{d.alerts.length} issues detected</span>
            <span style={{ fontSize:11.5, color:T.ink3 }}>
              {d.totalUnhealthy>0 && `${d.totalUnhealthy} unhealthy`}
              {d.totalUnhealthy>0 && d.totalDegraded>0 && ' · '}
              {d.totalDegraded>0  && `${d.totalDegraded} degraded`}
            </span>
          </div>
        )}

        {/* Topology tiers */}
        <div style={{ padding:'28px 36px 40px', display:'flex', flexDirection:'column', gap:28 }}>
          {tiers.map((tier:any) => (
            <div key={tier.id}>
              <div style={{ display:'flex', alignItems:'baseline', gap:10, padding:'0 2px', marginBottom:12 }}>
                <span style={{ fontSize:11, color:T.ink3, textTransform:'uppercase', letterSpacing:'0.08em', fontWeight:500 }}>{tier.label}</span>
                <span style={{ fontFamily:MONO, fontSize:11, color:T.ink4 }}>{tier.count}</span>
                <span style={{ flex:1, height:1, background:T.line }} />
              </div>
              <div style={{ display:'grid', gridTemplateColumns:`repeat(${Math.min(5,Math.max(2,tier.tiles.length))},1fr)`, gap:1, background:T.line, border:`1px solid ${T.line}`, borderRadius:10, overflow:'hidden' }}>
                {tier.tiles.map((tile:any) => (
                  <TileCard key={tile.id} tile={tile} selected={selectedId===tile.id} onClick={() => tile.id===selectedId ? closeAll() : handleTileClick(tile.id)} />
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* ── GroupDrawer — node list for selected tile ── */}
      {selectedId && groupInfo && !selectedNode && (
        <GroupDrawer
          group={groupInfo}
          vmCode={vmCode}
          onClose={closeAll}
          onSelectNode={id => {
            const n = detailNodes.find(n => n.id === id)
            if (n) { setSelectedNode(n); setShowLogs(false); setShowTerminal(false) }
          }}
        />
      )}

      {/* ── NodeDetailPanel ── */}
      {selectedNode && (
        <NodeDetailPanel
          node={selectedNode}
          vmCode={vmCode}
          onClose={() => setSelectedNode(null)}
          onShowLogs={['container','pod','host'].includes(selectedNode.type) ? () => { setShowLogs(true); setShowTerminal(false) } : undefined}
          onShowTerminal={['container','host','pod'].includes(selectedNode.type) ? () => {
            setTerminalLayer(selectedNode.type === 'host' ? 'host' : selectedNode.type === 'pod' ? 'kubernetes' : 'docker')
            setShowTerminal(true)
            setShowLogs(false)
          } : undefined}
        />
      )}

      {/* ── Logs panel (per-node) ── */}
      {selectedNode && showLogs && !showTerminal && (
        <LogsPanel node={selectedNode} vmCode={vmCode} onClose={() => setShowLogs(false)} />
      )}

      {/* ── Terminal panel (per-node) ── */}
      {selectedNode && showTerminal && !showLogs && (
        <TerminalPanel node={selectedNode} vmCode={vmCode} layer={terminalLayer} onClose={() => setShowTerminal(false)} />
      )}

      {/* ── Host terminal (top-level button) ── */}
      {showHostTerminal && d.host && (
        <TerminalPanel node={d.host} vmCode={vmCode} layer="host" onClose={() => setShowHostTerminal(false)} />
      )}

      <style>{`@keyframes pulse{0%,100%{opacity:1}50%{opacity:.4}} @keyframes slideIn{from{opacity:0;transform:translateX(8px)}to{opacity:1;transform:none}}`}</style>
    </div>
  )
}

/* ── Tile card ── */
function TileCard({ tile, selected, onClick }: { tile:any; selected:boolean; onClick:()=>void }) {
  const total = tile.health ? Math.max(tile.health.h+tile.health.d+tile.health.u,1) : 1
  const isAlert = tile.state==='alert'
  const isWarn  = tile.state==='warn'
  return (
    <div onClick={onClick} style={{ background:selected?T.surface2:T.bg, padding:'14px 16px', cursor:'pointer', display:'flex', flexDirection:'column', gap:10, transition:'background 0.12s', position:'relative', borderLeft:selected?`2px solid ${T.ink}`:'2px solid transparent' }}
      onMouseEnter={e=>{ if(!selected)(e.currentTarget as HTMLDivElement).style.background=T.surface }}
      onMouseLeave={e=>{ if(!selected)(e.currentTarget as HTMLDivElement).style.background=T.bg }}
    >
      <div style={{ display:'flex', alignItems:'flex-start', gap:10 }}>
        <div style={{ width:30, height:30, borderRadius:7, background:T.surface, border:`1px solid ${T.line2}`, display:'flex', alignItems:'center', justifyContent:'center', flexShrink:0, marginTop:1 }}>
          <TileIcon icon={tile.icon} />
        </div>
        <div style={{ minWidth:0, flex:1 }}>
          <div style={{ fontFamily:MONO, fontSize:10, color:T.ink3, textTransform:'uppercase', letterSpacing:'0.04em', marginBottom:3 }}>{tile.typeLabel}</div>
          <div style={{ fontSize:14, fontWeight:500, letterSpacing:'-0.015em', color:T.ink }}>
            {tile.name}
            {(isAlert||isWarn) && <span style={{ display:'inline-block', width:5, height:5, marginLeft:7, marginBottom:2, borderRadius:'50%', background:isAlert?H.unhealthy:H.degraded, verticalAlign:'middle', ...(isAlert?{animation:'pulse 1.6s ease-in-out infinite'}:{}) }} />}
          </div>
        </div>
        <span style={{ fontFamily:MONO, fontSize:11.5, color:T.ink2, background:T.surface, border:`1px solid ${T.line2}`, padding:'1px 8px', borderRadius:999, flexShrink:0 }}>{tile.count}</span>
      </div>

      {tile.health && (
        <div style={{ display:'flex', height:2, borderRadius:1, overflow:'hidden', background:T.line }}>
          {tile.health.h>0 && <div style={{ width:`${(tile.health.h/total)*100}%`, height:'100%', background:H.healthy }} />}
          {tile.health.d>0 && <div style={{ width:`${(tile.health.d/total)*100}%`, height:'100%', background:H.degraded }} />}
          {tile.health.u>0 && <div style={{ width:`${(tile.health.u/total)*100}%`, height:'100%', background:H.unhealthy }} />}
        </div>
      )}

      <div style={{ display:'flex', alignItems:'center', gap:16, fontSize:12, color:T.ink3, fontFamily:MONO }}>
        {tile.stats.map(([k,v]:[string,string],i:number) => (
          <span key={i} style={{ display:'flex', alignItems:'center', gap:5 }}>
            <span>{k}</span><b style={{ color:T.ink2, fontWeight:500 }}>{v}</b>
          </span>
        ))}
        <span style={{ marginLeft:'auto', color:T.ink4, fontSize:11 }}>→</span>
      </div>
    </div>
  )
}

const TILE_ICON_TYPE: Record<string, string> = {
  'k8s': 'cluster', 'docker': 'container_runtime', 'host': 'host',
  'k8s-node': 'node', 'namespace': 'namespace', 'deployment': 'deployment',
  'statefulset': 'statefulset', 'daemonset': 'daemonset', 'service': 'k8s_service',
  'ingress': 'ingress', 'pod': 'pod', 'job': 'job', 'cronjob': 'cronjob', 'volume': 'volume',
  'sysservice': 'service', 'process': 'process',
}

function TileIcon({ icon }: { icon: string }) {
  return <NodeSvgIcon type={TILE_ICON_TYPE[icon] ?? icon} size={18} />
}

function DotSep() {
  return <span style={{ width:2, height:2, borderRadius:'50%', background:T.ink4, display:'inline-block' }} />
}

const actionBtn: React.CSSProperties = {
  display:'inline-flex', alignItems:'center', gap:6,
  height:28, padding:'0 10px', borderRadius:6,
  border:`1px solid var(--line2)`, background:'var(--surface)',
  color:'var(--ink2)', fontSize:12, cursor:'pointer', fontFamily:"var(--font-geist,'Geist',ui-sans-serif,system-ui,sans-serif)",
}
