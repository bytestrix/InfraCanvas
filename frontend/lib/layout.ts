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
import { GROUP_NODE_WIDTH, GROUP_NODE_HEIGHT } from './graphPreprocess'

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
  // Tier 1 — category-summary nodes from the host's "•••" picker
  // (groupType is the FilterKey itself, e.g. "k8s", not a raw resource type)
  k8s:               { tier: 1, zone: 'left'   },
  docker:            { tier: 1, zone: 'right'  },
  pods:              { tier: 1, zone: 'center' },
  services:          { tier: 1, zone: 'center' },
  storage:           { tier: 1, zone: 'center' },
  events:            { tier: 1, zone: 'center' },
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

// ─── Namespace-scoped hierarchy layout ────────────────────────────────────────

/**
 * Lays the grouped canvas out as a namespace hierarchy instead of a flat grid.
 *
 * The flat grid bucketed every resource type into its own row, so `Pods ×9`
 * sat on the same tier as `DaemonSets ×2` with nothing saying which namespace
 * either belonged to. Grouping keys are already namespace-scoped by
 * `buildGroupedGraph`, so all this has to do is honour that: each namespace
 * gets its own column with its scoped child groups stacked beneath it, and
 * every node that is *not* namespace-scoped keeps the existing zone/tier rows.
 *
 * Each namespace column is sized to the widest child row it has to hold, so
 * two namespaces with different numbers of resource types cannot overlap.
 */
const NS_COL_X_START = 200
const NS_COL_GAP = 120
// The namespace hierarchy lays its rows out itself rather than via the zone
// tier table, so it needs its own row metrics. Values match the grouped
// canvas: node height plus a gap.
const ROW_H = 150
const ROW_START_Y = 60
const NS_COL_MIN_W = 560        // one group node plus breathing room
const NS_HEADER_H = 130         // vertical space reserved above a child row

export function applyNamespaceHierarchyLayout(
  nodes: Node[],
  edges: Edge[],
): { nodes: Node[]; edges: Edge[] } {
  const nsNodes = nodes.filter((n) => n.data?.nodeType === 'namespace')
  const nsIdSet = new Set(nsNodes.map((n) => n.id))
  const nsNames = new Set(nsNodes.map((n) => String(n.data?.label ?? '')))

  // Partition every node into either "belongs under a namespace" or "free".
  //
  // Both scoped group nodes AND individually-rendered namespaced nodes belong
  // under their namespace. The second case matters: a namespace holding a
  // single pod renders that pod by name instead of a redundant `Pods ×1`
  // box, and without nesting it here that node lands in the far corner of the
  // grid with nothing connecting it to the namespace that owns it.
  const scoped = new Map<string, Node[]>()
  const rest: Node[] = []
  for (const n of nodes) {
    if (nsIdSet.has(n.id)) continue

    const d = n.data as {
      namespace?: string
      metadata?: Record<string, unknown>
      nodeType?: string
      groupType?: string
    }
    const fromGroup = typeof d?.namespace === 'string' ? d.namespace : ''
    const fromMeta = typeof d?.metadata?.namespace === 'string' ? String(d.metadata.namespace) : ''
    const ns = fromGroup || fromMeta

    if (ns && nsNames.has(ns)) {
      scoped.set(ns, [...(scoped.get(ns) ?? []), n])
    } else {
      rest.push(n)
    }
  }

  // No namespaces on the canvas → nothing to nest under, keep the flat grid.
  if (nsNodes.length === 0) {
    return { nodes: applyGroupedZoneLayout(nodes), edges }
  }

  // Namespace columns are laid out first and the grid is pushed below them.
  // The grid keeps its own zone/tier maths untouched; only its vertical
  // offset is computed, from the real height of the hierarchy above, so the
  // two can never overlap no matter how many namespaces or resource types a
  // cluster has.
  const out: Node[] = []
  let hierarchyBottom = ROW_START_Y

  // One column per namespace, in a stable order so the layout does not
  // reshuffle between refreshes.
  const nsOrder = nsNodes
    .map((ns) => ({ ns: ns, name: String(ns.data?.label ?? '') }))
    .sort((a, b) => a.name.localeCompare(b.name))

  let cursorX = NS_COL_X_START
  for (const { ns, name } of nsOrder) {
    const children = scoped.get(name) ?? []

    // Group children by resource type so `Pods ×8` and
    // `DaemonSets ×2` stack rather than interleave. Individually-rendered
    // namespaced nodes group by their own type for the same reason.
    const byType = new Map<string, Node[]>()
    for (const c of children) {
      const cd = c.data as { groupType?: string; nodeType?: string }
      const t = cd?.groupType ?? cd?.nodeType ?? c.type ?? ''
      byType.set(t, [...(byType.get(t) ?? []), c])
    }

    // Widest row decides the column width, so columns never overlap.
    let widest = NS_COL_MIN_W
    for (const [, row] of byType) {
      widest = Math.max(widest, rowW(row))
    }

    const colX = cursorX
    const colCX = colX + widest / 2
    const nsNode = nsNodes.find((x) => x.id === ns.id)!
    let colBottom = ROW_START_Y + (nsNode.height ?? NS_HEADER_H)
    // Centre the header over its children rather than pinning it to the
    // column's left edge — otherwise the containment edges fan out sideways
    // and the hierarchy reads as a diagonal.
    out.push({
      ...nsNode,
      position: { x: colCX - (nsNode.width ?? NODE_WIDTH) / 2, y: ROW_START_Y },
    })

    // Children in rows beneath the namespace header. Types are ordered by
    // name so the column does not reshuffle between refreshes.
    const orderedTypes = [...byType.keys()].sort()
    let childY = ROW_START_Y + NS_HEADER_H
    for (const type of orderedTypes) {
      const row = byType.get(type)!
      let cursor = colCX - rowW(row) / 2
      for (const child of row) {
        const w = child.width ?? GROUP_NODE_WIDTH
        out.push({ ...child, position: { x: cursor, y: childY } })
        cursor += w + ZONE_CELL_W
      }
      // Tallest node in this row decides how far the column reaches down.
      const rowH = Math.max(...row.map((c) => c.height ?? GROUP_NODE_HEIGHT))
      colBottom = Math.max(colBottom, childY + rowH)
      childY += ROW_H
    }

    hierarchyBottom = Math.max(hierarchyBottom, colBottom)

    cursorX += widest + NS_COL_GAP
  }

  // Any scoped group whose namespace node was filtered out (or not present in
  // this snapshot) would otherwise vanish entirely. Park those on a trailing
  // row so they stay visible and drillable rather than silently disappearing.
  const placed = new Set(out.map((n) => n.id))
  const orphans = orphanScopedGroups(scoped, placed)
  let oy = hierarchyBottom + ROW_H
  for (const orphan of orphans) {
    out.push({ ...orphan, position: { x: NS_COL_X_START, y: oy } })
    oy += ROW_H
  }
  hierarchyBottom = oy

  // Everything not owned by a namespace keeps the existing zone/tier grid,
  // offset below the finished hierarchy.
  const gridOffsetY = hierarchyBottom + ROW_H - ROW_START_Y
  if (rest.length > 0) {
    for (const n of applyGroupedZoneLayout(rest)) {
      out.push({ ...n, position: { x: n.position.x, y: n.position.y + gridOffsetY } })
    }
  }

  return { nodes: out, edges }
}

/** Total width of one child row, gaps included. */
function rowW(row: Node[]): number {
  let w = 0
  row.forEach((n, i) => { w += (n.width ?? GROUP_NODE_WIDTH) + (i > 0 ? ZONE_CELL_W : 0) })
  return w
}

/** Scoped groups whose namespace has no node on the canvas. */
function orphanScopedGroups(
  scoped: Map<string, Node[]>,
  placed: Set<string>,
): Node[] {
  const out: Node[] = []
  for (const [, list] of scoped) {
    for (const n of list) if (!placed.has(n.id)) out.push(n)
  }
  return out
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
