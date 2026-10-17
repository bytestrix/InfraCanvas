/**
 * graphPreprocess.ts
 *
 * Transforms raw GraphNode/GraphEdge data into a "display graph" for React Flow.
 *
 * Two modes:
 *   flat    — one React Flow node per raw node (current behaviour)
 *   grouped — one GroupNode per type, edges bundled with ×N counts
 *
 * The raw data is never mutated — grouping/bundling happens here, drill-down
 * uses the raw nodes stored in GroupInfo.nodes.
 */

import { MarkerType, type Node, type Edge } from 'reactflow'
import { type GraphNode, type GraphEdge, type NodeHealth, getNodeColor, getNodeIcon } from '@/types'
import { type InfraNodeData } from '@/components/canvas/InfraNode'
import { NS_HEADER_W, NS_HEADER_H } from '@/components/canvas/NamespaceGroupNode'

// ─── Types ────────────────────────────────────────────────────────────────────

export interface HealthCounts {
  healthy: number
  degraded: number
  unhealthy: number
  unknown: number
}

export interface GroupInfo {
  id: string          // "group:container" or "group:pod@kube-system"
  /** Group key: `type`, or `type@namespace` for namespaced resources. */
  key: string
  type: string        // "container" / "pod"
  label: string       // "Containers" / "Pods"
  count: number
  healthCounts: HealthCounts
  nodes: GraphNode[]
  /** Owning namespace, or '' when this group is not namespaced. */
  namespace: string
}

export interface GroupNodeData {
  groupType: string
  /** Group key: `type`, or `type@namespace`. Unique per canvas node. */
  groupKey: string
  label: string
  count: number
  healthCounts: HealthCounts
  color: string
  icon: string
  isCritical: boolean  // >50% degraded/unhealthy → pulse dot
  /** Owning namespace, shown as a subtitle so the node states where it lives. */
  namespace: string
  /** Opens the "•••" drill-down menu listing this group's individual members. */
  onOpenPicker?: (nodeId: string, e: React.MouseEvent) => void
}

// ─── Constants ────────────────────────────────────────────────────────────────

/** Types that always render as individual nodes (logically unique / 1 instance) */
export const SINGLETON_TYPES = new Set([
  'host',
  'cluster',
  'container_runtime',
  'image_group',
  // Namespaces are the anchors the k8s hierarchy hangs off. Bundling them
  // into one "Namespaces ×3" node is what forced every namespace-scoped
  // resource into a flat type group, because the parent that explained
  // ownership was itself hidden inside a group. They render individually.
  'namespace',
])

/**
 * Types that live inside a Kubernetes namespace.
 *
 * Grouping these by type alone is what made the canvas unreadable: 9 pods
 * spread over 2 namespaces collapsed into one `Pods ×9` node, so the split
 * (8 in kube-system, 1 in local-path-storage) was only visible by opening a
 * side drawer. Scoping the group key by namespace makes the canvas show the
 * real shape of the cluster at a glance.
 */
export const NAMESPACE_SCOPED_TYPES = new Set([
  'pod',
  'deployment',
  'statefulset',
  'daemonset',
  'job',
  'cronjob',
  'k8s_service',
  'ingress',
  'pvc',
])

/** Namespace a node belongs to, or '' when it is not namespaced. */
export function namespaceOf(node: GraphNode): string {
  const ns = node.metadata?.namespace
  return typeof ns === 'string' && ns ? ns : ''
}

/**
 * Group key for a type: `type` for global resources, `type@namespace` for
 * namespaced ones. Scoped groups nest under their namespace node.
 */
export function groupKeyFor(type: string, namespace: string): string {
  return NAMESPACE_SCOPED_TYPES.has(type) && namespace ? `${type}@${namespace}` : type
}

/** Group id (canvas-level) for a group key. Unscoped types keep the legacy
 *  `group:<type>` form so existing expansion state keeps working. */
export function groupIdFor(key: string): string {
  return `group:${key}`
}

/**
 * The "ctx:<name>/" cluster prefix an entity id carries, or '' for the
 * primary cluster. Mirrors the backend multi-cluster ID scheme, so the same
 * namespace name in two clusters stays two distinct namespaces instead of
 * being merged into one group.
 */
function clusterKeyOf(id: string): string {
  if (id.startsWith('ctx:')) {
    const i = id.indexOf('/')
    if (i >= 0) return id.slice(0, i + 1)
  }
  return ''
}

const TYPE_LABELS: Record<string, string> = {
  host: 'Host',
  container_runtime: 'Container Runtime',
  container: 'Containers',
  image: 'Images',
  image_group: 'Build Cache',
  volume: 'Volumes',
  network: 'Networks',
  pod: 'Pods',
  deployment: 'Deployments',
  statefulset: 'StatefulSets',
  daemonset: 'DaemonSets',
  job: 'Jobs',
  cronjob: 'CronJobs',
  k8s_service: 'K8s Services',
  ingress: 'Ingresses',
  namespace: 'Namespaces',
  node: 'K8s Nodes',
  cluster: 'Cluster',
  pvc: 'PVCs',
  event: 'Events',
  service: 'Services',
  process: 'Processes',
}

export const GROUP_NODE_WIDTH = 260
export const GROUP_NODE_HEIGHT = 92

// ─── Helpers ──────────────────────────────────────────────────────────────────

export function getGroupLabel(type: string): string {
  return TYPE_LABELS[type] ?? type
}

/**
 * Normalize health for Docker entities: containers whose raw health is 'unknown'
 * but metadata.state === 'running' should count as healthy.
 */
function normalizeNodeHealth(node: GraphNode): NodeHealth {
  if (node.health !== 'unknown') return node.health as NodeHealth

  // Docker containers: infer from state
  if (node.type === 'container') {
    const state = String(node.metadata?.state ?? '').toLowerCase()
    if (state === 'running') return 'healthy'
    if (state === 'exited' || state === 'dead' || state === 'oomkilled') return 'degraded'
  }

  // Images are always healthy (they exist, they're not "running")
  if (node.type === 'image' || node.type === 'image_group') return 'healthy'

  // Volumes and networks are passive — treat as healthy
  if (node.type === 'volume' || node.type === 'network') return 'healthy'

  return 'unknown'
}

export function countHealth(nodes: GraphNode[]): HealthCounts {
  const c = { healthy: 0, degraded: 0, unhealthy: 0, unknown: 0 }
  for (const n of nodes) {
    const h = normalizeNodeHealth(n)
    if (h === 'healthy') c.healthy++
    else if (h === 'degraded') c.degraded++
    else if (h === 'unhealthy') c.unhealthy++
    else c.unknown++
  }
  return c
}

/** Dominant health of a group (worst wins) */
export function groupHealth(hc: HealthCounts): NodeHealth {
  if (hc.unhealthy > 0) return 'unhealthy'
  if (hc.degraded > 0) return 'degraded'
  if (hc.healthy > 0) return 'healthy'
  return 'unknown'
}

/** Returns true when more than half the group is degraded or unhealthy */
export function isCriticalGroup(hc: HealthCounts, total: number): boolean {
  return total > 0 && (hc.degraded + hc.unhealthy) / total > 0.5
}

// ─── Main function ────────────────────────────────────────────────────────────

/**
 * Build a grouped display graph.
 *
 * @param rawNodes       All nodes from the agent snapshot
 * @param rawEdges       All edges from the agent snapshot
 * @param visibleTypes   Which node types to include (from filter toggles)
 * @param expandedGroups Group IDs whose individual nodes are shown on canvas
 *
 * @returns React Flow nodes + edges + GroupInfo map for drawer drill-down
 */
export function buildGroupedGraph(
  rawNodes: GraphNode[],
  rawEdges: GraphEdge[],
  visibleTypes: Set<string>,
  expandedGroups: Set<string>,
  /** Specific raw node IDs to always render individually, even when their
   *  type as a whole is still collapsed. Powers the per-member "•••" picker
   *  on GroupNode — pick 2 of 11 pods, only those 2 pop out, the other 9
   *  stay bundled under a smaller group node. */
  expandedNodeIds: Set<string> = new Set(),
): {
  nodes: Node[]
  edges: Edge[]
  groups: Map<string, GroupInfo>
} {
  // 1. Filter to visible types
  const filtered = rawNodes.filter((n) => visibleTypes.has(n.type))

  // 2. Bucket by group key. Namespaced resources bucket by `type@namespace`
  //    so each namespace owns its own child groups; everything else buckets
  //    by type alone, exactly as before.
  const byKey = new Map<string, GraphNode[]>()
  for (const n of filtered) {
    const key = groupKeyFor(n.type, namespaceOf(n))
    const bucket = byKey.get(key) ?? []
    bucket.push(n)
    byKey.set(key, bucket)
  }

  // 3. Decide: individual vs group
  const groups = new Map<string, GroupInfo>()
  const individualIds = new Set<string>()

  for (const [key, nodes] of byKey) {
    const type = nodes[0].type
    const namespace = namespaceOf(nodes[0])
    const gid = groupIdFor(key)
    const singleton = SINGLETON_TYPES.has(type) || nodes.length === 1
    // A whole type being expanded still expands every one of its scoped
    // groups — `group:pod` is the user-facing switch, the scoped ids are the
    // internal bookkeeping.
    const wholeTypeExpanded = expandedGroups.has(`group:${type}`) || expandedGroups.has(gid)

    if (singleton || wholeTypeExpanded) {
      for (const n of nodes) individualIds.add(n.id)
      continue
    }

    // Partial expansion: specific members picked via the group's "•••" menu
    // pop out individually; the rest stay bundled under a (smaller) group node.
    const picked = nodes.filter((n) => expandedNodeIds.has(n.id))
    const remaining = nodes.filter((n) => !expandedNodeIds.has(n.id))
    for (const n of picked) individualIds.add(n.id)

    if (remaining.length > 0) {
      const hc = countHealth(remaining)
      groups.set(gid, {
        id: gid,
        key,
        type,
        label: getGroupLabel(type),
        count: remaining.length,
        healthCounts: hc,
        nodes: remaining,
        namespace,
      })
    }
  }

  // 4. Build React Flow node array
  const flowNodes: Node[] = []

  // Individual nodes
  for (const n of filtered) {
    if (!individualIds.has(n.id)) continue

    // A namespace header states what it is parent of. Counting the scoped
    // groups built above is enough — `Pods ×8` contributes one group and
    // eight resources — and a namespace holding a single pod has no group at
    // all (it renders the pod by name), so count those directly too.
    let childSummary: { groups: number; resources: number } | undefined
    if (n.type === 'namespace') {
      let groupCount = 0
      let resourceCount = 0
      for (const [, g] of groups) {
        if (g.namespace !== n.label) continue
        groupCount++
        resourceCount += g.count
      }
      for (const id of individualIds) {
        if (id === n.id) continue
        const c = filtered.find((x) => x.id === id)
        if (c && namespaceOf(c) === n.label) resourceCount++
      }
      childSummary = { groups: groupCount, resources: resourceCount }
    }

    const isNs = n.type === 'namespace'
    flowNodes.push({
      id: n.id,
      type: isNs ? 'namespaceGroup' : 'infraNode',
      position: { x: 0, y: 0 },
      data: {
        nodeType: n.type,
        label: n.label,
        health: normalizeNodeHealth(n),
        metadata: n.metadata,
        ...(childSummary ? { childSummary } : {}),
      } as InfraNodeData,
      width: isNs ? NS_HEADER_W : 220,
      height: isNs ? NS_HEADER_H : 100,
    })
  }

  // Group nodes
  for (const [, g] of groups) {
    flowNodes.push({
      id: g.id,
      type: 'groupNode',
      position: { x: 0, y: 0 },
      data: {
        groupType: g.type,
        groupKey: g.key,
        label: g.label,
        count: g.count,
        healthCounts: g.healthCounts,
        color: getNodeColor(g.type),
        icon: getNodeIcon(g.type),
        isCritical: isCriticalGroup(g.healthCounts, g.count),
        namespace: g.namespace,
      } as GroupNodeData,
      width: GROUP_NODE_WIDTH,
      height: GROUP_NODE_HEIGHT,
    })
  }

  // 5. Build canvas ID map (raw nodeId → canvas nodeId)
  const toCanvas = new Map<string, string>()
  for (const id of individualIds) toCanvas.set(id, id)
  for (const g of groups.values()) {
    for (const n of g.nodes) toCanvas.set(n.id, g.id)
  }

  const canvasIds = new Set(flowNodes.map((n) => n.id))

  // 6. Build bundled edges — bridging THROUGH hidden nodes to the nearest
  // visible one, instead of dropping the edge outright.
  //
  // Only revealing "cluster" + "deployment" (skipping "namespace") used to
  // sever every edge, because the raw chain is
  // cluster --CONTAINS--> namespace --CONTAINS--> deployment
  // and namespace has no canvas node once its type isn't revealed. We walk
  // past invisible nodes along the raw edge graph (forward for a hidden
  // target, backward for a hidden source) until we reach something that
  // does have a canvas id, so "Deployments" still lands under "kubernetes"
  // even with namespaces folded away.
  const outAdj = new Map<string, { target: string; type: string }[]>()
  const inAdj = new Map<string, { source: string; type: string }[]>()
  for (const e of rawEdges) {
    ;(outAdj.get(e.source) ?? outAdj.set(e.source, []).get(e.source)!).push({ target: e.target, type: e.type })
    ;(inAdj.get(e.target) ?? inAdj.set(e.target, []).get(e.target)!).push({ source: e.source, type: e.type })
  }

  function resolveForward(id: string, visited = new Set<string>()): string[] {
    const canvas = toCanvas.get(id)
    if (canvas) return [canvas]
    if (visited.has(id)) return []
    visited.add(id)
    const out: string[] = []
    for (const { target } of outAdj.get(id) ?? []) out.push(...resolveForward(target, visited))
    return [...new Set(out)]
  }

  function resolveBackward(id: string, visited = new Set<string>()): string[] {
    const canvas = toCanvas.get(id)
    if (canvas) return [canvas]
    if (visited.has(id)) return []
    visited.add(id)
    const out: string[] = []
    for (const { source } of inAdj.get(id) ?? []) out.push(...resolveBackward(source, visited))
    return [...new Set(out)]
  }

  // Key: "src→tgt→edgeType"  → accumulate count
  const bundle = new Map<
    string,
    { source: string; target: string; edgeType: string; count: number }
  >()

  for (const e of rawEdges) {
    const srcIds = toCanvas.has(e.source) ? [toCanvas.get(e.source)!] : resolveBackward(e.source)
    const tgtIds = toCanvas.has(e.target) ? [toCanvas.get(e.target)!] : resolveForward(e.target)
    for (const src of srcIds) {
      for (const tgt of tgtIds) {
        if (!src || !tgt || src === tgt) continue
        if (!canvasIds.has(src) || !canvasIds.has(tgt)) continue
        const key = `${src}→${tgt}→${e.type}`
        if (bundle.has(key)) {
          bundle.get(key)!.count++
        } else {
          bundle.set(key, { source: src, target: tgt, edgeType: e.type, count: 1 })
        }
      }
    }
  }

  // Scoped groups hang off the namespace that owns them. The raw
  // `namespace → pod` CONTAINS edges usually carry this on their own once
  // each pod maps to a namespace-scoped canvas id, but discovery data is not
  // guaranteed to emit them for every resource type, so the ownership edge is
  // stated explicitly. Without it a `Pods ×8` node sits on the canvas with
  // nothing indicating it belongs to kube-system.
  //
  // Runs *after* the raw-edge loop on purpose: inserting first would let the
  // real edges increment the synthetic entry and inflate the bundled `×N`.
  if (filtered.some((n) => n.type === 'namespace')) {
    // namespace label + cluster prefix → its canvas node id
    const nsCanvasByName = new Map<string, string>()
    for (const n of filtered) {
      if (n.type !== 'namespace') continue
      nsCanvasByName.set(clusterKeyOf(n.id) + ' ' + String(n.label ?? ''), n.id)
    }

    for (const [, g] of groups) {
      if (!g.namespace) continue
      const nsId = nsCanvasByName.get(clusterKeyOf(g.nodes[0].id) + ' ' + g.namespace)
      if (!nsId || nsId === g.id || !canvasIds.has(nsId)) continue
      const key = `${nsId}→${g.id}→CONTAINS`
      if (!bundle.has(key)) {
        bundle.set(key, { source: nsId, target: g.id, edgeType: 'CONTAINS', count: 1 })
      }
    }
  }

  // 7. Add synthetic bridge edges between islands that are logically connected
  //    but have no direct edge in the raw data (host ↔ cluster, host ↔ runtime).
  //    These help dagre produce a single connected tree.
  const syntheticBridges: Array<[string, string, string]> = [
    // [source canvas ID fragment, target canvas ID fragment, label]
    ['container_runtime', 'host', 'RUNS_ON'],
    ['cluster', 'host', 'RUNS_ON'],
  ]

  for (const [srcFrag, tgtFrag, label] of syntheticBridges) {
    // Find actual canvas node IDs that match the fragment
    const srcId = [...canvasIds].find((id) => id === srcFrag || id.startsWith(`${srcFrag}:`))
    const tgtId = [...canvasIds].find((id) => id === tgtFrag || id.startsWith(`${tgtFrag}:`))
    if (!srcId || !tgtId) continue

    // Only add if no edge (in either direction) already exists between them
    const fwdKey = `${srcId}→${tgtId}→${label}`
    const revKey = `${tgtId}→${srcId}→${label}`
    const alreadyConnected = [...bundle.keys()].some(
      (k) => k.startsWith(`${srcId}→${tgtId}`) || k.startsWith(`${tgtId}→${srcId}`)
    )
    if (!alreadyConnected) {
      bundle.set(fwdKey, { source: srcId, target: tgtId, edgeType: label, count: 1 })
    }
  }

  // 8. Convert bundle to React Flow edges
  const flowEdges: Edge[] = []
  for (const [key, b] of bundle) {
    const isBundled = b.count > 1
    const label = isBundled ? `${b.edgeType} ×${b.count}` : b.edgeType
    flowEdges.push({
      id: key,
      source: b.source,
      target: b.target,
      label,
      type: 'smoothstep',
      markerEnd: {
        type: MarkerType.ArrowClosed,
        width: 8,
        height: 8,
        color: isBundled ? '#4b5280' : '#2d2d52',
      },
      style: {
        stroke: isBundled ? '#4b5280' : '#2d2d52',
        strokeWidth: isBundled ? 2 : 1.5,
      },
      labelStyle: {
        fill: isBundled ? '#818cf8' : '#475569',
        fontSize: 9,
        fontWeight: isBundled ? 600 : 400,
        fontFamily: 'JetBrains Mono, monospace',
      },
      labelBgStyle: { fill: '#070711', fillOpacity: 0.9 },
    })
  }

  return { nodes: flowNodes, edges: flowEdges, groups }
}
