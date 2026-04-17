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

// ─── Types ────────────────────────────────────────────────────────────────────

export interface HealthCounts {
  healthy: number
  degraded: number
  unhealthy: number
  unknown: number
}

export interface GroupInfo {
  id: string          // "group:container"
  type: string        // "container"
  label: string       // "Containers"
  count: number
  healthCounts: HealthCounts
  nodes: GraphNode[]
}

export interface GroupNodeData {
  groupType: string
  label: string
  count: number
  healthCounts: HealthCounts
  color: string
  icon: string
  isCritical: boolean  // >50% degraded/unhealthy → pulse dot
}

// ─── Constants ────────────────────────────────────────────────────────────────

/** Types that always render as individual nodes (logically unique / 1 instance) */
const SINGLETON_TYPES = new Set([
  'host',
  'cluster',
  'container_runtime',
  'image_group',
])

const TYPE_LABELS: Record<string, string> = {
  host: 'Host',
  container_runtime: 'Docker Runtime',
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
): {
  nodes: Node[]
  edges: Edge[]
  groups: Map<string, GroupInfo>
} {
  // 1. Filter to visible types
  const filtered = rawNodes.filter((n) => visibleTypes.has(n.type))
  const filteredIdSet = new Set(filtered.map((n) => n.id))

  // 2. Bucket by type
  const byType = new Map<string, GraphNode[]>()
  for (const n of filtered) {
    const bucket = byType.get(n.type) ?? []
    bucket.push(n)
    byType.set(n.type, bucket)
  }

  // 3. Decide: individual vs group
  const groups = new Map<string, GroupInfo>()
  const individualIds = new Set<string>()

  for (const [type, nodes] of byType) {
    const gid = `group:${type}`
    const singleton = SINGLETON_TYPES.has(type) || nodes.length === 1
    const expanded = expandedGroups.has(gid)

    if (singleton || expanded) {
      for (const n of nodes) individualIds.add(n.id)
    } else {
      const hc = countHealth(nodes)
      groups.set(type, {
        id: gid,
        type,
        label: getGroupLabel(type),
        count: nodes.length,
        healthCounts: hc,
        nodes,
      })
    }
  }

  // 4. Build React Flow node array
  const flowNodes: Node[] = []

  // Individual nodes
  for (const n of filtered) {
    if (!individualIds.has(n.id)) continue
    flowNodes.push({
      id: n.id,
      type: n.type === 'namespace' ? 'namespaceGroup' : 'infraNode',
      position: { x: 0, y: 0 },
      data: {
        nodeType: n.type,
        label: n.label,
        health: normalizeNodeHealth(n),
        metadata: n.metadata,
      } as InfraNodeData,
      width: 220,
      height: 100,
    })
  }

  // Group nodes
  for (const [type, g] of groups) {
    flowNodes.push({
      id: g.id,
      type: 'groupNode',
      position: { x: 0, y: 0 },
      data: {
        groupType: type,
        label: g.label,
        count: g.count,
        healthCounts: g.healthCounts,
        color: getNodeColor(type),
        icon: getNodeIcon(type),
        isCritical: isCriticalGroup(g.healthCounts, g.count),
      } as GroupNodeData,
      width: GROUP_NODE_WIDTH,
      height: GROUP_NODE_HEIGHT,
    })
  }

  // 5. Build canvas ID map (raw nodeId → canvas nodeId)
  const toCanvas = new Map<string, string>()
  for (const id of individualIds) toCanvas.set(id, id)
  for (const [type, g] of groups) {
    for (const n of g.nodes) toCanvas.set(n.id, g.id)
  }

  const canvasIds = new Set(flowNodes.map((n) => n.id))

  // 6. Build bundled edges
  // Key: "src→tgt→edgeType"  → accumulate count
  const bundle = new Map<
    string,
    { source: string; target: string; edgeType: string; count: number }
  >()

  for (const e of rawEdges) {
    // Skip if EITHER endpoint is not visible
    if (!filteredIdSet.has(e.source) || !filteredIdSet.has(e.target)) continue
    const src = toCanvas.get(e.source)
    const tgt = toCanvas.get(e.target)
    if (!src || !tgt || src === tgt) continue
    if (!canvasIds.has(src) || !canvasIds.has(tgt)) continue

    const key = `${src}→${tgt}→${e.type}`
    if (bundle.has(key)) {
      bundle.get(key)!.count++
    } else {
      bundle.set(key, { source: src, target: tgt, edgeType: e.type, count: 1 })
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
