/**
 * hierarchyMap.ts
 *
 * A real containment hierarchy for the map view:
 *
 *   Kubernetes                     (big node, logo)
 *   ├── Pods            ×43
 *   ├── Namespaces      ×9         ── cs-agent  ── its pods
 *   │                                kube-system ── its pods
 *   ├── Deployments     ×30
 *   └── Services        ×44
 *
 * The previous attempts each got one half of this wrong. Grouping by type
 * alone threw away parentage — one flat `Pods ×9` node owned by nothing. A
 * free radial layout threw away grouping — 189 identical discs. Bucketing
 * into labelled blocks kept the grouping but had no hierarchy *inside* it, so
 * a namespace's resources looked like a flat shelf rather than something
 * belonging to it.
 *
 * So: platform roots at the top, resource-type nodes beneath them, individual
 * entities beneath those, and a namespace's own members beneath the namespace.
 * Containment is expressed by position *and* an edge, so the structure is
 * readable at a glance and still navigable by following lines.
 *
 * Deterministic — snapshots arrive every 15s and the layout must not reshuffle.
 */

import { Node, Edge } from 'reactflow'
import { MAP_NODE_WIDTH, MAP_NODE_HEIGHT } from './radialMap'

// ── Sizes ─────────────────────────────────────────────────────────────────────
const ROOT_W = 210
const ROOT_H = 62
const TYPE_W = 168
const TYPE_H = 42

const ENT_W = MAP_NODE_WIDTH
const ENT_H = MAP_NODE_HEIGHT

const GAP_X = 18
const GAP_Y = 34
const LEVEL_H = 62          // vertical gap between levels, on top of node height

/** Types that live inside a namespace and are therefore drawn under it. */
const NAMESPACE_SCOPED = new Set([
  'pod', 'deployment', 'statefulset', 'daemonset', 'job', 'cronjob',
  'k8s_service', 'ingress', 'pvc',
])

/** Human label for a raw type. */
const TYPE_LABEL: Record<string, string> = {
  pod: 'Pods', deployment: 'Deployments', statefulset: 'StatefulSets',
  daemonset: 'DaemonSets', job: 'Jobs', cronjob: 'CronJobs',
  k8s_service: 'Services', ingress: 'Ingresses', pvc: 'PVCs',
  namespace: 'Namespaces', node: 'Nodes', cluster: 'Cluster',
  container: 'Containers', image: 'Images', image_group: 'Build Cache',
  volume: 'Volumes', network: 'Networks', container_runtime: 'Container Runtime',
  event: 'Events', service: 'Services', process: 'Processes', host: 'Host',
  configmap: 'ConfigMaps', secret: 'Secrets',
  storageclass: 'Storage Classes', persistentvolume: 'Persistent Volumes',
  ingressclass: 'Ingress Classes', node_role: 'Node Roles',
}

/** Which platform a type belongs to, for the root row. */
const PLATFORM: Record<string, string> = (() => {
  const m: Record<string, string> = {}
  const put = (p: string, types: string[]) => types.forEach((t) => { m[t] = p })
  put('Kubernetes', ['cluster', 'node', 'namespace', 'pod', 'deployment', 'statefulset',
    'daemonset', 'job', 'cronjob', 'k8s_service', 'ingress', 'pvc', 'configmap',
    'secret', 'storageclass', 'persistentvolume', 'ingressclass', 'node_role'])
  put('Docker', ['container', 'image', 'image_group', 'volume', 'network', 'container_runtime'])
  put('Host', ['host', 'process', 'service'])
  return m
})()

export const HIER_ROOT_W = ROOT_W
export const HIER_ROOT_H = ROOT_H
export const HIER_TYPE_W = TYPE_W
export const HIER_TYPE_H = TYPE_H

type Meta = Record<string, any>

function nsOf(data: Meta): string {
  const ns = data?.metadata?.namespace
  return typeof ns === 'string' ? ns : ''
}

/**
 * Build the hierarchy nodes and the containment edges between them.
 * Returns React Flow nodes plus synthetic edges joining parent → child.
 */
export function buildHierarchy(
  nodes: Node[],
  edges: Edge[],
): { nodes: Node[]; edges: Edge[] } {
  const outNodes: Node[] = []
  const outEdges: Edge[] = []
  const tree = new Map<string, string[]>()   // parent id → child ids
  const add = (parent: string, child: string) => {
    tree.set(parent, [...(tree.get(parent) ?? []), child])
  }

  const byType = new Map<string, Node[]>()
  for (const n of nodes) {
    const t = String((n.data as Meta)?.nodeType ?? n.type ?? '')
    byType.set(t, [...(byType.get(t) ?? []), n])
  }

  // 1. Platform roots, in a stable order.
  const platforms = [...new Set([...byType.keys()].map((t) => PLATFORM[t]).filter(Boolean))]
  const rootId = (p: string) => `__root__${p}`
  for (const p of platforms) {
    const count = [...byType.keys()].filter((t) => PLATFORM[t] === p)
      .reduce((a, t) => a + (byType.get(t)?.length ?? 0), 0)
    outNodes.push({
      id: rootId(p), type: 'mapRoot',
      position: { x: 0, y: 0 },
      data: { label: p, count },
      width: ROOT_W, height: ROOT_H,
      draggable: true,
    } as Node)
  }

  // 2. Type nodes under each platform. Namespaces get their entities attached
  //    as real nodes; every other type's entities attach directly.
  for (const t of [...byType.keys()].sort()) {
    const list = byType.get(t)!
    const platform = PLATFORM[t]
    const typeId = `__type__${t}`

    if (!outNodes.some((n) => n.id === typeId)) {
      outNodes.push({
        id: typeId, type: 'mapType',
        position: { x: 0, y: 0 },
        data: { label: TYPE_LABEL[t] ?? t, kind: t, count: list.length },
        width: TYPE_W, height: TYPE_H,
      } as Node)
      if (platform) add(rootId(platform), typeId)
    }

    if (t === 'namespace') {
      // Namespace entities become their own visible nodes; their members hang
      // beneath them.
      for (const ns of list.slice().sort(byLabel)) {
        add(typeId, ns.id)
      }
      continue
    }

    if (NAMESPACE_SCOPED.has(t)) {
      // Drawn under their namespace instead, so the parentage is real.
      // Still linked to the type node so "Pods" reaches its pods.
      continue
    }

    for (const n of list.slice().sort(byLabel)) add(typeId, n.id)
  }

  // 3. Namespaced resources under their namespace.
  const nsByName = new Map<string, Node>()
  for (const n of byType.get('namespace') ?? []) nsByName.set(nsOf(n.data as Meta) || String((n.data as Meta)?.label ?? ''), n)
  // A namespace node may carry the name in label rather than metadata.
  for (const n of byType.get('namespace') ?? []) {
    const nm = String((n.data as Meta)?.label ?? '')
    if (nm && !nsByName.has(nm)) nsByName.set(nm, n)
  }

  for (const t of [...byType.keys()].sort()) {
    if (!NAMESPACE_SCOPED.has(t)) continue
    const typeId = `__type__${t}`
    for (const n of (byType.get(t) ?? []).slice().sort(byLabel)) {
      const nsName = nsOf(n.data as Meta)
      const parent = nsByName.get(nsName)
      if (parent) {
        add(parent.id, n.id)
        // Also reach it from the type node, so "Pods" connects to its pods.
        outEdges.push({ id: `he:${typeId}→${n.id}`, source: typeId, target: n.id, kind: 'type-link' } as Edge)
      } else {
        // Orphan (its namespace is filtered out) — keep it under its type.
        add(typeId, n.id)
      }
    }
  }

  // 4. All the real nodes.
  outNodes.push(...nodes)

  // 5. Containment edges.
  for (const [parent, children] of tree) {
    for (const child of children) {
      outEdges.push({ id: `hc:${parent}→${child}`, source: parent, target: child, kind: 'contain' } as Edge)
    }
  }

  return { nodes: outNodes, edges: outEdges }
}

function byLabel(a: Node, b: Node): number {
  const la = String((a.data as Meta)?.label ?? a.id)
  const lb = String((b.data as Meta)?.label ?? b.id)
  return la < lb ? -1 : la > lb ? 1 : 0
}

// ── Layout ────────────────────────────────────────────────────────────────────

interface Box { w: number; h: number; x: number; y: number; node: Node }

/**
 * Lay the hierarchy out top-down, centring each node over its children.
 *
 * Wide fan-outs (a namespace with 40 pods) wrap into a grid instead of one
 * absurd row — the alternative is a tree tens of thousands of pixels wide,
 * which fit-view answers by zooming to unreadability.
 */
export function applyHierarchyLayout(
  nodes: Node[],
  edges: Edge[],
): { nodes: Node[]; edges: Edge[] } {
  if (nodes.length === 0) return { nodes, edges }

  const byId = new Map(nodes.map((n) => [n.id, n]))
  const children = new Map<string, string[]>()
  const hasParent = new Set<string>()
  for (const e of edges) {
    if ((e as { kind?: string }).kind !== 'contain') continue
    if (!byId.has(e.source) || !byId.has(e.target)) continue
    children.set(e.source, [...(children.get(e.source) ?? []), e.target])
    hasParent.add(e.target)
  }
  const roots = nodes.filter((n) => !hasParent.has(n.id)).map((n) => n.id)

  const placed: Box[] = []
  // Wider blocks mean fewer levels stacked on top of each other; capped so a
  // 50-pod namespace stays a readable rectangle rather than a 9000px ribbon.
  const MAX_COLS = 8
  /** Roughly how wide a level of the tree should be. */
  const TARGET_BLOCK_W = 1500
  const CELL_W = Math.max(ENT_W, TYPE_W) + GAP_X
  const CELL_H = ENT_H + GAP_Y

  // Depth-first, bottom-up: each subtree reports its size, parent is centred
  // above it.
  const measure = (id: string): { w: number; h: number; place: (x: number, y: number) => void } => {
    const node = byId.get(id)!
    const isSynthetic = id.startsWith('__')
    const selfW = node.width ?? ENT_W
    const selfH = node.height ?? ENT_H
    const kids = (children.get(id) ?? []).filter((c) => byId.has(c))
    // Sorted so ordering is stable between refreshes.
    kids.sort((a, b) => byLabel(byId.get(a)!, byId.get(b)!))

    if (kids.length === 0) {
      return { w: selfW, h: selfH, place: (x, y) => placed.push({ x, y, w: selfW, h: selfH, node }) }
    }

    const sub = kids.map(measure)
    // Choose the column count from how wide the children actually are, not just
    // how many there are. A fixed count put four ~1200px namespace blocks side
    // by side and made the whole tree 6000px wide, which fit-view answers by
    // zooming to unreadability. Sizing to a target block width keeps the tree
    // near the shape of the viewport.
    const avgW = sub.reduce((a, s) => a + s.w, 0) / sub.length
    const cols = Math.max(1, Math.min(kids.length, MAX_COLS, Math.round(TARGET_BLOCK_W / (avgW + GAP_X))))
    const rows = Math.ceil(kids.length / cols)

    const colW: number[] = new Array(cols).fill(0)
    const rowH: number[] = new Array(rows).fill(0)
    sub.forEach((s, i) => {
      const c = i % cols
      const r = Math.floor(i / cols)
      colW[c] = Math.max(colW[c], s.w)
      rowH[r] = Math.max(rowH[r], s.h)
    })
    const gridW = colW.reduce((a, b) => a + b, 0) + GAP_X * (cols - 1)
    const gridH = rowH.reduce((a, b) => a + b, 0) + GAP_Y * (rows - 1)

    const w = Math.max(selfW, gridW)
    const h = selfH + LEVEL_H + gridH

    return {
      w, h,
      place: (x, y) => {
        placed.push({ x: x + (w - selfW) / 2, y, w: selfW, h: selfH, node })
        const gx = x + (w - gridW) / 2
        sub.forEach((s, i) => {
          const c = i % cols
          const r = Math.floor(i / cols)
          let cx = gx
          for (let k = 0; k < c; k++) cx += colW[k] + GAP_X
          // Offset by the height of every row above this one — without this
          // every row lands on the same y and stacks into one line.
          let ry = y + selfH + LEVEL_H
          for (let k = 0; k < r; k++) ry += rowH[k] + GAP_Y
          s.place(cx, ry + (rowH[r] - s.h) / 2)
        })
        void isSynthetic
      },
    }
  }

  const top = roots.map((id) => measure(id))

  // Roots in a row, spaced by their own widths.
  let cursor = 0
  top.forEach((t) => {
    t.place(cursor, 0)
    cursor += t.w + GAP_X * 3
  })

  // Normalise so the origin is at the top-left of content.
  const minX = Math.min(...placed.map((b) => b.x))
  const minY = Math.min(...placed.map((b) => b.y))

  const out = placed.map((b) => ({
    ...b.node,
    position: { x: Math.round(b.x - minX), y: Math.round(b.y - minY) },
  }))

  return { nodes: out, edges }
}
