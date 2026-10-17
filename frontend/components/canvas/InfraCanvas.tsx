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
import { applyRootedDagreLayout, applyNamespaceHierarchyLayout } from '@/lib/layout'
import { MAP_NODE_WIDTH, MAP_NODE_HEIGHT } from '@/lib/radialMap'
import { applyStructuredMapLayout } from '@/lib/structuredMap'
import MapNode from './MapNode'
import MapLabel from './MapLabel'
import { sendCommand } from '@/lib/wsManager'
import {
  buildGroupedGraph,
  type GroupInfo,
  type GroupNodeData,
  isCriticalGroup,
  countHealth,
  getGroupLabel,
  SINGLETON_TYPES,
} from '@/lib/graphPreprocess'
import InfraNode, { type InfraNodeData } from './InfraNode'
import NamespaceGroupNode from './NamespaceGroupNode'
import GroupNode from './GroupNode'
import GroupDrawer from './GroupDrawer'
import NodePickerMenu from './NodePickerMenu'
import NodeDetailPanel from './NodeDetailPanel'
import LogsPanel from './LogsPanel'
import TerminalPanel from './TerminalPanel'
import {
  ArrowLeft,
  RefreshCw,
  Clock,
  GitBranch,
  Layers,
  Filter,
  Rows3,
  Network,
  Waypoints,
  Server,
  AlertTriangle,
  Download,
  Search,
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
      // Pods live in the Kubernetes category, not a separate one. Splitting
      // them out meant the one chip a Kubernetes user clicks ("Kubernetes")
      // silently excluded the resource they were looking for.
      'pod',
    ],
  },
  docker: {
    label: 'Docker',
    color: '#22c55e',
    types: ['container', 'container_runtime', 'image', 'image_group', 'volume', 'network'],
  },
  host: {
    label: 'Host',
    color: '#64748b',
    types: ['host'],
  },
  services: {
    label: 'Services',
    color: '#a855f7',
    types: ['service', 'process'],
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

/** Toolbar order. `i === 4` draws the divider after Services. */
const TOOLBAR_ORDER = ['k8s', 'docker', 'host', 'services', 'storage', 'events'] as FilterKey[]

/** category-summary node id, distinct from a type-level `group:${type}` id */
function categoryNodeId(key: FilterKey) {
  return `category:${key}`
}

/** raw type → its FilterKey category, e.g. 'deployment' → 'k8s' */
const TYPE_TO_CATEGORY: Record<string, FilterKey> = (() => {
  const map: Record<string, FilterKey> = {}
  for (const key of Object.keys(FILTER_GROUPS) as FilterKey[]) {
    for (const t of FILTER_GROUPS[key].types) map[t] = key
  }
  return map
})()

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
      markerEnd: { type: MarkerType.ArrowClosed, width: 8, height: 8, color: 'var(--line2)' },
      style: { stroke: 'var(--line2)', strokeWidth: 1.5 },
      labelStyle: { fill: 'var(--ink3)', fontSize: 9, fontFamily: 'Geist Mono, JetBrains Mono, monospace' },
      labelBgStyle: { fill: 'var(--bg)', fillOpacity: 0.9 },
    }))

  return { nodes, edges }
}

// ─── Node types registry ──────────────────────────────────────────────────────

const nodeTypes: NodeTypes = {
  infraNode: InfraNode,
  namespaceGroup: NamespaceGroupNode,
  groupNode: GroupNode,
  mapNode: MapNode,
  mapLabel: MapLabel,
}

// ─── Main component ───────────────────────────────────────────────────────────

interface InfraCanvasProps {
  vm: VMState
  onBack?: () => void
}

export default function InfraCanvas({ vm, onBack }: InfraCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState([])
  const [edges, setEdges, onEdgesChange] = useEdgesState([])

  const [viewMode, setViewMode] = useState<'map' | 'grouped' | 'flat'>('map')
  // Toolbar chips (Kubernetes / Docker / Host / ...) — unchanged, existing
  // behaviour: toggling one on immediately shows every type in that category.
  // Kept separate from the "•••" picker tree below on purpose.
  // Default to 'host' for Machines (VM agents always report one), but
  // Clusters connections (kubeconfig direct-connect) report no host node at
  // all — defaulting to 'host' there hides everything on first load ("No
  // nodes to display"). Fall back to 'k8s' when there's no host.
  const [activeFilters, setActiveFilters] = useState<Set<FilterKey>>(() => {
    const hasHost = vm.graph?.nodes?.some((n) => n.type === 'host')
    return new Set<FilterKey>([hasHost ? 'host' : 'k8s'])
  })
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set())

  // Live node count per filter category, for the chip badges.
  const filterCounts = useMemo(() => {
    const source = vm.graph?.nodes ?? []
    const out = {} as Record<FilterKey, number>
    for (const key of Object.keys(FILTER_GROUPS) as FilterKey[]) {
      const types = new Set<string>(FILTER_GROUPS[key].types as readonly string[])
      out[key] = source.reduce((sum, n) => (types.has(n.type) ? sum + 1 : sum), 0)
    }
    return out
  }, [vm.graph])

  // ── "•••" drill-down tree: host → category → type → individual ────────────
  // Tier 1: categories with a single summary node on canvas (e.g. "Kubernetes
  // ×82"), revealed from the host's "•••" menu. Does NOT explode into types.
  const [revealedCategories, setRevealedCategories] = useState<Set<FilterKey>>(new Set())
  // Tier 2: raw types pulled out of a category's summary into their own
  // type-level group node (e.g. "Deployments ×17"), via that summary's "•••".
  const [revealedTypes, setRevealedTypes] = useState<Set<string>>(new Set())
  // Tier 3: specific raw node IDs pulled out of a type-level group node
  // individually via its "•••" (pick 2 of 11 pods → only those 2 pop out).
  const [expandedNodeIds, setExpandedNodeIds] = useState<Set<string>>(new Set())
  // The open "•••" picker popup, if any.
  const [pickerFor, setPickerFor] = useState<
    | { kind: 'host'; x: number; y: number }
    | { kind: 'category'; category: FilterKey; x: number; y: number }
    | { kind: 'group'; groupType: string; groupKey: string; x: number; y: number }
    | null
  >(null)
  const [drawerGroup, setDrawerGroup] = useState<GroupInfo | null>(null)
  const [drawerInitialFilter, setDrawerInitialFilter] = useState<string | undefined>(undefined)
  const [groupsMap, setGroupsMap] = useState<Map<string, GroupInfo>>(new Map())
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [showLogs, setShowLogs] = useState(false)
  const [showTerminal, setShowTerminal] = useState(false)
  const [terminalLayer, setTerminalLayer] = useState<'docker' | 'lxd' | 'host' | 'kubernetes'>('docker')
  const canvasWrapRef = useRef<HTMLDivElement>(null)

  const [spotlightKey, setSpotlightKey] = useState<FilterKey | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchFocused, setSearchFocused] = useState(false)
  // Node awaiting a pan-to, set when a search reveals something currently
  // hidden by a filter. Consumed by the effect below once it renders.
  const pendingFocusId = useRef<string | null>(null)

  const rfRef = useRef<ReactFlowInstance | null>(null)

  const selectedNode = useMemo(() => {
    if (!selectedNodeId || !vm.graph) return null
    return vm.graph.nodes.find((n) => n.id === selectedNodeId) ?? null
  }, [selectedNodeId, vm.graph])

  // ── Build + layout whenever graph/filters/viewMode changes ────────────────
  useEffect(() => {
    if (!vm.graph) return

    // Toolbar-driven types (unchanged, existing behaviour) + types drilled
    // into individually via the "•••" tree. Types still folded inside an
    // unexpanded category summary are deliberately excluded here — they're
    // rendered as ONE summary node below, not as their own type groups.
    const toolbarTypes = new Set<string>()
    for (const key of activeFilters) {
      for (const t of FILTER_GROUPS[key].types) toolbarTypes.add(t)
    }
    const visibleTypes = new Set<string>(['host', ...toolbarTypes, ...revealedTypes])

    let flowNodes: Node[]
    let flowEdges: Edge[]

    if (viewMode === 'grouped') {
      const { nodes: gn, edges: ge, groups } = buildGroupedGraph(
        vm.graph.nodes,
        vm.graph.edges,
        visibleTypes,
        expandedGroups,
        expandedNodeIds,
      )

      // Category-summary nodes: one per revealed-but-not-fully-drilled
      // category (e.g. "Kubernetes ×82"), sitting between host and the
      // type-level groups that appear once you drill into that summary.
      const hostNode = vm.graph.nodes.find((n) => n.type === 'host')
      // Clusters connections (kubeconfig direct-connect) report no host node —
      // fall back to the cluster node as the canvas root anchor so category
      // summaries and orphans still attach to something instead of floating.
      const rootAnchor = hostNode ?? vm.graph.nodes.find((n) => n.type === 'cluster')
      const categoryNodes: Node[] = []
      const categoryEdges: Edge[] = []
      for (const key of revealedCategories) {
        if (activeFilters.has(key)) continue // toolbar already fully expanded it
        const catTypes = FILTER_GROUPS[key].types
        const remainingTypes = catTypes.filter((t) => !revealedTypes.has(t))
        if (remainingTypes.length === 0) continue // fully drilled, nothing left to summarize
        const remainingNodes = vm.graph.nodes.filter((n) => remainingTypes.includes(n.type as any))
        if (remainingNodes.length === 0) continue
        const hc = countHealth(remainingNodes)
        const cid = categoryNodeId(key)
        categoryNodes.push({
          id: cid,
          type: 'groupNode',
          position: { x: 0, y: 0 },
          data: {
            groupType: key,
            label: FILTER_GROUPS[key].label,
            count: remainingNodes.length,
            healthCounts: hc,
            color: FILTER_GROUPS[key].color,
            icon: '',
            isCritical: isCriticalGroup(hc, remainingNodes.length),
          } as GroupNodeData,
          width: 260,
          height: 90,
        })
        if (rootAnchor) {
          categoryEdges.push({
            id: `${rootAnchor.id}→${cid}→RUNS_ON`,
            source: rootAnchor.id,
            target: cid,
            label: 'RUNS_ON',
            type: 'smoothstep',
            markerEnd: { type: MarkerType.ArrowClosed, width: 8, height: 8, color: '#2d2d52' },
            style: { stroke: '#2d2d52', strokeWidth: 1.5 },
            labelStyle: { fill: '#475569', fontSize: 9, fontFamily: 'Geist Mono, JetBrains Mono, monospace' },
            labelBgStyle: { fill: 'var(--bg)', fillOpacity: 0.9 },
          })
        }
      }

      flowNodes = [...gn, ...categoryNodes]
      flowEdges = [...ge, ...categoryEdges]

      // Attach orphans: some raw relationships only point DOWNWARD from a
      // hidden node (e.g. Namespace → Deployment exists, but nothing points
      // INTO Namespace — there is no Cluster → Namespace edge in the data at
      // all). When such a hidden node is skipped, its children have no path
      // to bridge through and would float with zero connections. Give every
      // still-unconnected node a sensible fallback parent instead of leaving
      // it visibly detached.
      const flowNodeIds = new Set(flowNodes.map((n) => n.id))
      const hasIncoming = new Set(flowEdges.map((e) => e.target))
      const findIndividualByType = (t: string) =>
        flowNodes.find((n) => n.type === 'infraNode' && (n.data as InfraNodeData).nodeType === t)?.id

      for (const n of flowNodes) {
        if (rootAnchor && n.id === rootAnchor.id) continue
        if (hasIncoming.has(n.id)) continue

        const category = n.id.startsWith('category:')
          ? ((n.data as GroupNodeData).groupType as FilterKey)
          : n.type === 'groupNode'
            ? TYPE_TO_CATEGORY[(n.data as GroupNodeData).groupType]
            : n.type === 'infraNode'
              ? TYPE_TO_CATEGORY[(n.data as InfraNodeData).nodeType]
              : undefined

        let anchorId: string | undefined
        if (category === 'k8s') anchorId = findIndividualByType('cluster') ?? categoryNodeId('k8s')
        else if (category === 'docker') anchorId = findIndividualByType('container_runtime') ?? categoryNodeId('docker')
        if (!anchorId || anchorId === n.id || !flowNodeIds.has(anchorId)) anchorId = hostNode?.id

        if (anchorId && anchorId !== n.id) {
          flowEdges.push({
            id: `${anchorId}→${n.id}→CONTAINS`,
            source: anchorId,
            target: n.id,
            label: 'CONTAINS',
            type: 'smoothstep',
            markerEnd: { type: MarkerType.ArrowClosed, width: 8, height: 8, color: '#2d2d52' },
            style: { stroke: '#2d2d52', strokeWidth: 1.5, strokeDasharray: '3 3' },
            labelStyle: { fill: '#475569', fontSize: 9, fontFamily: 'Geist Mono, JetBrains Mono, monospace' },
            labelBgStyle: { fill: 'var(--bg)', fillOpacity: 0.9 },
          })
          hasIncoming.add(n.id)
        }
      }

      setGroupsMap(groups)
    } else {
      const { nodes: fn, edges: fe } = buildFlatFlowElements(
        vm.graph.nodes,
        vm.graph.edges,
        activeFilters,
      )
      // The map draws every resource with the compact disc-and-label node;
      // `buildFlatFlowElements` emits the card nodes used by the tree view.
      flowNodes = viewMode === 'map'
        ? fn.map((n) => ({ ...n, type: 'mapNode', width: MAP_NODE_WIDTH, height: MAP_NODE_HEIGHT }))
        : fn
      flowEdges = fe
      setGroupsMap(new Map())
    }

    if (flowNodes.length === 0) {
      setNodes([])
      setEdges([])
      return
    }

    let ln: Node[]
    let le: Edge[]
    if (viewMode === 'map') {
      // Every node on screen, arranged as a hub-and-spokes map. Deterministic
      // by design: snapshots arrive every 15s and a re-randomised layout would
      // make the whole map jump on every refresh.
      ln = applyStructuredMapLayout(flowNodes, flowEdges).nodes
      le = flowEdges.map((e) => ({
        ...e,
        label: undefined,
        type: 'straight',
        style: { stroke: '#3a3a52', strokeWidth: 1, strokeDasharray: '2 4', opacity: 0.75 },
        markerEnd: undefined,
      }))
    } else if (viewMode === 'grouped') {
      // Namespace-scoped hierarchy: namespaced resources nest under the
      // namespace that owns them, everything else keeps the zone/tier grid.
      ln = applyNamespaceHierarchyLayout(flowNodes, flowEdges).nodes
      le = flowEdges
    } else {
      const laid = applyRootedDagreLayout(flowNodes, flowEdges, { rankdir: 'TB', ranksep: 120, nodesep: 70 })
      ln = laid.nodes
      le = laid.edges
    }

    const finalNodes = applySpotlight(ln, spotlightKey, viewMode)

    // Wire the "•••" picker onto the host node and every group node. Defined
    // fresh each rebuild so it always closes over the current groupsMap/state.
    const withPickers = finalNodes.map((n) => {
      if (n.type === 'infraNode' && (n.data as InfraNodeData).nodeType === 'host') {
        return {
          ...n,
          data: {
            ...n.data,
            onOpenPicker: (_nodeId: string, e: React.MouseEvent) => {
              setPickerFor({ kind: 'host', x: e.clientX, y: e.clientY })
            },
          },
        }
      }
      if (n.type === 'groupNode') {
        const gdata = n.data as GroupNodeData
        const isCategory = n.id.startsWith('category:')
        return {
          ...n,
          data: {
            ...gdata,
            onOpenPicker: (_nodeId: string, e: React.MouseEvent) => {
              if (isCategory) {
                setPickerFor({ kind: 'category', category: gdata.groupType as FilterKey, x: e.clientX, y: e.clientY })
              } else {
                setPickerFor({ kind: 'group', groupType: gdata.groupType, groupKey: gdata.groupKey ?? gdata.groupType, x: e.clientX, y: e.clientY })
              }
            },
          },
        }
      }
      return n
    })

    setNodes(withPickers)
    setEdges(le)
    // spotlightKey is read above but left out of the deps on purpose. A spotlight
    // toggle only restyles nodes (the effect below); re-running layout for it would
    // reset any node positions the user has dragged. With React 19.2+ this read can
    // move into a useEffectEvent and the suppression can go.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- spotlightKey, see above
  }, [vm.graph, activeFilters, viewMode, expandedGroups, expandedNodeIds, revealedCategories, revealedTypes, setNodes, setEdges])

  // ── Re-apply spotlight without re-running layout ───────────────────────────
  useEffect(() => {
    setNodes((prev) => applySpotlight(prev, spotlightKey, viewMode))
  }, [spotlightKey, viewMode, setNodes])

  // ── Spotlight helpers ──────────────────────────────────────────────────────

  function applySpotlight(
    allNodes: Node[],
    key: FilterKey | null,
    mode: 'map' | 'grouped' | 'flat',
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

  function handleFilterClick(key: FilterKey) {
    if (!activeFilters.has(key)) {
      setActiveFilters((prev) => new Set([...prev, key]))
      setSpotlightKey(null)
      return
    }
    if (spotlightKey === key) {
      setSpotlightKey(null)
    } else {
      setSpotlightKey(key)
      zoomToFilterGroup(key)
    }
  }

  function handleFilterRightClick(e: React.MouseEvent, key: FilterKey) {
    e.preventDefault()
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
      // Groups are keyed by scope: `group:pod@kube-system` is a different node
      // from `group:pod@local-path-storage`, so the canvas id resolves the
      // group rather than the bare type.
      const group = groupsMap.get(`group:${gdata.groupKey ?? gdata.groupType}`)
      if (group) {
        setSelectedNodeId(null)
        setDrawerGroup(group)
        setDrawerInitialFilter(undefined)
        setShowLogs(false)
        setShowTerminal(false)
      }
    } else {
      setDrawerGroup(null)
      setSelectedNodeId((prev) => {
        if (prev !== node.id) { setShowLogs(false); setShowTerminal(false) }
        return node.id
      })
    }
  }, [groupsMap])

  function handleSelectNodeFromDrawer(nodeId: string) {
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

  const searchMatches = useMemo(() => {
    const q = searchQuery.trim().toLowerCase()
    if (!q || !vm.graph) return []
    return vm.graph.nodes.filter((n) => n.label.toLowerCase().includes(q)).slice(0, 20)
  }, [searchQuery, vm.graph])

  /**
   * Would this node actually be on the canvas right now? Drives the
   * "hidden by the current filter" hint next to a search result.
   */
  function isNodeHidden(node: GraphNode): boolean {
    if (SINGLETON_TYPES.has(node.type)) return false
    const cat = TYPE_TO_CATEGORY[node.type]
    if (!cat) return true
    // Either the category is switched on, or this type was drilled into
    // individually via a "•••" picker.
    if (!activeFilters.has(cat) && !revealedTypes.has(node.type)) return true
    // Non-singleton types render as one group node unless the whole group is
    // expanded, or this specific node was picked out of it.
    if (!expandedGroups.has(`group:${node.type}`)) {
      return !expandedNodeIds.has(node.id)
    }
    return false
  }

  function handleSelectSearchResult(nodeId: string) {
    setSearchQuery('')
    setSearchFocused(false)
    const node = vm.graph?.nodes.find((n) => n.id === nodeId)
    if (!node) return
    // A match hidden by the current filter used to do nothing visible when
    // selected. Activate its category and expand its type group first, then
    // pan to it once React has rendered it.
    const cat = TYPE_TO_CATEGORY[node.type]
    if (cat && !activeFilters.has(cat)) {
      setActiveFilters((prev) => new Set([...prev, cat]))
    }
    if (!SINGLETON_TYPES.has(node.type) && node.type !== 'cluster') {
      setExpandedGroups((prev) => new Set([...prev, `group:${node.type}`]))
    }
    setSpotlightKey(null)
    pendingFocusId.current = nodeId
  }

  // Centre on the node once React has actually rendered it. Runs after the
  // filter/group changes above commit, which is when the node exists.
  useEffect(() => {
    const id = pendingFocusId.current
    if (!id || !rfRef.current) return
    const rfNode = rfRef.current.getNode(id)
    if (!rfNode) return // not mounted yet; the next render will retry
    pendingFocusId.current = null
    setDrawerGroup(null)
    setSelectedNodeId(id)
    rfRef.current.setCenter(
      rfNode.position.x + (rfNode.width ?? 220) / 2,
      rfNode.position.y + (rfNode.height ?? 60) / 2,
      { zoom: 1.8, duration: 600 },
    )
  }, [nodes, selectedNodeId])

  function handlePaneClick() {
    setSelectedNodeId(null)
    setDrawerGroup(null)
    setSpotlightKey(null)
    setShowLogs(false)
    setShowTerminal(false)
  }

  // ── Export helpers ─────────────────────────────────────────────────────────
  async function handleExportPNG() {
    if (!canvasWrapRef.current) return
    try {
      const { toPng } = await import('html-to-image')
      const dataUrl = await toPng(canvasWrapRef.current, { backgroundColor: 'var(--bg)', pixelRatio: 2 })
      const a = document.createElement('a')
      a.href = dataUrl
      a.download = `${vm.hostname ?? vm.code}-canvas.png`
      a.click()
    } catch (err) {
      console.error('[export] PNG failed', err)
    }
  }

  function handleExportJSON() {
    if (!vm.graph) return
    const json = JSON.stringify(vm.graph, null, 2)
    const blob = new Blob([json], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${vm.hostname ?? vm.code}-graph.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  async function handleExportSVG() {
    if (!canvasWrapRef.current) return
    try {
      const { toSvg } = await import('html-to-image')
      const bg = getComputedStyle(document.documentElement).getPropertyValue('--bg').trim() || '#0A0A0A'
      const dataUrl = await toSvg(canvasWrapRef.current, { backgroundColor: bg })
      const a = document.createElement('a')
      a.href = dataUrl
      a.download = `${vm.hostname ?? vm.code}-canvas.svg`
      a.click()
    } catch (err) {
      console.error('[export] SVG failed', err)
    }
  }

  // ── Critical alert banner ──────────────────────────────────────────────────
  const criticalGroups = useMemo(() => {
    const alerts: Array<{ label: string; degraded: number; id: string }> = []
    for (const [id, g] of groupsMap) {
      if (isCriticalGroup(g.healthCounts, g.count)) {
        alerts.push({
          // Scoped groups can repeat a label (three namespaces each with a
          // degraded pod). Without the namespace the banner would say
          // "Pods: 2 degraded" three times and read as a rendering bug.
          label: g.namespace ? `${g.label} in ${g.namespace}` : g.label,
          degraded: g.healthCounts.degraded + g.healthCounts.unhealthy,
          id,
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
    <div style={{ width: '100%', height: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--bg)', overflow: 'hidden' }}>

      {/* ── Top bar ──────────────────────────────────────────────── */}
      <div style={{
        flexShrink: 0, display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 8,
        padding: '8px 14px', minHeight: 50, zIndex: 10,
        background: 'var(--surface)', borderBottom: '1px solid var(--line)',
        backdropFilter: 'blur(12px)',
      }}>
        {onBack && (
          <>
            <button onClick={onBack} style={ICON_BTN}
              onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--line)'; e.currentTarget.style.color = 'var(--ink2)' }}
              onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--ink3)' }}>
              <ArrowLeft size={15} />
            </button>
            <div style={DIVIDER} />
          </>
        )}

        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Server size={13} color="var(--ink2)" strokeWidth={1.5} />
          <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)' }}>{vm.hostname ?? vm.code}</span>
          <span style={{ fontSize: 10, padding: '1px 8px', borderRadius: 20, background: 'var(--line)', color: 'var(--ink2)', border: '1px solid var(--line2)', fontFamily: 'monospace' }}>
            {vm.code}
          </span>
        </div>

        {stats && (
          <>
            <div style={DIVIDER} />
            <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: 'var(--ink3)' }}>
              <GitBranch size={11} /><span style={{ color: 'var(--ink2)' }}>{stats.totalNodes}</span> nodes
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: 'var(--ink3)' }}>
              <Layers size={11} /><span style={{ color: 'var(--ink2)' }}>{stats.totalEdges}</span> edges
            </span>
            {snapshot?.collectionDuration != null && (
              <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: 'var(--ink3)' }}>
                <Clock size={11} /><span style={{ color: 'var(--ink2)' }}>{fmt(snapshot.collectionDuration)}</span>
              </span>
            )}
          </>
        )}

        <div style={{ flex: 1 }} />

        {/* Spotlight hint */}
        {spotlightKey && (
          <span style={{ fontSize: 10, color: '#f59e0b', display: 'flex', alignItems: 'center', gap: 5 }}>
            <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#f59e0b', display: 'inline-block' }} />
            Spotlight: {FILTER_GROUPS[spotlightKey].label}
            <button onClick={() => setSpotlightKey(null)} style={{ background: 'none', border: 'none', color: '#f59e0b', cursor: 'pointer', padding: 0, fontSize: 11 }}>✕</button>
          </span>
        )}

        {/* Search */}
        <div style={{ position: 'relative' }}>
          <div style={{
            display: 'flex', alignItems: 'center', gap: 6,
            background: 'var(--bg)', border: '1px solid var(--line)', borderRadius: 8,
            padding: '4px 8px', width: 180,
          }}>
            <Search size={12} color="var(--ink4)" />
            <input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onFocus={() => setSearchFocused(true)}
              onBlur={() => setTimeout(() => setSearchFocused(false), 150)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && searchMatches.length > 0) handleSelectSearchResult(searchMatches[0].id)
                if (e.key === 'Escape') { setSearchQuery(''); setSearchFocused(false) }
              }}
              placeholder="Search nodes…"
              style={{ flex: 1, minWidth: 0, background: 'transparent', border: 'none', outline: 'none', fontSize: 12, color: 'var(--ink)', fontFamily: 'inherit' }}
            />
            {searchQuery && (
              <button onClick={() => setSearchQuery('')} style={{ background: 'none', border: 'none', color: 'var(--ink4)', cursor: 'pointer', padding: 0, fontSize: 11, lineHeight: 1 }}>✕</button>
            )}
          </div>
          {searchFocused && searchQuery && (
            <div style={{
              position: 'absolute', top: 'calc(100% + 4px)', left: 0, width: 260, maxHeight: 320,
              overflowY: 'auto', background: 'var(--surface)', border: '1px solid var(--line)',
              borderRadius: 8, boxShadow: '0 8px 24px rgba(0,0,0,0.25)', zIndex: 50,
            }}>
              {searchMatches.length === 0 ? (
                <div style={{ padding: '8px 10px', fontSize: 11, color: 'var(--ink4)' }}>No matches</div>
              ) : (
                searchMatches.map((n) => {
                  const hidden = isNodeHidden(n)
                  return (
                  <button
                    key={n.id}
                    onMouseDown={() => handleSelectSearchResult(n.id)}
                    style={{
                      display: 'flex', width: '100%', textAlign: 'left', alignItems: 'center', gap: 8,
                      padding: '6px 10px', background: 'transparent', border: 'none', cursor: 'pointer',
                      fontSize: 12, color: 'var(--ink)',
                    }}
                  >
                    <span style={{ width: 6, height: 6, borderRadius: '50%', flexShrink: 0, background: getNodeColor(n.type) }} />
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{n.label}</span>
                    <span style={{ marginLeft: 'auto', fontSize: 10, color: 'var(--ink4)', flexShrink: 0 }}>{n.type}</span>
                    {hidden && (
                      <span
                        title="Not visible under the current filter. Click to reveal it."
                        style={{ fontSize: 9, flexShrink: 0, padding: '1px 5px', borderRadius: 4,
                          border: '1px solid var(--line2)', color: 'var(--ink3)' }}
                      >
                        hidden
                      </span>
                    )}
                  </button>
                  )
                })
              )}
            </div>
          )}
        </div>

        <div style={DIVIDER} />

        {/* View mode toggle */}
        <div style={{ display: 'flex', gap: 1, background: 'var(--surface)', border: '1px solid var(--line)', borderRadius: 8, padding: 2 }}>
          {(['map', 'grouped', 'flat'] as const).map((m) => (
            <button key={m} onClick={() => setViewMode(m)}
              title={m === 'map' ? 'Map: every resource as a node' : m === 'grouped' ? 'Grouped: one card per type' : 'Flat: full hierarchy'}
              style={{
                display: 'flex', alignItems: 'center', gap: 4,
                padding: '3px 9px', borderRadius: 6, border: 'none', cursor: 'pointer',
                fontSize: 11, fontWeight: viewMode === m ? 600 : 400,
                background: viewMode === m ? 'var(--line)' : 'transparent',
                color: viewMode === m ? 'var(--ink)' : 'var(--ink3)',
                transition: 'all 0.15s',
              }}>
              {m === 'flat' ? <Network size={11} /> : m === 'map' ? <Waypoints size={11} /> : <Rows3 size={11} />}
              {m === 'map' ? 'Map' : m === 'grouped' ? 'Grouped' : 'Flat'}
            </button>
          ))}
        </div>

        <div style={DIVIDER} />

        {/* Filter chips — every chip uses identical styling. The previous two
            rows styled the second group with a dashed border, which read as
            disabled even though the chips were live. Differing by position
            only. */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <Filter size={11} color="var(--ink3)" />
          {TOOLBAR_ORDER.map((key, i) => {
            const g = FILTER_GROUPS[key]
            const active = activeFilters.has(key)
            const spotlit = spotlightKey === key
            const count = filterCounts[key]
            return (
              <span key={key} style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                {i === 4 && (
                  <span style={{ width: 1, height: 13, background: 'var(--line)', display: 'inline-block', margin: '0 2px' }} />
                )}
                <button
                  onClick={() => handleFilterClick(key)}
                  onContextMenu={(e) => handleFilterRightClick(e, key)}
                  title={`${g.label} — ${count} node${count === 1 ? '' : 's'}` + (count === 0 ? ' (none discovered)' : '')
                    + (active ? ' · click to spotlight, right-click to hide' : ' · click to show')}
                  style={{
                    padding: '3px 9px', borderRadius: 6, fontSize: 11, cursor: 'pointer',
                    display: 'inline-flex', alignItems: 'center', gap: 5,
                    fontWeight: active ? 600 : 400,
                    background: spotlit ? g.color : active ? `${g.color}22` : 'transparent',
                    color: spotlit ? '#fff' : active ? g.color : 'var(--ink3)',
                    border: `1px solid ${spotlit ? g.color : active ? `${g.color}40` : 'var(--line)'}`,
                    transition: 'all 0.15s',
                  }}>
                  {g.label}
                  {/* Counts on every chip. A chip with no number gives no
                      reason to click it — the number is the call to action. */}
                  <span style={{
                    fontSize: 10, fontVariantNumeric: 'tabular-nums',
                    color: spotlit ? '#fff' : count ? 'var(--ink3)' : 'var(--line2)',
                    opacity: active ? 0.85 : 1,
                  }}>
                    {count}
                  </span>
                </button>
              </span>
            )
          })}
        </div>

        <div style={DIVIDER} />

        <button onClick={handleRefresh}
          style={{ display: 'flex', alignItems: 'center', gap: 5, padding: '4px 10px', borderRadius: 7, border: '1px solid var(--line)', background: 'transparent', color: 'var(--ink2)', fontSize: 11, cursor: 'pointer', transition: 'all 0.15s' }}
          onMouseEnter={(e) => { e.currentTarget.style.borderColor = 'var(--line2)'; e.currentTarget.style.color = 'var(--ink)' }}
          onMouseLeave={(e) => { e.currentTarget.style.borderColor = 'var(--line)'; e.currentTarget.style.color = 'var(--ink2)' }}>
          <RefreshCw size={12} className={isRefreshing ? 'animate-spin' : ''} />
          Refresh
        </button>

        <LastUpdated ts={vm.lastUpdated} />

        <div style={DIVIDER} />

        {/* Export dropdown */}
        <div style={{ position: 'relative' }}>
          <button
            id="export-btn"
            onClick={() => {
              const menu = document.getElementById('export-menu')
              if (menu) menu.style.display = menu.style.display === 'none' ? 'flex' : 'none'
            }}
            style={{ display: 'flex', alignItems: 'center', gap: 5, padding: '4px 10px', borderRadius: 7, border: '1px solid var(--line)', background: 'transparent', color: 'var(--ink2)', fontSize: 11, cursor: 'pointer', transition: 'all 0.15s' }}
            onMouseEnter={(e) => { e.currentTarget.style.borderColor = 'var(--line2)'; e.currentTarget.style.color = 'var(--ink)' }}
            onMouseLeave={(e) => { e.currentTarget.style.borderColor = 'var(--line)'; e.currentTarget.style.color = 'var(--ink2)' }}>
            <Download size={12} /> Export
          </button>
          <div id="export-menu" style={{ display: 'none', flexDirection: 'column', position: 'absolute', top: '100%', right: 0, marginTop: 4, background: 'var(--surface)', border: '1px solid var(--line)', borderRadius: 10, overflow: 'hidden', minWidth: 130, zIndex: 50, boxShadow: '0 12px 32px rgba(0,0,0,0.12)' }}>
            {[
              { label: 'Export PNG', action: () => { handleExportPNG(); const m = document.getElementById('export-menu'); if (m) m.style.display = 'none' } },
              { label: 'Export SVG', action: () => { handleExportSVG(); const m = document.getElementById('export-menu'); if (m) m.style.display = 'none' } },
              { label: 'Export JSON', action: () => { handleExportJSON(); const m = document.getElementById('export-menu'); if (m) m.style.display = 'none' } },
            ].map(({ label, action }) => (
              <button key={label} onClick={action}
                style={{ padding: '9px 14px', background: 'transparent', border: 'none', color: 'var(--ink2)', fontSize: 12, cursor: 'pointer', textAlign: 'left', width: '100%', transition: 'all 0.12s' }}
                onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--line)'; e.currentTarget.style.color = 'var(--ink)' }}
                onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--ink2)' }}>
                {label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* ── Critical alert banner ─────────────────────────────────── */}
      {criticalGroups.length > 0 && (
        <div style={{
          flexShrink: 0,
          background: 'rgba(248,113,113,0.07)',
          borderBottom: '1px solid rgba(248,113,113,0.18)',
          padding: '5px 16px',
          display: 'flex',
          alignItems: 'center',
          gap: 12,
        }}>
          <AlertTriangle size={13} color="#ef4444" strokeWidth={1.5} />
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {criticalGroups.map((a) => (
              <button
                key={a.id}
                onClick={() => {
                  const g = groupsMap.get(a.id)
                  if (g) { setDrawerGroup(g); setDrawerInitialFilter('degraded'); setSelectedNodeId(null) }
                }}
                style={{
                  display: 'flex', alignItems: 'center', gap: 6,
                  padding: '3px 10px', borderRadius: 6,
                  border: '1px solid rgba(248,113,113,0.25)',
                  background: 'rgba(248,113,113,0.08)',
                  cursor: 'pointer', fontSize: 11, color: '#fca5a5',
                }}
              >
                ⚠ {a.label}: <strong>{a.degraded} degraded</strong>, view &amp; act →
              </button>
            ))}
          </div>
        </div>
      )}

      {/* ── Canvas ───────────────────────────────────────────────── */}
      <div ref={canvasWrapRef} style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
        {!vm.graph ? (
          <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 14 }}>
              <div style={{ width: 36, height: 36, borderRadius: '50%', border: '2.5px solid rgba(250,250,250,0.08)', borderTopColor: 'var(--ink2)' }} className="animate-spin" />
              <p style={{ fontSize: 13, color: 'var(--ink3)' }}>Loading infrastructure graph…</p>
            </div>
          </div>
        ) : nodes.length === 0 ? (
          <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <div style={{ textAlign: 'center' }}>
              <p style={{ fontSize: 16, fontWeight: 600, color: 'var(--ink)', marginBottom: 6 }}>No nodes to display</p>
              <p style={{ fontSize: 13, color: 'var(--ink3)' }}>Try enabling more filter categories</p>
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
            <Background variant={BackgroundVariant.Dots} gap={22} size={1} color="rgba(255,255,255,0.04)" />
            <Controls style={{ bottom: 80, left: 16 }} showInteractive={false} />
            <MiniMap
              nodeColor={(n) => {
                if (n.type === 'groupNode') return getNodeColor((n.data as GroupNodeData).groupType)
                return getNodeColor((n.data as InfraNodeData)?.nodeType, (n.data as InfraNodeData)?.health)
              }}
              maskColor="rgba(0,0,0,0.5)"
              style={{ bottom: 16, right: 16, width: 160, height: 100 }}
            />
          </ReactFlow>
        )}

        {drawerGroup && (
          <GroupDrawer
            group={drawerGroup}
            vmCode={vm.code}
            initialHealthFilter={drawerInitialFilter}
            onClose={() => { setDrawerGroup(null); setDrawerInitialFilter(undefined) }}
            onSelectNode={handleSelectNodeFromDrawer}
          />
        )}

        {pickerFor && pickerFor.kind === 'host' && (
          <NodePickerMenu
            title="Show on canvas"
            x={pickerFor.x}
            y={pickerFor.y}
            items={(Object.keys(FILTER_GROUPS) as FilterKey[])
              .filter((key) => key !== 'host')
              .map((key) => {
                const types = new Set<string>(FILTER_GROUPS[key].types)
                const count = vm.graph?.nodes.filter((n) => types.has(n.type)).length ?? 0
                return { key, label: FILTER_GROUPS[key].label, count, checked: revealedCategories.has(key) }
              })
              .filter((item) => item.count > 0)}
            onClose={() => setPickerFor(null)}
            onConfirm={(selected) => {
              // Each checked category gets exactly ONE summary node — it does
              // not explode into its internal types. Drilling further happens
              // from that summary's own "•••".
              setRevealedCategories(new Set(selected as FilterKey[]))
              setPickerFor(null)
            }}
          />
        )}

        {pickerFor && pickerFor.kind === 'category' && (
          <NodePickerMenu
            title={FILTER_GROUPS[pickerFor.category].label}
            x={pickerFor.x}
            y={pickerFor.y}
            items={FILTER_GROUPS[pickerFor.category].types
              .map((t) => {
                const count = vm.graph?.nodes.filter((n) => n.type === t).length ?? 0
                // Already-revealed types stay in the list, checked — so
                // reopening the menu never makes an item silently vanish.
                return { key: t, label: getGroupLabel(t), count, checked: revealedTypes.has(t) }
              })
              .filter((item) => item.count > 0)}
            onClose={() => setPickerFor(null)}
            onConfirm={(selectedTypes) => {
              setRevealedTypes((prev) => new Set([...prev, ...selectedTypes]))
              setPickerFor(null)
            }}
          />
        )}

        {pickerFor && pickerFor.kind === 'group' && (
          <NodePickerMenu
            title={FILTER_GROUPS[
              (Object.keys(FILTER_GROUPS) as FilterKey[]).find((k) =>
                (FILTER_GROUPS[k].types as readonly string[]).includes(pickerFor.groupType)
              ) ?? 'k8s'
            ].label + ': pick individual nodes'}
            x={pickerFor.x}
            y={pickerFor.y}
            items={(groupsMap.get(`group:${pickerFor.groupKey}`)?.nodes ?? []).map((n) => ({
              key: n.id,
              label: n.label,
              checked: false,
              health: n.health as any,
            }))}
            onClose={() => setPickerFor(null)}
            onConfirm={(selectedIds) => {
              setExpandedNodeIds((prev) => new Set([...prev, ...selectedIds]))
              setPickerFor(null)
            }}
          />
        )}

        {selectedNode && !drawerGroup && (
          <NodeDetailPanel
            node={selectedNode}
            vmCode={vm.code}
            onClose={() => { setSelectedNodeId(null); setShowLogs(false); setShowTerminal(false) }}
            onShowLogs={['container', 'pod', 'host', 'deployment', 'statefulset', 'daemonset', 'service'].includes(selectedNode.type) ? () => { setShowLogs(true); setShowTerminal(false) } : undefined}
            onShowTerminal={['container', 'host', 'pod'].includes(selectedNode.type) ? () => {
              setTerminalLayer(
                  selectedNode.type === 'host' ? 'host'
                      : selectedNode.type === 'pod' ? 'kubernetes'
                          : (['lxd', 'incus'].includes(String(selectedNode.metadata?.runtime ?? '').toLowerCase()) ? 'lxd' : 'docker'),
              )
              setShowTerminal(true)
              setShowLogs(false)
            } : undefined}
          />
        )}

        {selectedNode && showLogs && !showTerminal && (
          <LogsPanel
            node={selectedNode}
            vmCode={vm.code}
            onClose={() => setShowLogs(false)}
            allNodes={vm.graph?.nodes ?? []}
            allEdges={vm.graph?.edges ?? []}
          />
        )}

        {selectedNode && showTerminal && !showLogs && (
          <TerminalPanel
            node={selectedNode}
            vmCode={vm.code}
            layer={terminalLayer}
            onClose={() => setShowTerminal(false)}
          />
        )}
      </div>

      {/* ── Status bar ───────────────────────────────────────────── */}
      <div style={{
        flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '0 14px', height: 24, background: 'var(--bg)', borderTop: '1px solid var(--line)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
          {viewMode === 'grouped' && groupsMap.size > 0 && (
            <span style={{ fontSize: 10, color: 'var(--ink4)' }}>
              {groupsMap.size} groups · click card to drill down · right-click filter to hide
            </span>
          )}
          {stats && Object.entries(stats.nodesByType)
            .sort(([, a], [, b]) => b - a).slice(0, 4)
            .map(([type, count]) => (
              <span key={type} style={{ display: 'flex', alignItems: 'center', gap: 3, fontSize: 10, color: 'var(--ink4)' }}>
                <span style={{ color: getNodeColor(type) }}>●</span>
                {type}:{count}
              </span>
            ))}
        </div>
        {snapshot?.timestamp && (
          <span style={{ fontSize: 10, color: 'var(--ink4)', fontFamily: 'Geist Mono, JetBrains Mono, monospace' }}>
            {new Date(snapshot.timestamp).toLocaleTimeString()}
          </span>
        )}
      </div>

    </div>
  )
}

// ─── Micro-styles ─────────────────────────────────────────────────────────────

const ICON_BTN: React.CSSProperties = {
  width: 28, height: 28, borderRadius: 7, border: 'none',
  background: 'transparent', color: 'var(--ink3)', cursor: 'pointer',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  transition: 'background 0.15s, color 0.15s',
}

const DIVIDER: React.CSSProperties = {
  width: 1, height: 16, background: 'var(--line)', flexShrink: 0,
}

function LastUpdated({ ts }: { ts: number | null }) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(id)
  }, [])
  if (!ts) return null
  const sec = Math.max(0, Math.floor((now - ts) / 1000))
  const label =
    sec < 5         ? 'just now' :
    sec < 60        ? `${sec}s ago` :
    sec < 3600      ? `${Math.floor(sec / 60)}m ago` :
                      `${Math.floor(sec / 3600)}h ago`
  const stale = sec >= 60
  const color = stale ? '#f59e0b' : 'var(--ink3)'
  return (
    <span
      title={`Last updated ${new Date(ts).toLocaleTimeString()}`}
      style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 10, color, fontFamily: 'Geist Mono, JetBrains Mono, monospace' }}
    >
      <Clock size={10} />
      {label}
    </span>
  )
}
