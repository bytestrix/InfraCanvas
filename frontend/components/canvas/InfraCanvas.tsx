'use client'

import { useEffect, useMemo, useState, useCallback, useRef } from 'react'
import ReactFlow, {
  Background,
  BackgroundVariant,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  type NodeTypes,
  type ReactFlowInstance,
  MarkerType,
} from 'reactflow'
import 'reactflow/dist/style.css'

import {
  type VMState,
  type GraphNode,
  type GraphEdge,
  getNodeColor,
} from '@/types'
import { applyRootedDagreLayout, applyGroupedZoneLayout } from '@/lib/layout'
import { sendCommand } from '@/lib/wsManager'
import {
  buildGroupedGraph,
  type GroupInfo,
  type GroupNodeData,
  isCriticalGroup,
} from '@/lib/graphPreprocess'
import InfraNode, { type InfraNodeData } from './InfraNode'
import NamespaceGroupNode from './NamespaceGroupNode'
import GroupNode from './GroupNode'
import GroupDrawer from './GroupDrawer'
import NodeDetailPanel from './NodeDetailPanel'
import {
  ArrowLeft,
  RefreshCw,
  Clock,
  GitBranch,
  Layers,
  Filter,
  Rows3,
  Network,
  Server,
  AlertTriangle,
} from 'lucide-react'

// ─── Filter groups ────────────────────────────────────────────────────────────

const FILTER_GROUPS = {
  k8s: {
    label: 'Kubernetes',
    color: '#6366f1',
    types: [
      'cluster', 'node', 'namespace',
      'deployment', 'statefulset', 'daemonset', 'job', 'cronjob',
      'k8s_service', 'ingress',
    ],
  },
  pods: {
    label: 'Pods',
    color: '#22d3ee',
    types: ['pod'],
  },
  docker: {
    label: 'Docker',
    color: '#10b981',
    types: ['container', 'container_runtime', 'image', 'image_group', 'volume', 'network'],
  },
  host: {
    label: 'Host',
    color: '#64748b',
    types: ['host'],
  },
  storage: {
    label: 'Storage',
    color: '#f59e0b',
    types: ['pvc'],
  },
  events: {
    label: 'Events',
    color: '#ef4444',
    types: ['event'],
  },
} as const

type FilterKey = keyof typeof FILTER_GROUPS

// ─── Flat mode builder ────────────────────────────────────────────────────────

function buildFlatFlowElements(
  graphNodes: GraphNode[],
  graphEdges: GraphEdge[],
  activeFilters: Set<FilterKey>,
) {
  const visibleTypes = new Set<string>()
  for (const key of activeFilters) {
    for (const t of FILTER_GROUPS[key].types) visibleTypes.add(t)
  }
  const filtered = graphNodes.filter((n) => visibleTypes.has(n.type))
  const visibleIds = new Set(filtered.map((n) => n.id))

  const nodes: Node<InfraNodeData>[] = filtered.map((n) => ({
    id: n.id,
    type: n.type === 'namespace' ? 'namespaceGroup' : 'infraNode',
    position: { x: 0, y: 0 },
    data: { nodeType: n.type, label: n.label, health: n.health, metadata: n.metadata },
    width: 220,
    height: 100,
  }))

  const edges: Edge[] = graphEdges
    .filter((e) => visibleIds.has(e.source) && visibleIds.has(e.target))
    .map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      label: e.type,
      type: 'smoothstep',
      markerEnd: { type: MarkerType.ArrowClosed, width: 8, height: 8, color: '#2d2d52' },
      style: { stroke: '#2d2d52', strokeWidth: 1.5 },
      labelStyle: { fill: '#475569', fontSize: 9, fontFamily: 'JetBrains Mono, monospace' },
      labelBgStyle: { fill: '#070711', fillOpacity: 0.9 },
    }))

  return { nodes, edges }
}

// ─── Node types registry ──────────────────────────────────────────────────────

const nodeTypes: NodeTypes = {
  infraNode: InfraNode,
  namespaceGroup: NamespaceGroupNode,
  groupNode: GroupNode,
}

// ─── Main component ───────────────────────────────────────────────────────────

interface InfraCanvasProps {
  vm: VMState
  onBack: () => void
}

export default function InfraCanvas({ vm, onBack }: InfraCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState([])
  const [edges, setEdges, onEdgesChange] = useEdgesState([])

  const [viewMode, setViewMode] = useState<'grouped' | 'flat'>('grouped')
  const [activeFilters, setActiveFilters] = useState<Set<FilterKey>>(
    new Set<FilterKey>(['k8s', 'docker', 'host'])
  )
  const [expandedGroups] = useState<Set<string>>(new Set())
  const [drawerGroup, setDrawerGroup] = useState<GroupInfo | null>(null)
  const [groupsMap, setGroupsMap] = useState<Map<string, GroupInfo>>(new Map())
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [isRefreshing, setIsRefreshing] = useState(false)

  // Spotlight: which filter key to dim everything else for (null = no spotlight)
  const [spotlightKey, setSpotlightKey] = useState<FilterKey | null>(null)

  // ReactFlow instance — for programmatic pan/zoom
  const rfRef = useRef<ReactFlowInstance | null>(null)

  const selectedNode = useMemo(() => {
    if (!selectedNodeId || !vm.graph) return null
    return vm.graph.nodes.find((n) => n.id === selectedNodeId) ?? null
  }, [selectedNodeId, vm.graph])

  // ── Build + layout whenever graph/filters/viewMode changes ────────────────
  useEffect(() => {
    if (!vm.graph) return

    const visibleTypes = new Set<string>()
    for (const key of activeFilters) {
      for (const t of FILTER_GROUPS[key].types) visibleTypes.add(t)
    }

    let flowNodes: Node[]
    let flowEdges: Edge[]

    if (viewMode === 'grouped') {
      const { nodes: gn, edges: ge, groups } = buildGroupedGraph(
        vm.graph.nodes,
        vm.graph.edges,
        visibleTypes,
        expandedGroups,
      )
      flowNodes = gn
      flowEdges = ge
      setGroupsMap(groups)
    } else {
      const { nodes: fn, edges: fe } = buildFlatFlowElements(
        vm.graph.nodes,
        vm.graph.edges,
        activeFilters,
      )
      flowNodes = fn
      flowEdges = fe
      setGroupsMap(new Map())
    }

    if (flowNodes.length === 0) {
      setNodes([])
      setEdges([])
      return
    }

    // Grouped mode: fixed zone positions — no layout algorithm needed
    // Flat mode: rooted dagre for proper hierarchy
    let ln: Node[]
    let le: Edge[]
    if (viewMode === 'grouped') {
      ln = applyGroupedZoneLayout(flowNodes)
      le = flowEdges
    } else {
      const laid = applyRootedDagreLayout(flowNodes, flowEdges, { rankdir: 'TB', ranksep: 120, nodesep: 70 })
      ln = laid.nodes
      le = laid.edges
    }

    // Apply spotlight opacity if active
    const finalNodes = applySpotlight(ln, spotlightKey, viewMode)
    setNodes(finalNodes)
    setEdges(le)
  }, [vm.graph, activeFilters, viewMode, expandedGroups])

  // ── Re-apply spotlight without re-running layout ───────────────────────────
  useEffect(() => {
    setNodes((prev) => applySpotlight(prev, spotlightKey, viewMode))
  }, [spotlightKey, viewMode])

  // ── Spotlight helpers ──────────────────────────────────────────────────────

  function applySpotlight(
    allNodes: Node[],
    key: FilterKey | null,
    mode: 'grouped' | 'flat',
  ): Node[] {
    if (!key) return allNodes.map((n) => ({ ...n, style: { ...n.style, opacity: 1 } }))

    const spotTypes = new Set<string>(FILTER_GROUPS[key].types)

    return allNodes.map((n) => {
      const nodeType = n.type === 'groupNode'
        ? (n.data as GroupNodeData).groupType
        : n.type === 'namespaceGroup'
          ? 'namespace'
          : (n.data as InfraNodeData)?.nodeType ?? ''
      const isSpotlit = spotTypes.has(nodeType)
      return { ...n, style: { ...n.style, opacity: isSpotlit ? 1 : 0.12 } }
    })
  }

  // ── Filter toggle — first click activates, second spotlights, third deactivates
  function handleFilterClick(key: FilterKey) {
    if (!activeFilters.has(key)) {
      // Activate it
      setActiveFilters((prev) => new Set([...prev, key]))
      setSpotlightKey(null)
      return
    }
    if (spotlightKey === key) {
      // Already spotlit → clear spotlight
      setSpotlightKey(null)
    } else {
      // Active but not spotlit → spotlight it
      setSpotlightKey(key)
      // Zoom to those nodes
      zoomToFilterGroup(key)
    }
  }

  function handleFilterRightClick(e: React.MouseEvent, key: FilterKey) {
    e.preventDefault()
    // Right-click = deactivate (if more than 1 active)
    setActiveFilters((prev) => {
      const next = new Set(prev)
      if (next.size > 1) next.delete(key)
      return next
    })
    if (spotlightKey === key) setSpotlightKey(null)
  }

  function zoomToFilterGroup(key: FilterKey) {
    if (!rfRef.current) return
    const types = new Set<string>(FILTER_GROUPS[key].types)
    const matchIds = nodes
      .filter((n) => {
        const t = n.type === 'groupNode'
          ? (n.data as GroupNodeData).groupType
          : (n.data as InfraNodeData)?.nodeType ?? ''
        return types.has(t)
      })
      .map((n) => n.id)
    if (matchIds.length === 0) return
    rfRef.current.fitView({ nodes: matchIds.map((id) => ({ id })), duration: 500, padding: 0.3 })
  }

  function handleRefresh() {
    setIsRefreshing(true)
    sendCommand(vm.code, 'refresh')
    setTimeout(() => setIsRefreshing(false), 2000)
  }

  const handleNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    if (node.type === 'groupNode') {
      const gdata = node.data as GroupNodeData
      const group = groupsMap.get(gdata.groupType)
      if (group) {
        setSelectedNodeId(null)
        setDrawerGroup(group)
      }
    } else {
      setDrawerGroup(null)
      setSelectedNodeId(node.id)
    }
  }, [groupsMap])

  function handleSelectNodeFromDrawer(nodeId: string) {
    // Find which group contains this raw node, zoom to its group node on canvas
    const groupEntry = [...groupsMap.entries()].find(
      ([, g]) => g.nodes.some((n) => n.id === nodeId)
    )
    if (groupEntry && rfRef.current) {
      const groupCanvasId = `group:${groupEntry[0]}`
      const rfNode = rfRef.current.getNode(groupCanvasId)
      if (rfNode) {
        rfRef.current.setCenter(
          rfNode.position.x + (rfNode.width ?? 260) / 2,
          rfNode.position.y + (rfNode.height ?? 92) / 2,
          { zoom: 1.8, duration: 600 },
        )
      }
    }
    setDrawerGroup(null)
    setSelectedNodeId(nodeId)
  }

  function handlePaneClick() {
    setSelectedNodeId(null)
    setDrawerGroup(null)
    setSpotlightKey(null)
  }

  // ── Critical alert banner ──────────────────────────────────────────────────
  const criticalGroups = useMemo(() => {
    const alerts: Array<{ label: string; degraded: number; type: string }> = []
    for (const [, g] of groupsMap) {
      if (isCriticalGroup(g.healthCounts, g.count)) {
        alerts.push({
          label: g.label,
          degraded: g.healthCounts.degraded + g.healthCounts.unhealthy,
          type: g.type,
        })
      }
    }
    return alerts
  }, [groupsMap])

  const stats = vm.graph?.stats
  const snapshot = vm.graph?.snapshot

  function fmt(s: number) {
    if (!s) return '—'
    return s < 1 ? `${Math.round(s * 1000)}ms` : `${s.toFixed(2)}s`
  }

  return (
    <div style={{ width: '100vw', height: '100vh', display: 'flex', flexDirection: 'column', background: '#070711', overflow: 'hidden' }}>

      {/* ── Top bar ──────────────────────────────────────────────── */}
      <div style={{
        flexShrink: 0, display: 'flex', alignItems: 'center', gap: 10,
        padding: '0 14px', height: 50, zIndex: 10,
        background: 'rgba(7,7,17,0.97)', borderBottom: '1px solid #1e1e3a',
        backdropFilter: 'blur(8px)',
      }}>
        <button onClick={onBack} style={ICON_BTN}
          onMouseEnter={(e) => { e.currentTarget.style.background = '#13131f'; e.currentTarget.style.color = '#94a3b8' }}
          onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = '#475569' }}>
          <ArrowLeft size={15} />
        </button>

        <div style={DIVIDER} />

        <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
          <Server size={13} color="#6366f1" />
          <span style={{ fontSize: 13, fontWeight: 600, color: '#e2e8f0' }}>{vm.hostname ?? vm.code}</span>
          <span style={{ fontSize: 10, padding: '1px 8px', borderRadius: 20, background: 'rgba(99,102,241,0.1)', color: '#818cf8', border: '1px solid rgba(99,102,241,0.15)', fontFamily: 'monospace' }}>
            {vm.code}
          </span>
        </div>

        {stats && (
          <>
            <div style={DIVIDER} />
            <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: '#64748b' }}>
              <GitBranch size={11} /><span style={{ color: '#94a3b8' }}>{stats.totalNodes}</span> nodes
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: '#64748b' }}>
              <Layers size={11} /><span style={{ color: '#94a3b8' }}>{stats.totalEdges}</span> edges
            </span>
            {snapshot?.collectionDuration != null && (
              <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: '#64748b' }}>
                <Clock size={11} /><span style={{ color: '#94a3b8' }}>{fmt(snapshot.collectionDuration)}</span>
              </span>
            )}
          </>
        )}

        <div style={{ flex: 1 }} />

        {/* Spotlight hint */}
        {spotlightKey && (
          <span style={{ fontSize: 10, color: '#f59e0b', display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#f59e0b', display: 'inline-block' }} />
            Spotlight: {FILTER_GROUPS[spotlightKey].label}
            <button onClick={() => setSpotlightKey(null)} style={{ background: 'none', border: 'none', color: '#f59e0b', cursor: 'pointer', padding: 0, marginLeft: 2, fontSize: 11 }}>✕</button>
          </span>
        )}

        {/* View mode toggle */}
        <div style={{ display: 'flex', gap: 1, background: '#0e0e1a', border: '1px solid #1e1e3a', borderRadius: 7, padding: 2 }}>
          {(['grouped', 'flat'] as const).map((m) => (
            <button key={m} onClick={() => setViewMode(m)}
              title={m === 'grouped' ? 'Grouped — one card per type' : 'Flat — every node'}
              style={{
                display: 'flex', alignItems: 'center', gap: 4,
                padding: '3px 9px', borderRadius: 5, border: 'none', cursor: 'pointer',
                fontSize: 11, fontWeight: viewMode === m ? 600 : 400,
                background: viewMode === m ? '#1e1e3a' : 'transparent',
                color: viewMode === m ? '#e2e8f0' : '#475569',
              }}>
              {m === 'grouped' ? <Rows3 size={11} /> : <Network size={11} />}
              {m === 'grouped' ? 'Grouped' : 'Flat'}
            </button>
          ))}
        </div>

        <div style={DIVIDER} />

        {/* Filter chips — click to spotlight, right-click to deactivate */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <Filter size={11} color="#334155" />
          {(['k8s', 'docker', 'host'] as FilterKey[]).map((key) => {
            const g = FILTER_GROUPS[key]
            const active = activeFilters.has(key)
            const spotlit = spotlightKey === key
            return (
              <button key={key}
                onClick={() => handleFilterClick(key)}
                onContextMenu={(e) => handleFilterRightClick(e, key)}
                title={active ? 'Click to spotlight · right-click to hide' : 'Click to show'}
                style={{
                  padding: '3px 9px', borderRadius: 6, fontSize: 11, cursor: 'pointer',
                  fontWeight: active ? 600 : 400,
                  background: spotlit ? g.color : active ? `${g.color}18` : '#0e0e1a',
                  color: spotlit ? '#fff' : active ? g.color : '#475569',
                  border: `1px solid ${spotlit ? g.color : active ? `${g.color}35` : '#1e1e3a'}`,
                  transition: 'all 0.15s',
                }}>
                {g.label}
              </button>
            )
          })}
          <span style={{ width: 1, height: 13, background: '#1e1e3a', display: 'inline-block', margin: '0 1px' }} />
          {(['pods', 'storage', 'events'] as FilterKey[]).map((key) => {
            const g = FILTER_GROUPS[key]
            const active = activeFilters.has(key)
            const spotlit = spotlightKey === key
            return (
              <button key={key}
                onClick={() => handleFilterClick(key)}
                onContextMenu={(e) => handleFilterRightClick(e, key)}
                title={active ? 'Click to spotlight · right-click to hide' : 'Click to show'}
                style={{
                  padding: '3px 9px', borderRadius: 6, fontSize: 11, cursor: 'pointer',
                  fontWeight: active ? 600 : 400,
                  background: spotlit ? g.color : active ? `${g.color}18` : 'transparent',
                  color: spotlit ? '#fff' : active ? g.color : '#334155',
                  border: `1px dashed ${spotlit ? g.color : active ? `${g.color}35` : '#1e1e3a'}`,
                  transition: 'all 0.15s',
                }}>
                {g.label}
              </button>
            )
          })}
        </div>

        <div style={DIVIDER} />

        <button onClick={handleRefresh}
          style={{ display: 'flex', alignItems: 'center', gap: 5, padding: '4px 10px', borderRadius: 7, border: '1px solid #1e1e3a', background: '#0e0e1a', color: '#94a3b8', fontSize: 11, cursor: 'pointer' }}
          onMouseEnter={(e) => { e.currentTarget.style.borderColor = '#2d2d52'; e.currentTarget.style.color = '#e2e8f0' }}
          onMouseLeave={(e) => { e.currentTarget.style.borderColor = '#1e1e3a'; e.currentTarget.style.color = '#94a3b8' }}>
          <RefreshCw size={12} className={isRefreshing ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      {/* ── Critical alert banner ─────────────────────────────────── */}
      {criticalGroups.length > 0 && (
        <div style={{
          flexShrink: 0,
          background: 'rgba(239,68,68,0.07)',
          borderBottom: '1px solid rgba(239,68,68,0.2)',
          padding: '5px 16px',
          display: 'flex',
          alignItems: 'center',
          gap: 12,
        }}>
          <AlertTriangle size={13} color="#ef4444" />
          <div style={{ display: 'flex', gap: 14, flexWrap: 'wrap' }}>
            {criticalGroups.map((a) => (
              <button
                key={a.type}
                onClick={() => {
                  const g = groupsMap.get(a.type)
                  if (g) { setDrawerGroup(g); setSelectedNodeId(null) }
                }}
                style={{
                  background: 'none', border: 'none', cursor: 'pointer', padding: 0,
                  fontSize: 11, color: '#fca5a5', display: 'flex', alignItems: 'center', gap: 5,
                }}
              >
                ⚠ {a.label}: <strong>{a.degraded} degraded</strong> — click to inspect
              </button>
            ))}
          </div>
        </div>
      )}

      {/* ── Canvas ───────────────────────────────────────────────── */}
      <div style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
        {!vm.graph ? (
          <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12 }}>
              <div style={{ width: 34, height: 34, borderRadius: 9, border: '2px solid #6366f1', borderTopColor: 'transparent' }} className="animate-spin" />
              <p style={{ fontSize: 13, color: '#475569' }}>Loading infrastructure graph…</p>
            </div>
          </div>
        ) : nodes.length === 0 ? (
          <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <div style={{ textAlign: 'center' }}>
              <p style={{ fontSize: 16, fontWeight: 600, color: '#e2e8f0', marginBottom: 6 }}>No nodes to display</p>
              <p style={{ fontSize: 13, color: '#475569' }}>Try enabling more filter categories</p>
            </div>
          </div>
        ) : (
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onNodeClick={handleNodeClick}
            onPaneClick={handlePaneClick}
            nodeTypes={nodeTypes}
            onInit={(instance) => { rfRef.current = instance }}
            fitView
            fitViewOptions={{ padding: 0.15 }}
            minZoom={0.04}
            maxZoom={2.5}
            proOptions={{ hideAttribution: true }}
          >
            <Background variant={BackgroundVariant.Dots} gap={22} size={1} color="#1a1a2e" />
            <Controls style={{ bottom: 80, left: 16 }} showInteractive={false} />
            <MiniMap
              nodeColor={(n) => {
                if (n.type === 'groupNode') return getNodeColor((n.data as GroupNodeData).groupType)
                return getNodeColor((n.data as InfraNodeData)?.nodeType, (n.data as InfraNodeData)?.health)
              }}
              maskColor="rgba(99,102,241,0.07)"
              style={{ bottom: 16, right: 16, width: 160, height: 100 }}
            />
          </ReactFlow>
        )}

        {drawerGroup && (
          <GroupDrawer
            group={drawerGroup}
            onClose={() => setDrawerGroup(null)}
            onSelectNode={handleSelectNodeFromDrawer}
          />
        )}

        {selectedNode && !drawerGroup && (
          <NodeDetailPanel node={selectedNode} vmCode={vm.code} onClose={() => setSelectedNodeId(null)} />
        )}
      </div>

      {/* ── Status bar ───────────────────────────────────────────── */}
      <div style={{
        flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '0 14px', height: 24, background: '#070711', borderTop: '1px solid #0f0f1e',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
          {viewMode === 'grouped' && groupsMap.size > 0 && (
            <span style={{ fontSize: 10, color: '#1e1e3a' }}>
              {groupsMap.size} groups · click card to drill down · right-click filter to hide
            </span>
          )}
          {stats && Object.entries(stats.nodesByType)
            .sort(([, a], [, b]) => b - a).slice(0, 4)
            .map(([type, count]) => (
              <span key={type} style={{ display: 'flex', alignItems: 'center', gap: 3, fontSize: 10, color: '#1e1e3a' }}>
                <span style={{ color: getNodeColor(type) }}>●</span>
                {type}:{count}
              </span>
            ))}
        </div>
        {snapshot?.timestamp && (
          <span style={{ fontSize: 10, color: '#1a1a2e' }}>
            {new Date(snapshot.timestamp).toLocaleTimeString()}
          </span>
        )}
      </div>

    </div>
  )
}

// ─── Micro-styles ─────────────────────────────────────────────────────────────

const ICON_BTN: React.CSSProperties = {
  width: 28, height: 28, borderRadius: 6, border: 'none',
  background: 'transparent', color: '#475569', cursor: 'pointer',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
}

const DIVIDER: React.CSSProperties = {
  width: 1, height: 16, background: '#1e1e3a', flexShrink: 0,
}
