/**
 * hubMap.ts
 *
 * Radial hub-and-spoke layout for the map view, per the model:
 *
 *              ┌─ Pod1 ─┐
 *              │  Pod2 ─┤── NS1 ─┐
 *         Pod3 ┘        │        ├── Namespace ─┐
 *         Pod4 ───────────── NS2 ┘              │
 *   Pods ──────────────────────────────────────── ├── K8s ── Deployment ── Deployment1..4
 *                                                  │
 *
 * One root in the middle, a **type hub** per resource type around it, and each
 * hub's individual entities on concentric rings around that hub.
 *
 * Earlier attempts each missed something. Grouping by type alone destroyed
 * parentage. A free radial layout put all 189 nodes on one circle, so nothing
 * was distinguishable. A top-down hierarchy was readable but was not what was
 * asked for — it read like a table, not a map. This is the centre-and-spokes
 * shape, with a hub per type so a pod is always near "Pods" and a deployment
 * always near "Deployments".
 *
 * Deterministic: snapshots arrive every 15s and the layout must not reshuffle.
 */

import { Node, Edge } from 'reactflow'

export const ORB_D = 26
export const ORB_CELL_W = 112
export const ORB_CELL_H = 30
export const HUB_D = 62
export const ROOT_D = 84
/** Radius of an orb's first ring around its hub — wide enough to hold 3 orbs. */
const RING_R0 = 150

/** Types that live in a namespace. Their hub says so, and namespaced items are
 *  still individually drawn under their type hub. */
const NS_SCOPED = new Set([
  'pod', 'deployment', 'statefulset', 'daemonset', 'job', 'cronjob',
  'k8s_service', 'ingress', 'pvc',
])

/** Platform membership for the root row. */
const PLATFORM: Record<string, string> = (() => {
  const m: Record<string, string> = {}
  const put = (p: string, t: string[]) => t.forEach((x) => { m[x] = p })
  put('Kubernetes', ['cluster', 'node', 'namespace', 'pod', 'deployment', 'statefulset',
    'daemonset', 'job', 'cronjob', 'k8s_service', 'ingress', 'pvc', 'configmap',
    'secret', 'storageclass', 'persistentvolume', 'ingressclass', 'node_role'])
  put('Docker', ['container', 'image', 'image_group', 'volume', 'network', 'container_runtime'])
  put('Host', ['host', 'process', 'service'])
  return m
})()

type Meta = Record<string, any>

/** Stable display label for a type. */
function typeLabel(t: string): string {
  const map: Record<string, string> = {
    pod: 'Pods', deployment: 'Deployments', statefulset: 'StatefulSets',
    daemonset: 'DaemonSets', job: 'Jobs', cronjob: 'CronJobs',
    k8s_service: 'Services', ingress: 'Ingresses', pvc: 'PVCs',
    namespace: 'Namespaces', node: 'Nodes', cluster: 'Cluster',
    container: 'Containers', image: 'Images', image_group: 'Build Cache',
    volume: 'Volumes', network: 'Networks', container_runtime: 'Runtime',
    event: 'Events', service: 'Services', process: 'Processes', host: 'Host',
    configmap: 'ConfigMaps', secret: 'Secrets', storageclass: 'Storage Classes',
    persistentvolume: 'Persistent Volumes', ingressclass: 'Ingress Classes',
    node_role: 'Node Roles',
  }
  return map[t] ?? t
}

/** Accent colour per type, so hubs are told apart at a glance. */
const HUB_COLOR: Record<string, string> = {
  pod: '#ef4444', deployment: '#ef4444', statefulset: '#ef4444',
  namespace: '#a855f7', node: '#a855f7', cluster: '#a855f7',
  k8s_service: '#3b82f6', ingress: '#3b82f6', pvc: '#3b82f6',
  daemonset: '#f59e0b', job: '#f59e0b', cronjob: '#f59e0b',
  container: '#10b981', image: '#10b981', volume: '#10b981', network: '#10b981',
  event: '#64748b', service: '#64748b', process: '#64748b', host: '#64748b',
}

interface Hub {
  type: string
  items: Node[]
}

export interface HubMapResult {
  nodes: Node[]
  edges: Edge[]
}

/** Build root → type-hub → entity, plus the extra links that make the map
 *  navigable (entity → its namespace). */
export function buildHubMap(nodes: Node[], edges: Edge[]): HubMapResult {
  const byType = new Map<string, Node[]>()
  for (const n of nodes) {
    const t = String((n.data as Meta)?.nodeType ?? n.type ?? '')
    byType.set(t, [...(byType.get(t) ?? []), n])
  }

  const out: Node[] = []
  const outEdges: Edge[] = []
  const tree = new Map<string, string[]>()
  const add = (p: string, c: string) => tree.set(p, [...(tree.get(p) ?? []), c])

  // Roots, one per platform present.
  const platforms = [...new Set([...byType.keys()].map((t) => PLATFORM[t]).filter(Boolean))]
  const rootId = (p: string) => `__root__${p}`
  for (const p of platforms) {
    const total = [...byType.keys()]
      .filter((t) => PLATFORM[t] === p)
      .reduce((a, t) => a + (byType.get(t)?.length ?? 0), 0)
    out.push({
      id: rootId(p), type: 'mapRoot',
      position: { x: 0, y: 0 },
      data: { label: p, count: total, color: p === 'Kubernetes' ? '#eab308' : p === 'Docker' ? '#2496ED' : '#64748b' },
      width: ROOT_D + 16, height: ROOT_D + 30,
    } as Node)
  }

  // One hub per type. Biggest types first so they get the roomiest arc.
  const hubs: Hub[] = [...byType.keys()]
    .map((t) => ({ type: t, items: (byType.get(t) ?? []).slice().sort(byLabel) }))
    .sort((a, b) => b.items.length - a.items.length)

  for (const h of hubs) {
    const hid = `__hub__${h.type}`
    out.push({
      id: hid, type: 'mapHub',
      position: { x: 0, y: 0 },
      data: { label: typeLabel(h.type), kind: h.type, count: h.items.length, color: HUB_COLOR[h.type] ?? '#64748b' },
      width: HUB_D + 16, height: HUB_D + 28,
    } as Node)
    const platform = PLATFORM[h.type]
    if (platform) add(rootId(platform), hid)
    for (const it of h.items) add(hid, it.id)
  }

  out.push(...nodes)

  for (const [p, kids] of tree) {
    for (const k of kids) outEdges.push({ id: `hm:${p}→${k}`, source: p, target: k, kind: 'contain' } as Edge)
  }

  // Entities → their namespace, so a namespace is reachable from its members.
  const nsByName = new Map<string, string>()
  for (const n of byType.get('namespace') ?? []) {
    const nm = String((n.data as Meta)?.label ?? '')
    if (nm) nsByName.set(nm, n.id)
  }
  for (const [t, list] of byType) {
    if (!NS_SCOPED.has(t)) continue
    for (const n of list) {
      const nsName = String((n.data as Meta)?.metadata?.namespace ?? '')
      const nsId = nsByName.get(nsName)
      if (!nsId || nsId === n.id) continue
      outEdges.push({ id: `ns:${n.id}→${nsId}`, source: n.id, target: nsId, kind: 'ns-link' } as Edge)
    }
  }

  return { nodes: out, edges: [...outEdges, ...edges] }
}

function byLabel(a: Node, b: Node): number {
  const la = String((a.data as Meta)?.label ?? a.id)
  const lb = String((b.data as Meta)?.label ?? b.id)
  return la < lb ? -1 : la > lb ? 1 : 0
}

/**
 * Place the root at the centre, hubs on a ring around it, and each hub's items
 * on concentric rings around that hub.
 *
 * Items go on *concentric* rings rather than one ring because a hub can own 44
 * pods: a single ring big enough for 44 cells would push that hub's whole
 * cluster thousands of pixels out and collide with its neighbours.
 */
export function applyHubRadialLayout(
  nodes: Node[],
  edges: Edge[],
): { nodes: Node[]; edges: Edge[] } {
  if (nodes.length === 0) return { nodes, edges }

  const byId = new Map(nodes.map((n) => [n.id, n]))
  const kids = new Map<string, string[]>()
  const hubIds: string[] = []
  const rootIds: string[] = []

  for (const n of nodes) {
    if (n.id.startsWith('__root__')) rootIds.push(n.id)
    else if (n.id.startsWith('__hub__')) hubIds.push(n.id)
  }
  for (const e of edges) {
    if ((e as { kind?: string }).kind !== 'contain') continue
    if (!byId.has(e.source) || !byId.has(e.target)) continue
    kids.set(e.source, [...(kids.get(e.source) ?? []), e.target])
  }

  const pos = new Map<string, { x: number; y: number }>()
  const out: Node[] = []

  // ── Orbs: concentric rings around their hub ────────────────────────────────
  const hubRadius = new Map<string, number>()
  for (const hid of hubIds) {
    const list = (kids.get(hid) ?? []).filter((c) => byId.has(c)).sort((a, b) => byLabel(byId.get(a)!, byId.get(b)!))
    let i = 0
    let ring = 0
    // Ring capacity is whatever the circumference can actually hold. A floor on
    // capacity looked harmless but forced six 168px-wide orbs onto a 48px
    // ring, which put them all on top of each other.
    while (i < list.length) {
      // Rings are spaced further apart than an orb is wide. Orbs are laid out
      // radially, so a gap narrower than the label makes consecutive rings
      // overlap no matter how they are staggered.
      const r = RING_R0 + ring * (ORB_CELL_W + 40)
      const capacity = Math.max(3, Math.floor((2 * Math.PI * r) / (ORB_CELL_W + 10)))
      const slice = list.slice(i, i + capacity)
      slice.forEach((id, k) => {
        const a = (k / slice.length) * Math.PI * 2 - Math.PI / 2
        pos.set(id, { x: Math.cos(a) * r, y: Math.sin(a) * r })
      })
      i += capacity
      ring++
    }
    hubRadius.set(hid, ring === 0 ? 0 : RING_R0 + (ring - 1) * (ORB_CELL_W + 40) + ORB_CELL_W / 2)
  }

  // ── Hubs: ring around the root, each given an angular wedge big enough for
  //    its own cluster ───────────────────────────────────────────────────────
  //
  // Equal spacing is not enough. A hub owning 55 pods has a cluster ~330px in
  // radius; a hub owning 1 cluster node is ~30px. Sharing the circle evenly
  // puts the big cluster straight over the root and over its neighbours.
  //
  // So each hub claims a wedge, and the ring is grown until every wedge fits.
  const ordered = hubIds.slice().sort((a, b) => {
    const la = String((byId.get(a)!.data as Meta)?.label ?? '')
    const lb = String((byId.get(b)!.data as Meta)?.label ?? '')
    return la < lb ? -1 : la > lb ? 1 : 0
  })

  const radii = ordered.map((h) => Math.max(60, (hubRadius.get(h) ?? 0) + 40))
  let R = 300
  const fits = (r: number) =>
    radii.reduce((a, x) => a + Math.asin(Math.min(0.97, x / r)), 0) <= Math.PI * 0.88
  let guard = 0
  while (!fits(R) && guard++ < 60) R *= 1.12

  let cursor = -Math.PI / 2
  ordered.forEach((hid, i) => {
    const half = Math.asin(Math.min(0.97, radii[i] / R))
    const a = cursor + half          // centre this hub inside its own wedge
    cursor += half * 2
    const cx = Math.cos(a) * R
    const cy = Math.sin(a) * R
    pos.set(hid, { x: cx, y: cy })
    const set = new Set(kids.get(hid) ?? [])
    for (const [id, p] of pos) {
      if (set.has(id)) pos.set(id, { x: p.x + cx, y: p.y + cy })
    }
  })

  for (const rid of rootIds) pos.set(rid, { x: 0, y: 0 })
  // Any hub with no root above it still needs somewhere to sit.
  for (const hid of ordered) if (!pos.has(hid)) pos.set(hid, { x: 0, y: 0 })

  // ── Emit, centred on the origin ────────────────────────────────────────────
  const placed = nodes.filter((nd) => pos.has(nd.id))
  const placedSet = new Set(placed.map((p) => p.id))

  for (const nd of placed) {
    const p = pos.get(nd.id)!
    const w = nd.width ?? (nd.type === 'mapRoot' ? ROOT_D + 16 : nd.type === 'mapHub' ? HUB_D + 16 : ORB_CELL_W)
    const h = nd.height ?? (nd.type === 'mapRoot' ? ROOT_D + 30 : nd.type === 'mapHub' ? HUB_D + 28 : ORB_CELL_H)
    out.push({ ...nd, position: { x: Math.round(p.x - w / 2), y: Math.round(p.y - h / 2) } })
  }

  return { nodes: out, edges: edges.filter((e) => byId.has(e.source) && byId.has(e.target)) }
}
