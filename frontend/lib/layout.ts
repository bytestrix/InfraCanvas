/**
 * Layout utilities for React Flow nodes/edges.
 *
 * applyDagreLayout        — flat dagre layout (fallback / non-k8s views)
 * applyRootedDagreLayout  — rooted TB layout used by the grouped canvas:
 *                           • reverses RUNS_ON edges so "host" sits at the top
 *                           • injects a hidden __root__ if multiple unconnected
 *                             islands exist, so everything flows in one tree
 * applyNamespaceGroupLayout — groups k8s nodes inside their namespace boxes,
 *                             then runs a compact outer dagre for namespace+top-level nodes.
 */

import dagre from '@dagrejs/dagre'
import { Node, Edge } from 'reactflow'

export const NODE_WIDTH = 220
export const NODE_HEIGHT = 100

// Padding inside a namespace group container
const NS_PAD = { top: 44, right: 16, bottom: 16, left: 16 }
const NS_MIN_W = 260
const NS_MIN_H = 150

export interface LayoutOptions {
  rankdir?: 'TB' | 'LR' | 'BT' | 'RL'
  ranksep?: number
  nodesep?: number
  edgesep?: number
}

// ─── Rooted dagre (used by grouped canvas) ───────────────────────────────────
//
// Key insight: raw edges have RUNS_ON pointing child→parent (container→host),
// which makes dagre place containers ABOVE host. We reverse those for layout
// only, so "host" ends up at rank-0 (top) and everything flows downward.
// A hidden __root__ node stitches multiple disconnected islands together.

export function applyRootedDagreLayout(
  nodes: Node[],
  edges: Edge[],
  options: LayoutOptions = {}
): { nodes: Node[]; edges: Edge[] } {
  const {
    rankdir = 'TB',
    ranksep = 110,
    nodesep = 60,
    edgesep = 20,
  } = options

  const g = new dagre.graphlib.Graph({ multigraph: true })
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir, ranksep, nodesep, edgesep, marginx: 60, marginy: 60 })

  const nodeIdSet = new Set(nodes.map((n) => n.id))
  nodes.forEach((n) => {
    g.setNode(n.id, { width: n.width ?? NODE_WIDTH, height: n.height ?? NODE_HEIGHT })
  })

  // Track which nodes gain an incoming edge (for root detection)
  const hasIncoming = new Set<string>()

  edges.forEach((edge) => {
    if (!nodeIdSet.has(edge.source) || !nodeIdSet.has(edge.target)) return
    const label = typeof edge.label === 'string' ? edge.label : ''
    const isRunsOn = label.includes('RUNS_ON')

    if (isRunsOn) {
      // Reverse: target (e.g. host) becomes the dagre source → sits above
      g.setEdge(edge.target, edge.source, {}, `layout:${edge.id}`)
      hasIncoming.add(edge.source)
    } else {
      g.setEdge(edge.source, edge.target, {}, `layout:${edge.id}`)
      hasIncoming.add(edge.target)
    }
  })

  // Inject virtual root to connect disconnected islands
  const roots = nodes.filter((n) => !hasIncoming.has(n.id))
  if (roots.length > 1) {
    g.setNode('__root__', { width: 1, height: 1 })
    roots.forEach((r) => g.setEdge('__root__', r.id, {}, `__root__→${r.id}`))
  }

  dagre.layout(g)

  const positioned = nodes.map((n) => {
    const dp = g.node(n.id)
    if (!dp) return n
    return {
      ...n,
      position: {
        x: dp.x - (n.width ?? NODE_WIDTH) / 2,
        y: dp.y - (n.height ?? NODE_HEIGHT) / 2,
      },
    }
  })

  return { nodes: positioned, edges }
}

// ─── Flat dagre (used when no namespace nodes are visible) ────────────────────

export function applyDagreLayout(
  nodes: Node[],
  edges: Edge[],
  options: LayoutOptions = {}
): { nodes: Node[]; edges: Edge[] } {
  const {
    rankdir = 'TB',
    ranksep = 80,
    nodesep = 40,
    edgesep = 20,
  } = options

  const g = new dagre.graphlib.Graph({ multigraph: true })
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir, ranksep, nodesep, edgesep, marginx: 40, marginy: 40 })

  nodes.forEach((node) => {
    g.setNode(node.id, {
      width: node.width ?? NODE_WIDTH,
      height: node.height ?? NODE_HEIGHT,
    })
  })

  edges.forEach((edge) => {
    g.setEdge(edge.source, edge.target, {}, edge.id)
  })

  dagre.layout(g)

  const layoutedNodes = nodes.map((node) => {
    const { x, y, width, height } = g.node(node.id)
    return {
      ...node,
      position: {
        x: x - (width ?? NODE_WIDTH) / 2,
        y: y - (height ?? NODE_HEIGHT) / 2,
      },
    }
  })

  return { nodes: layoutedNodes, edges }
}

// ─── Grouped-mode zone layout ─────────────────────────────────────────────────
//
// Fixed zone layout for the grouped view.
// Each node type is assigned a fixed (tier, zone) slot:
//
//   Tier 0 center:  [ host ]
//   Tier 1 left:    [ cluster ]              Tier 1 right: [ container_runtime ]
//   Tier 2 left:    [ namespace ] [ node ]   Tier 2 right: [ image_group ] [ container ] [ image ] [ volume ] [ network ]
//   Tier 3 left:    [ deployment ] [ statefulset ] [ daemonset ] [ pod ] [ k8s_service ] [ ingress ]
//   Tier 4 left:    [ job ] [ cronjob ] [ pvc ] [ event ]
//
// No dagre needed — structure is always the same shape.

const ZONE_TIER_Y  = [60, 230, 410, 580, 750]  // y-coordinate per tier
const ZONE_CX: Record<'left' | 'right' | 'center', number> = {
  left:   800,
  center: 1250,
  right:  1700,
}
const ZONE_CELL_W = 270  // horizontal step between nodes in the same bucket

const TYPE_ZONE: Record<string, { tier: number; zone: 'left' | 'right' | 'center' }> = {
  // Tier 0
  host:              { tier: 0, zone: 'center' },
  // Tier 1
  cluster:           { tier: 1, zone: 'left'   },
  container_runtime: { tier: 1, zone: 'right'  },
  // Tier 2 left — K8s structural
  namespace:         { tier: 2, zone: 'left'   },
  node:              { tier: 2, zone: 'left'   },
  // Tier 2 right — Docker
  image_group:       { tier: 2, zone: 'right'  },
  container:         { tier: 2, zone: 'right'  },
  image:             { tier: 2, zone: 'right'  },
  volume:            { tier: 2, zone: 'right'  },
  network:           { tier: 2, zone: 'right'  },
  // Tier 3 left — K8s workloads
  deployment:        { tier: 3, zone: 'left'   },
  statefulset:       { tier: 3, zone: 'left'   },
  daemonset:         { tier: 3, zone: 'left'   },
  pod:               { tier: 3, zone: 'left'   },
  k8s_service:       { tier: 3, zone: 'left'   },
  ingress:           { tier: 3, zone: 'left'   },
  // Tier 4 left — K8s less common
  job:               { tier: 4, zone: 'left'   },
  cronjob:           { tier: 4, zone: 'left'   },
  pvc:               { tier: 4, zone: 'left'   },
  event:             { tier: 4, zone: 'left'   },
}

function getZoneType(node: Node): string {
  if (node.type === 'groupNode') return (node.data as { groupType?: string })?.groupType ?? ''
  return (node.data as { nodeType?: string })?.nodeType ?? ''
}

export function applyGroupedZoneLayout(nodes: Node[]): Node[] {
  // Bucket nodes by {tier}:{zone}
  const buckets = new Map<string, Node[]>()

  for (const node of nodes) {
    const type = getZoneType(node)
    const slot = TYPE_ZONE[type] ?? { tier: 5, zone: 'center' as const }
    const key = `${slot.tier}:${slot.zone}`
    if (!buckets.has(key)) buckets.set(key, [])
    buckets.get(key)!.push(node)
  }

  const result: Node[] = []

  for (const [key, bucket] of buckets) {
    const [tierStr, zone] = key.split(':') as [string, 'left' | 'right' | 'center']
    const tier = parseInt(tierStr, 10)
    const y    = ZONE_TIER_Y[tier] ?? ZONE_TIER_Y[ZONE_TIER_Y.length - 1] + (tier - ZONE_TIER_Y.length + 1) * 170
    const cx   = ZONE_CX[zone] ?? ZONE_CX.center
    const n    = bucket.length

    bucket.forEach((node, i) => {
      const nodeW = node.width ?? NODE_WIDTH
      // Center the spread around cx
      const x = cx + (i - (n - 1) / 2) * ZONE_CELL_W - nodeW / 2
      result.push({ ...node, position: { x, y } })
    })
  }

  return result
}

// ─── Namespace-grouped layout ─────────────────────────────────────────────────

/**
 * Groups nodes that have metadata.namespace under their namespace container node.
 * Each namespace is sized to contain its children, then an outer dagre arranges
 * namespace boxes + top-level nodes (cluster, k8s node, host, docker containers).
 *
 * Result: namespace boxes appear as compact, labelled areas instead of dozens of
 * individual nodes all spread across one very wide rank.
 */
export function applyNamespaceGroupLayout(
  nodes: Node[],
  edges: Edge[],
): { nodes: Node[]; edges: Edge[] } {
  // ── 1. Identify namespace nodes ───────────────────────────────────────────
  const nsNodes = nodes.filter((n) => n.data?.nodeType === 'namespace')

  // No namespaces visible → fall back to flat layout
  if (nsNodes.length === 0) {
    return applyDagreLayout(nodes, edges)
  }

  const nsIdByName = new Map<string, string>()  // namespace name → node id
  const nsIdSet = new Set(nsNodes.map((n) => n.id))
  for (const ns of nsNodes) {
    nsIdByName.set(String(ns.data?.label ?? ''), ns.id)
  }

  // ── 2. Sort nodes into namespace children vs top-level ────────────────────
  const childrenByNs = new Map<string, Node[]>()
  const topLevel: Node[] = []

  for (const node of nodes) {
    if (nsIdSet.has(node.id)) continue  // namespace node itself — handled separately

    const ns = String((node.data?.metadata as Record<string, unknown>)?.namespace ?? '')
    if (ns && nsIdByName.has(ns)) {
      const nsId = nsIdByName.get(ns)!
      const bucket = childrenByNs.get(nsId) ?? []
      bucket.push(node)
      childrenByNs.set(nsId, bucket)
    } else {
      topLevel.push(node)
    }
  }

  // ── 3. Layout children within each namespace (compact LR dagre) ───────────
  type InnerLayout = { relNodes: Node[]; w: number; h: number }
  const innerLayouts = new Map<string, InnerLayout>()

  for (const [nsId, children] of childrenByNs) {
    if (children.length === 0) continue

    const childSet = new Set(children.map((c) => c.id))
    const innerEdges = edges.filter(
      (e) => childSet.has(e.source) && childSet.has(e.target)
    )

    const ig = new dagre.graphlib.Graph({ multigraph: true })
    ig.setDefaultEdgeLabel(() => ({}))
    ig.setGraph({ rankdir: 'LR', ranksep: 40, nodesep: 16, marginx: 10, marginy: 10 })

    for (const c of children) {
      ig.setNode(c.id, { width: NODE_WIDTH, height: NODE_HEIGHT })
    }
    for (const e of innerEdges) {
      ig.setEdge(e.source, e.target, {}, e.id)
    }

    dagre.layout(ig)

    let maxX = 0
    let maxY = 0
    const relNodes = children.map((c) => {
      const dp = ig.node(c.id)
      const x = dp.x - NODE_WIDTH / 2
      const y = dp.y - NODE_HEIGHT / 2
      maxX = Math.max(maxX, x + NODE_WIDTH)
      maxY = Math.max(maxY, y + NODE_HEIGHT)
      // Position is relative — will be offset by NS_PAD after outer layout
      return { ...c, position: { x: x + NS_PAD.left, y: y + NS_PAD.top } }
    })

    innerLayouts.set(nsId, {
      relNodes,
      w: Math.max(maxX + NS_PAD.left + NS_PAD.right, NS_MIN_W),
      h: Math.max(maxY + NS_PAD.top + NS_PAD.bottom, NS_MIN_H),
    })
  }

  // ── 4. Size namespace nodes to contain their children ─────────────────────
  const sizedNsNodes = nsNodes.map((ns) => {
    const inner = innerLayouts.get(ns.id)
    const w = inner?.w ?? NS_MIN_W
    const h = inner?.h ?? NS_MIN_H
    return {
      ...ns,
      width: w,
      height: h,
      style: { ...(ns.style ?? {}), width: w, height: h },
    }
  })

  // ── 5. Outer dagre: namespace boxes + top-level nodes ─────────────────────
  const outerAll = [...topLevel, ...sizedNsNodes]
  const outerIdSet = new Set(outerAll.map((n) => n.id))

  // Only keep edges where both endpoints are outer-level (not inside a namespace)
  const outerEdges = edges.filter(
    (e) => outerIdSet.has(e.source) && outerIdSet.has(e.target)
  )

  const og = new dagre.graphlib.Graph({ multigraph: true })
  og.setDefaultEdgeLabel(() => ({}))
  og.setGraph({ rankdir: 'TB', ranksep: 60, nodesep: 30, marginx: 40, marginy: 40 })

  for (const n of outerAll) {
    og.setNode(n.id, {
      width: n.width ?? NODE_WIDTH,
      height: n.height ?? NODE_HEIGHT,
    })
  }
  for (const e of outerEdges) {
    og.setEdge(e.source, e.target, {}, e.id)
  }

  dagre.layout(og)

  const outerPositioned = outerAll.map((n) => {
    const dp = og.node(n.id)
    return {
      ...n,
      position: {
        x: dp.x - (n.width ?? NODE_WIDTH) / 2,
        y: dp.y - (n.height ?? NODE_HEIGHT) / 2,
      },
    }
  })

  // ── 6. Attach children to parent namespace nodes ──────────────────────────
  // Positions stay relative to namespace origin (set in step 3).
  // parentNode + extent:'parent' tells ReactFlow to render them inside the ns box.
  const finalChildren: Node[] = []
  for (const [nsId, inner] of innerLayouts) {
    for (const child of inner.relNodes) {
      finalChildren.push({
        ...child,
        parentNode: nsId,
        extent: 'parent' as const,
        zIndex: 1,
      })
    }
  }

  // Parent nodes MUST appear before their children in the array for ReactFlow v11
  return { nodes: [...outerPositioned, ...finalChildren], edges }
}
