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
import { applyNamespaceHierarchyLayout } from '@/lib/layout'
import { buildHubMap, applyHubRadialLayout, ORB_CELL_W, ORB_CELL_H } from '@/lib/hubMap'
import MapNode from './MapNode'
import { MapRoot, MapHub, MapOrb } from './MapOrbs'
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
import { useTerminalStore } from '@/store/terminalStore'
import {
  ArrowLeft,
  RefreshCw,
  Clock,
  GitBranch,
  Layers,
  Filter,
  Rows3,
  Waypoints,
  Server,
  AlertTriangle,
  Download,
  Search,
  X,
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
    color: '#10b981',
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
      labelStyle: { fill: 'var(--ink4)', fontSize: 9, fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)' },
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
  mapRoot: MapRoot,
  mapHub: MapHub,
  mapOrb: MapOrb,
}

// ─── Main component ───────────────────────────────────────────────────────────

interface InfraCanvasProps {
  vm: VMState
  // Optional — when omitted, the back button is hidden. In single-VM
  // local mode there is no parent view to return to.
  onBack?: () => void
}

export default function InfraCanvas({ vm, onBack }: InfraCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState([])
  const [edges, setEdges, onEdgesChange] = useEdgesState([])

  const [viewMode, setViewMode] = useState<'map' | 'grouped'>('grouped')
  // Toolbar chips (Kubernetes / Docker / Host / ...) — unchanged, existing
  // behaviour: toggling one on immediately shows every type in that category.
  // Kept separate from the "•••" picker tree below on purpose.
  // Default to 'host' for Machines (VM agents always report one), but
  // Clusters connections (kubeconfig direct-connect / relay pod) report no
  // host node at all — defaulting to 'host' there hides everything on first
  // load ("No nodes to display"). Fall back to 'k8s' when there's no host.
  const [activeFilters, setActiveFilters] = useState<Set<FilterKey>>(() => {
    const hasHost = vm.graph?.nodes?.some((n) => n.type === 'host')
    return new Set<FilterKey>([hasHost ? 'host' : 'k8s'])
  })
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set())

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
  // Namespace focus — dim the canvas to a single namespace. A pod group
  // spans every namespace, so "which pods do I care about" needs its own
  // axis next to the type filters.
  const [nsFocus, setNsFocus] = useState<string | null>(null)
  const [groupsMap, setGroupsMap] = useState<Map<string, GroupInfo>>(new Map())
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const refreshingRef = useRef(false)
  const [alertDismissed, setAlertDismissed] = useState(false)
  const [hideDegraded, setHideDegraded] = useState(false)

  // Live node count per filter category, for the chip badges. Mirrors the
  // "Hide degraded" toggle so a chip never advertises nodes that are hidden.
  const filterCounts = useMemo(() => {
    const source = hideDegraded
      ? (vm.graph?.nodes ?? []).filter((n) => n.health !== 'degraded' && n.health !== 'unhealthy')
      : (vm.graph?.nodes ?? [])
    const out = {} as Record<FilterKey, number>
    for (const key of Object.keys(FILTER_GROUPS) as FilterKey[]) {
      const types = new Set<string>(FILTER_GROUPS[key].types as readonly string[])
      out[key] = source.reduce((sum, n) => (types.has(n.type) ? sum + 1 : sum), 0)
    }
    return out
  }, [vm.graph, hideDegraded])
  const [showLogs, setShowLogs] = useState(false)
  const { openTerminal } = useTerminalStore()
  const canvasWrapRef = useRef<HTMLDivElement>(null)

  // Spotlight: which filter key to dim everything else for (null = no spotlight)
  const [spotlightKey, setSpotlightKey] = useState<FilterKey | null>(null)

  // ReactFlow instance — for programmatic pan/zoom
  const rfRef = useRef<ReactFlowInstance | null>(null)

  // Right panel resize
  const [panelWidth, setPanelWidth] = useState(340)
  const dragRef = useRef<{ startX: number; startW: number } | null>(null)

  const onResizeMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    dragRef.current = { startX: e.clientX, startW: panelWidth }
    const onMove = (mv: MouseEvent) => {
      if (!dragRef.current) return
      const delta = dragRef.current.startX - mv.clientX
      const next = Math.min(700, Math.max(240, dragRef.current.startW + delta))
      setPanelWidth(next)
    }
    const onUp = () => {
      dragRef.current = null
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }, [panelWidth])

  // Cache last found node so the panel doesn't flicker closed during graph refreshes
  const selectedNodeCache = useRef<GraphNode | null>(null)
  const selectedNode = useMemo(() => {
    if (!selectedNodeId) {
      selectedNodeCache.current = null
      return null
    }
    if (!vm.graph) return selectedNodeCache.current
    const found = vm.graph.nodes.find((n) => n.id === selectedNodeId) ?? null
    if (found) selectedNodeCache.current = found
    return found ?? selectedNodeCache.current
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

    let sourceNodes = hideDegraded
      ? vm.graph.nodes.filter((n) => n.health !== 'degraded' && n.health !== 'unhealthy')
      : vm.graph.nodes

    // Namespace focus filters the graph rather than dimming it. Dimming left
    // every cross-namespace group half-faded, which read as a broken canvas
    // rather than a focus. Filtering here means the groups are rebuilt from
    // the focused namespace only, so "Pods ×12" is that namespace's pods and
    // not the cluster total. Unnamespaced nodes (cluster, k8s node, host,
    // containers) are structural context and always stay.
    if (nsFocus) {
      sourceNodes = sourceNodes.filter((n) => {
        // Keep only the focused namespace node itself, so the canvas says
        // which namespace it is filtered to instead of listing all of them.
        if (n.type === 'namespace') return n.label === nsFocus
        const ns = n.metadata?.namespace
        return ns === undefined || ns === null || ns === '' || ns === nsFocus
      })
    }

    const sourceNodeIds = new Set(sourceNodes.map((n) => n.id))
    let sourceEdges = vm.graph.edges.filter((e) => sourceNodeIds.has(e.source) && sourceNodeIds.has(e.target))
    if (hideDegraded) {
      const okIds = new Set(
        vm.graph.nodes
          .filter((n) => n.health !== 'degraded' && n.health !== 'unhealthy')
          .map((n) => n.id),
      )
      sourceEdges = sourceEdges.filter((e) => okIds.has(e.source) && okIds.has(e.target))
    }

    let flowNodes: Node[]
    let flowEdges: Edge[]

    if (viewMode === 'grouped') {
      const { nodes: gn, edges: ge, groups } = buildGroupedGraph(
        sourceNodes,
        sourceEdges,
        visibleTypes,
        expandedGroups,
        expandedNodeIds,
      )

      // Category-summary nodes: one per revealed-but-not-fully-drilled
      // category (e.g. "Kubernetes ×82"), sitting between host and the
      // type-level groups that appear once you drill into that summary.
      const hostNode = sourceNodes.find((n) => n.type === 'host')
      // Clusters connections (kubeconfig direct-connect / relay pod) report no
      // host node — fall back to the cluster node as the canvas root anchor so
      // category summaries and orphans still attach to something.
      const rootAnchor = hostNode ?? sourceNodes.find((n) => n.type === 'cluster')
      const categoryNodes: Node[] = []
      const categoryEdges: Edge[] = []
      for (const key of revealedCategories) {
        if (activeFilters.has(key)) continue // toolbar already fully expanded it
        const catTypes = FILTER_GROUPS[key].types
        const remainingTypes = catTypes.filter((t) => !revealedTypes.has(t))
        if (remainingTypes.length === 0) continue // fully drilled, nothing left to summarize
        const remainingNodes = sourceNodes.filter((n) => remainingTypes.includes(n.type as any))
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
            labelStyle: { fill: '#475569', fontSize: 9, fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)' },
            labelBgStyle: { fill: '#070711', fillOpacity: 0.9 },
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
        if (!anchorId || anchorId === n.id || !flowNodeIds.has(anchorId)) anchorId = rootAnchor?.id

        if (anchorId && anchorId !== n.id) {
          flowEdges.push({
            id: `${anchorId}→${n.id}→CONTAINS`,
            source: anchorId,
            target: n.id,
            label: 'CONTAINS',
            type: 'smoothstep',
            markerEnd: { type: MarkerType.ArrowClosed, width: 8, height: 8, color: '#2d2d52' },
            style: { stroke: '#2d2d52', strokeWidth: 1.5, strokeDasharray: '3 3' },
            labelStyle: { fill: '#475569', fontSize: 9, fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)' },
            labelBgStyle: { fill: '#070711', fillOpacity: 0.9 },
          })
          hasIncoming.add(n.id)
        }
      }

      setGroupsMap(groups)
    } else {
      const { nodes: fn, edges: fe } = buildFlatFlowElements(
        sourceNodes,
        sourceEdges,
        activeFilters,
      )
      // The map draws every resource with the compact disc-and-label node.
      // `buildFlatFlowElements` emits `infraNode` (and `namespaceGroup`) for
      // the hierarchy tree, so swap the type here rather than duplicating the
      // whole builder.
      if (viewMode === 'map') {
        // The map is a containment hierarchy: platform → resource type →
        // namespace → its members. Synthetic parent/child nodes and edges are
        // added here; `applyHierarchyLayout` positions them.
        const h = buildHubMap(
          fn.map((n) => ({ ...n, type: 'mapOrb', width: ORB_CELL_W, height: ORB_CELL_H })) as Node[],
          fe as Edge[],
        )
        flowNodes = h.nodes
        flowEdges = h.edges as Edge[]
      } else {
        flowNodes = fn
        flowEdges = fe
      }
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
    if (viewMode === 'map') {
      // Structured map: nodes bucketed by the thing that owns them (namespace
      // for k8s, type otherwise), each bucket a labelled grid of individual
      // nodes. A free radial layout loses all structure once the graph is big
      // — 189 identical discs in a circle is as unreadable as the count cards
      // it replaced. Deterministic, so a 15s refresh never reshuffles it.
      ln = applyHubRadialLayout(flowNodes, flowEdges).nodes
      // Three kinds of line, drawn differently so the structure is legible:
      // containment is the solid spine, the type→member reach is dashed (it
      // spans levels, so solid would read as false parentage), and the
      // original snapshot edges stay faint in the background.
      le = flowEdges.map((e) => {
        const kind = (e as { kind?: string }).kind
        if (kind === 'contain') {
          return {
            ...e, label: undefined, type: 'straight' as const,
            style: { stroke: '#4b5280', strokeWidth: 1.1, opacity: 0.55 },
            markerEnd: undefined,
          }
        }
        if (kind === 'ns-link') {
          return {
            ...e, label: undefined, type: 'straight' as const,
            style: { stroke: '#6d6d94', strokeWidth: 1.1, strokeDasharray: '3 5', opacity: 0.55 },
            markerEnd: undefined,
          }
        }
        return {
          ...e, label: undefined, type: 'straight' as const,
          style: { stroke: '#2a2a3a', strokeWidth: 1, strokeDasharray: '2 5', opacity: 0.16 },
          markerEnd: undefined,
        }
      })
    } else {
      // Namespace-scoped tree: namespaced resources nest under the namespace
      // that owns them, everything else keeps the zone/tier grid.
      ln = applyNamespaceHierarchyLayout(flowNodes, flowEdges).nodes
      le = flowEdges
    }

    // Apply spotlight opacity if active
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
  }, [vm.graph, activeFilters, viewMode, expandedGroups, expandedNodeIds, revealedCategories, revealedTypes, hideDegraded, nsFocus])

  // ── Re-apply spotlight without re-running layout ───────────────────────────
  useEffect(() => {
    setNodes((prev) => applySpotlight(prev, spotlightKey, viewMode))
  }, [spotlightKey, viewMode, nsFocus])

  // ── Spotlight helpers ──────────────────────────────────────────────────────

  /**
   * Which namespace does this flow node belong to?
   * - a namespace node is its own namespace, keyed by its label
   * - any other namespaced node carries metadata.namespace
   * - a collapsed type group resolves only when every member shares one
   *   namespace; a group spanning namespaces stays unlabelled so it dims
   *   rather than falsely claiming to be in one place
   */
  function applySpotlight(
    allNodes: Node[],
    key: FilterKey | null,
    mode: 'map' | 'grouped',
  ): Node[] {
    // Namespace focus is a real graph filter (see the build effect), not a
    // dim, so only the type spotlight has any say over opacity here.
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

  // ── Search + reveal ────────────────────────────────────────────────────────
  // Searches the whole graph, not just the currently visible slice, so a
  // node hidden by a filter is still findable. Selecting a hidden match
  // activates its category and expands its type group so it renders, then
  // pans to it.
  const [searchQuery, setSearchQuery] = useState('')
  const [searchFocused, setSearchFocused] = useState(false)
  const pendingFocusId = useRef<string | null>(null)

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
    // A non-singleton type renders as namespace-scoped groups unless the whole
    // type is expanded (which collapses every scope at once), or this specific
    // node was picked out of its group.
    if (!expandedGroups.has(`group:${node.type}`)) {
      return !expandedNodeIds.has(node.id)
    }
    return false
  }

  function revealAndFocus(node: GraphNode) {
    // Namespace focus filters the graph, so a match sitting in another
    // namespace would never render. Clear the focus rather than silently
    // panning to nothing.
    const ns = node.metadata?.namespace
    if (nsFocus && typeof ns === 'string' && ns && ns !== nsFocus) setNsFocus(null)

    const cat = TYPE_TO_CATEGORY[node.type]
    if (cat) setActiveFilters((prev) => (prev.has(cat) ? prev : new Set([...prev, cat])))
    // A non-singleton type only renders individual nodes once its group is
    // expanded, so expanding here is what actually makes the node appear.
    if (!SINGLETON_TYPES.has(node.type) && node.type !== 'cluster') {
      setExpandedGroups((prev) => new Set([...prev, `group:${node.type}`]))
    }
    setSpotlightKey(null)
    pendingFocusId.current = node.id
  }

  // Centre on the node once React has actually rendered it. Runs after the
  // filter/group changes above commit, which is when the node exists.
  useEffect(() => {
    const id = pendingFocusId.current
    if (!id || !rfRef.current) return
    const rfNode = rfRef.current.getNode(id)
    if (!rfNode) return // not mounted yet; the next render will retry
    pendingFocusId.current = null
    setSelectedNodeId(id)
    rfRef.current.setCenter(rfNode.position.x + (rfNode.width ?? 0) / 2,
      rfNode.position.y + (rfNode.height ?? 0) / 2, { zoom: 1.1, duration: 500 })
  }, [nodes, selectedNodeId])

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

  // Clear isRefreshing when a new snapshot arrives after refresh was requested
  const snapshotTimestamp = vm.graph?.snapshot?.timestamp
  useEffect(() => {
    if (refreshingRef.current) {
      refreshingRef.current = false
      setIsRefreshing(false)
    }
  }, [snapshotTimestamp])

  function handleRefresh() {
    refreshingRef.current = true
    setIsRefreshing(true)
    sendCommand(vm.code, 'refresh')
    // Fallback: clear after 30s if no snapshot arrives
    setTimeout(() => {
      if (refreshingRef.current) {
        refreshingRef.current = false
        setIsRefreshing(false)
      }
    }, 30_000)
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
        setShowLogs(false)
      }
    } else {
      // A namespace is a container, not a thing with an inspector: its detail
      // panel is a single metadata row and nothing actionable. Clicking one
      // filters the canvas down to it instead of opening that empty panel.
      if (node.type === 'namespaceGroup') {
        const ns = (node.data as InfraNodeData)?.label
        if (ns) setNsFocus((prev) => (prev === ns ? null : ns))
        setDrawerGroup(null)
        setSelectedNodeId(null)
        setShowLogs(false)
        return
      }
      setDrawerGroup(null)
      setSelectedNodeId((prev) => {
        if (prev !== node.id) { setShowLogs(false) }
        return node.id
      })
    }
  }, [groupsMap])

  function handleSelectNodeFromDrawer(nodeId: string) {
    // Open the node detail panel for this specific item — same setters
    // handleNodeClick uses for a direct canvas click. The drawer must close:
    // it and NodeDetailPanel dock in the same right-side slot.
    // Picking a namespace from the drawer focuses it on the canvas, same as
    // clicking the namespace node — no near-empty inspector for a container.
    const raw = vm.graph?.nodes.find((n) => n.id === nodeId)
    if (raw?.type === 'namespace') {
      setNsFocus((prev) => (prev === raw.label ? null : raw.label))
      setDrawerGroup(null)
      setSelectedNodeId(null)
      setShowLogs(false)
      return
    }
    setDrawerGroup(null)
    setSelectedNodeId(nodeId)
    setShowLogs(false)
  }

  function handlePaneClick() {
    setSelectedNodeId(null)
    setDrawerGroup(null)
    setSpotlightKey(null)
    setShowLogs(false)
  }

  // ── Export helpers ─────────────────────────────────────────────────────────
  async function handleExportPNG() {
    if (!canvasWrapRef.current) return
    try {
      const { toPng } = await import('html-to-image')
      const dataUrl = await toPng(canvasWrapRef.current, { backgroundColor: '#08080E', pixelRatio: 2 })
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
    <div style={{ width: '100%', height: '100%', display: 'flex', flexDirection: 'column', background: 'var(--bg)', overflow: 'hidden' }}>

      {/* ── Top bar ──────────────────────────────────────────────── */}
      <div style={{
        flexShrink: 0, display: 'flex', alignItems: 'center', gap: 8,
        padding: '0 14px', height: 46, zIndex: 10,
        // The title truncates so a long agent id never forces the bar wider
        // (that was what made it scroll). Kept as a fallback rather than
        // `hidden`: hiding silently clipped Export/Refresh/chips out of
        // reach on narrower windows instead of letting them scroll.
        overflowX: 'auto',
        overflowY: 'hidden',
        background: 'var(--bg)', borderBottom: '1px solid var(--line2)',
      }}>
        {onBack && (
          <>
            <button onClick={onBack} style={ICON_BTN}
              onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--line)'; e.currentTarget.style.color = 'var(--ink2)' }}
              onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--ink4)' }}>
              <ArrowLeft size={15} />
            </button>
            <div style={DIVIDER} />
          </>
        )}

        <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0, flexShrink: 1 }}>
          <Server size={13} color="var(--ink3)" style={{ flexShrink: 0 }} />
          <span
            title={vm.hostname ?? vm.code}
            style={{
              fontSize: 13, fontWeight: 600, color: 'var(--ink)', letterSpacing: '-0.015em',
              // Truncate rather than wrap: a 36-char agent id used to break the
              // header onto two lines and push the toolbar out of view.
              whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
              minWidth: 0, maxWidth: 220,
            }}
          >
            {vm.hostname ?? vm.code}
          </span>
          {/* Only worth showing when it adds information. Cluster connections
              have no hostname, so both slots rendered the same id twice. */}
          {vm.hostname && vm.hostname !== vm.code && (
            <span
              title={vm.code}
              style={{
                fontSize: 10, padding: '1px 7px', borderRadius: 20, background: 'var(--line)',
                color: 'var(--ink3)', border: '1px solid #2A2A2A',
                fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
                maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                flexShrink: 0,
              }}
            >
              {vm.code}
            </span>
          )}
        </div>

        {stats && (
          <>
            <div style={DIVIDER} />
            <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: 'var(--ink4)' }}>
              <GitBranch size={11} /><span style={{ color: 'var(--ink2)' }}>{stats.totalNodes}</span> nodes
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: 'var(--ink4)' }}>
              <Layers size={11} /><span style={{ color: 'var(--ink2)' }}>{stats.totalEdges}</span> edges
            </span>
            {snapshot?.collectionDuration != null && (
              <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: 'var(--ink4)' }}>
                <Clock size={11} /><span style={{ color: 'var(--ink2)' }}>{fmt(snapshot.collectionDuration)}</span>
              </span>
            )}
          </>
        )}

        <div style={{ flex: 1 }} />

        {/* Spotlight hint */}
        {spotlightKey && (
          <span style={{ fontSize: 10, color: 'var(--ink2)', display: 'flex', alignItems: 'center', gap: 5 }}>
            <span style={{ width: 5, height: 5, borderRadius: '50%', background: 'var(--ink)', display: 'inline-block' }} />
            Spotlight: {FILTER_GROUPS[spotlightKey].label}
            <button onClick={() => setSpotlightKey(null)} style={{ background: 'none', border: 'none', color: 'var(--ink3)', cursor: 'pointer', padding: 0, fontSize: 11 }}>✕</button>
          </span>
        )}

        {/* View mode toggle */}
        <div style={{ display: 'flex', gap: 1, background: 'var(--surface)', border: '1px solid #1E1E1E', borderRadius: 7, padding: 2 }}>
          {([
            ['map', 'Map', 'Radial map: root, type hubs, resources'],
            ['grouped', 'Grouped', 'Namespace tree with per-type counts'],
          ] as const).map(([m, Label, hint]) => (
            <button key={m} onClick={() => setViewMode(m)} title={hint}
              style={{
                display: 'flex', alignItems: 'center', gap: 4,
                padding: '3px 9px', borderRadius: 5, border: 'none', cursor: 'pointer',
                fontSize: 11, fontWeight: viewMode === m ? 600 : 400,
                background: viewMode === m ? 'var(--line2)' : 'transparent',
                color: viewMode === m ? 'var(--ink)' : 'var(--ink4)',
                transition: 'all 0.1s',
              }}>
              {m === 'map' ? <Waypoints size={11} /> : <Rows3 size={11} />}
              {Label}
            </button>
          ))}
        </div>

        {/* Search — searches the whole graph, including nodes the current
            filter is hiding, and reveals the match when selected. */}
        <div style={{ position: 'relative' }}>
          <div style={{
            display: 'flex', alignItems: 'center', gap: 6,
            background: 'var(--bg)', border: '1px solid var(--line)', borderRadius: 8,
            padding: '4px 8px',
            // Shrinks before the chips do, so a narrow window compresses the
            // search box rather than pushing Export off the end of the bar.
            width: 'auto', minWidth: 110, maxWidth: 180, flex: '0 1 180px',
          }}>
            <Search size={12} color="var(--ink4)" />
            <input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onFocus={() => setSearchFocused(true)}
              onBlur={() => setTimeout(() => setSearchFocused(false), 150)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && searchMatches.length > 0) {
                  setSearchQuery(''); setSearchFocused(false); revealAndFocus(searchMatches[0])
                }
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
              position: 'absolute', top: 'calc(100% + 4px)', left: 0, width: 300, maxHeight: 320,
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
                      onMouseDown={() => { setSearchQuery(''); setSearchFocused(false); revealAndFocus(n) }}
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

        {/* Namespace focus indicator. Dimming the canvas is invisible
            feedback unless you say what caused it. */}
        {nsFocus && (
          <button
            onClick={() => setNsFocus(null)}
            title={`Canvas is focused on namespace "${nsFocus}". Click to clear.`}
            style={{
              display: 'inline-flex', alignItems: 'center', gap: 6,
              padding: '3px 9px', borderRadius: 6, fontSize: 11, cursor: 'pointer',
              background: 'var(--line)', color: 'var(--ink)',
              border: '1px solid var(--line2)', transition: 'all 0.1s',
            }}
          >
            <span style={{ color: 'var(--ink4)' }}>ns</span>
            <span style={{ fontWeight: 600 }}>{nsFocus}</span>
            <X size={10} color="var(--ink3)" />
          </button>
        )}

        {/* Filter chips — every chip uses identical styling. The previous two
            rows styled the second group with --ink3 text and a dashed border,
            which read as disabled even though the chips were live. Differing
            by position only. */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <Filter size={11} color="var(--ink4)" />
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
                    background: spotlit ? 'var(--ink)' : active ? 'var(--line)' : 'transparent',
                    color: spotlit ? 'var(--bg)' : active ? 'var(--ink)' : 'var(--ink4)',
                    border: `1px solid ${spotlit ? 'var(--ink)' : active ? 'var(--line2)' : 'var(--line)'}`,
                    transition: 'all 0.1s',
                  }}>
                  {g.label}
                  {/* Counts on every chip. A chip with no number gives no
                      reason to click it — the number is the call to action. */}
                  <span style={{
                    fontSize: 10, fontVariantNumeric: 'tabular-nums',
                    color: spotlit ? 'var(--bg)' : active ? 'var(--ink3)' : count ? 'var(--ink3)' : 'var(--line2)',
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
          style={{ display: 'flex', alignItems: 'center', gap: 5, padding: '4px 10px', borderRadius: 7, border: '1px solid #1E1E1E', background: 'transparent', color: 'var(--ink3)', fontSize: 11, cursor: 'pointer', transition: 'all 0.1s' }}
          onMouseEnter={(e) => { e.currentTarget.style.borderColor = 'var(--line2)'; e.currentTarget.style.color = 'var(--ink)' }}
          onMouseLeave={(e) => { e.currentTarget.style.borderColor = 'var(--line)'; e.currentTarget.style.color = 'var(--ink3)' }}>
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
            style={{ display: 'flex', alignItems: 'center', gap: 5, padding: '4px 10px', borderRadius: 7, border: '1px solid #1E1E1E', background: 'transparent', color: 'var(--ink3)', fontSize: 11, cursor: 'pointer', transition: 'all 0.1s' }}
            onMouseEnter={(e) => { e.currentTarget.style.borderColor = 'var(--line2)'; e.currentTarget.style.color = 'var(--ink)' }}
            onMouseLeave={(e) => { e.currentTarget.style.borderColor = 'var(--line)'; e.currentTarget.style.color = 'var(--ink3)' }}>
            <Download size={12} /> Export
          </button>
          <div id="export-menu" style={{ display: 'none', flexDirection: 'column', position: 'absolute', top: '100%', right: 0, marginTop: 4, background: 'var(--surface)', border: '1px solid var(--line2)', borderRadius: 10, overflow: 'hidden', minWidth: 130, zIndex: 50 }}>
            {[
              { label: 'Export PNG', action: () => { handleExportPNG(); const m = document.getElementById('export-menu'); if (m) m.style.display = 'none' } },
              { label: 'Export JSON', action: () => { handleExportJSON(); const m = document.getElementById('export-menu'); if (m) m.style.display = 'none' } },
            ].map(({ label, action }) => (
              <button key={label} onClick={action}
                style={{ padding: '9px 14px', background: 'transparent', border: 'none', color: 'var(--ink3)', fontSize: 12, cursor: 'pointer', textAlign: 'left', width: '100%', transition: 'all 0.1s' }}
                onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--line)'; e.currentTarget.style.color = 'var(--ink)' }}
                onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--ink3)' }}>
                {label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* ── Critical alert banner ─────────────────────────────────── */}
      {criticalGroups.length > 0 && !alertDismissed && (
        <div style={{
          flexShrink: 0,
          background: 'var(--surface)',
          borderBottom: '1px solid #2A2A2A',
          padding: '6px 16px',
          display: 'flex', alignItems: 'center', gap: 10,
        }}>
          <AlertTriangle size={13} color="var(--ink3)" style={{ flexShrink: 0 }} />
          <div style={{ flex: 1, display: 'flex', gap: 14, flexWrap: 'wrap', alignItems: 'center' }}>
            {criticalGroups.map((a) => (
              <button
                key={a.id}
                onClick={() => { const g = groupsMap.get(a.id); if (g) { setDrawerGroup(g); setSelectedNodeId(null) } }}
                style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, fontSize: 11, color: 'var(--ink2)' }}
              >
                <strong>{a.label}</strong>: {a.degraded} degraded
              </button>
            ))}
          </div>
          <button
            onClick={() => setHideDegraded((v) => !v)}
            style={{
              fontSize: 11, padding: '2px 8px', borderRadius: 4, cursor: 'pointer',
              background: hideDegraded ? 'var(--line)' : 'transparent',
              border: '1px solid #2A2A2A', color: 'var(--ink2)',
            }}
          >
            {hideDegraded ? 'Show degraded' : 'Hide degraded'}
          </button>
          <button
            onClick={() => setAlertDismissed(true)}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--ink4)', padding: '2px 4px', fontSize: 14, lineHeight: 1 }}
            title="Dismiss"
          >
            ×
          </button>
        </div>
      )}

      {/* ── Canvas ───────────────────────────────────────────────── */}
      <div ref={canvasWrapRef} style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
        {!vm.graph ? (
          <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 14 }}>
              <div style={{ width: 28, height: 28, borderRadius: '50%', border: '1.5px solid #1E1E1E', borderTopColor: 'var(--ink)' }} className="animate-spin" />
              <p style={{ fontSize: 12, color: 'var(--ink4)', fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)' }}>Loading graph…</p>
            </div>
          </div>
        ) : nodes.length === 0 ? (
          <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <div style={{ textAlign: 'center' }}>
              <p style={{ fontSize: 14, fontWeight: 500, color: 'var(--ink)', marginBottom: 6 }}>No nodes to display</p>
              <p style={{ fontSize: 12, color: 'var(--ink4)' }}>Enable more filter categories above</p>
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
            <Background variant={BackgroundVariant.Dots} gap={24} size={1} color="rgba(255,255,255,0.04)" />
            <Controls style={{ bottom: 80, left: 16 }} showInteractive={false} />
            <MiniMap
              nodeColor={() => 'var(--line)'}
              maskColor="rgba(0,0,0,0.4)"
              style={{ bottom: 16, right: 16, width: 160, height: 100 }}
            />
          </ReactFlow>
        )}

        {drawerGroup && (
          <GroupDrawer
            group={drawerGroup}
            onClose={() => setDrawerGroup(null)}
            onSelectNode={handleSelectNodeFromDrawer}
            onFocusNamespace={setNsFocus}
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

        {selectedNode && !drawerGroup && !showLogs && (
          <>
            {/* Drag resize handle — sits on left edge of panel */}
            <div
              onMouseDown={onResizeMouseDown}
              style={{
                position: 'absolute', right: panelWidth - 3, top: 0, bottom: 0, width: 6, zIndex: 40,
                cursor: 'col-resize',
                background: 'transparent',
                transition: 'background 0.1s',
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = 'rgba(250,250,250,0.15)' }}
              onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
            />
            <NodeDetailPanel
              node={selectedNode}
              vmCode={vm.code}
              width={panelWidth}
              onClose={() => { setSelectedNodeId(null); setShowLogs(false) }}
              onShowLogs={['container', 'pod', 'host', 'deployment', 'statefulset', 'daemonset', 'service'].includes(selectedNode.type) ? () => { setShowLogs(true) } : undefined}
              onShowTerminal={['container', 'host', 'pod'].includes(selectedNode.type) ? () => {
                const layer = selectedNode.type === 'host' ? 'host' : selectedNode.type === 'pod' ? 'kubernetes' : 'docker'
                openTerminal({
                  label: selectedNode.label,
                  vmLabel: vm.code,
                  vmCode: vm.code,
                  node: selectedNode,
                  layer,
                  namespace: selectedNode.metadata?.namespace,
                })
              } : undefined}
            />
          </>
        )}

        {selectedNode && showLogs && (
          <LogsPanel
            node={selectedNode}
            vmCode={vm.code}
            panelWidth={0}
            onClose={() => setShowLogs(false)}
            allNodes={vm.graph?.nodes}
            allEdges={vm.graph?.edges}
          />
        )}
      </div>

      {/* ── Status bar ───────────────────────────────────────────── */}
      <div style={{
        flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '0 14px', height: 22, background: 'var(--bg)', borderTop: '1px solid #1E1E1E',
        fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
          {viewMode === 'grouped' && groupsMap.size > 0 && (
            <span style={{ fontSize: 10, color: 'var(--line2)' }}>
              {groupsMap.size} groups · click to drill down
            </span>
          )}
          {stats && Object.entries(stats.nodesByType)
            .sort(([, a], [, b]) => b - a).slice(0, 5)
            .map(([type, count]) => (
              <span key={type} style={{ fontSize: 10, color: 'var(--line3)' }}>
                {type}:{count}
              </span>
            ))}
        </div>
        {snapshot?.timestamp && (
          <span style={{ fontSize: 10, color: 'var(--line2)' }}>
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
  background: 'transparent', color: 'var(--ink4)', cursor: 'pointer',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  transition: 'background 0.1s, color 0.1s',
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
    sec < 5    ? 'just now' :
    sec < 60   ? `${sec}s ago` :
    sec < 3600 ? `${Math.floor(sec / 60)}m ago` :
                 `${Math.floor(sec / 3600)}h ago`
  const stale = sec >= 60
  return (
    <span
      title={`Last updated ${new Date(ts).toLocaleTimeString()}`}
      style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 10, color: stale ? 'var(--ink2)' : 'var(--ink4)', fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)' }}
    >
      <Clock size={10} />
      {label}
    </span>
  )
}
