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
export const ORB_CELL_H = 38
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
      data: {
        label: p,
        count: total,
        color: p === 'Kubernetes' ? '#eab308' : p === 'Docker' ? '#2496ED' : '#64748b',
        // Which engine mark the root disc carries. Kubernetes keeps its own
        // helm (drawn in MapRoot); Docker and Podman reuse NodeSvgIcon's marks
        // so the centre of each platform's map shows the actual engine, not
        // the word for it.
        kind: p === 'Docker' ? 'container_runtime' : p === 'Podman' ? 'podman' : p === 'Host' ? 'host' : '',
      },
      width: ROOT_D + 16, height: ROOT_D + 30,
    } as Node)
  }

  // One hub per type. Biggest types first so they get the roomiest arc.
  const hubs: Hub[] = [...byType.keys()]
    // Sorted by namespace first, then name. Every deployment in a hub belongs
    // to some namespace, and grouping those runs together is what turns a
    // hub's cluster from "30 unrelated dots" into "these are `cs`, these are
    // `ingress-nginx`" — which is also what makes the link back to the
    // namespace node short enough to read.
    .map((t) => ({ type: t, items: (byType.get(t) ?? []).slice().sort(byNsThenLabel) }))
    .sort((a, b) => b.items.length - a.items.length)

  for (const h of hubs) {
    const hid = `__hub__${h.type}`
    out.push({
      id: hid, type: 'mapHub',
      position: { x: 0, y: 0 },
      data: { label: typeLabel(h.type), kind: h.type, count: h.items.length, color: HUB_COLOR[h.type] ?? '#64748b', degree: h.items.length },
      width: HUB_D + 16, height: HUB_D + 28,
    } as Node)
    const platform = PLATFORM[h.type]
    if (platform) add(rootId(platform), hid)
    for (const it of h.items) add(hid, it.id)
  }

  // Connection count per node, so size can reflect connectivity: a namespace
  // with 40 pods under it should read as more important than a namespace with
  // one, without anyone having to hover to find out.
  // The `hm:` tree edges we just added are structural, not real connections,
  // so a node's displayed degree comes from the snapshot's own edges only.
  const realDegree = new Map<string, number>()
  for (const n of nodes) realDegree.set(n.id, 0)
  for (const e of edges) {
    if (e.id?.startsWith('hm:')) continue
    if (realDegree.has(e.source)) realDegree.set(e.source, (realDegree.get(e.source) ?? 0) + 1)
    if (realDegree.has(e.target)) realDegree.set(e.target, (realDegree.get(e.target) ?? 0) + 1)
  }

  out.push(
    ...nodes.map((n) => ({
      ...n,
      data: { ...(n.data as Meta), degree: realDegree.get(n.id) ?? 0 },
    })),
  )

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

function nsNameOf(n: Node): string {
  const m = n.data as { metadata?: Record<string, unknown> }
  const ns = m?.metadata?.namespace
  return typeof ns === 'string' ? ns : '~'
}

function byNsThenLabel(a: Node, b: Node): number {
  const na = nsNameOf(a)
  const nb = nsNameOf(b)
  if (na !== nb) return na < nb ? -1 : 1
  return byLabel(a, b)
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

  // ── Hubs: ring around their platform root, each hub a wedge sized to its
  //    cluster; then the roots themselves share a super-ring ─────────────────
  //
  // Equal spacing is not enough. A hub owning 55 pods has a cluster ~330px in
  // radius; a hub owning 1 cluster node is ~30px. Sharing the circle evenly
  // puts the big cluster straight over the root and over its neighbours. So
  // each hub claims a wedge, and the ring is grown until every wedge fits.
  //
  // This runs once PER PLATFORM. A machine canvas has Kubernetes, Docker and
  // Host roots at once, and stacking all three roots at the origin interleaved
  // their hub clusters into one knot. Each platform lays its hubs out in
  // private root-at-the-origin coordinates, then every root takes a slot on
  // the super-ring and its constellation moves with it. A single-platform
  // canvas has one root pinned at the origin — identical to the old output.

  const byLabelStrict = (a: string, b: string) => {
    const la = String((byId.get(a)?.data as Meta)?.label ?? '')
    const lb = String((byId.get(b)?.data as Meta)?.label ?? '')
    return la < lb ? -1 : la > lb ? 1 : 0
  }

  // Group hubs under their root; hubs with no root at all form one extra
  // platform-less constellation so they still lay out cleanly.
  const claimed = new Set<string>()
  const groups: { root: string | null; hubs: string[] }[] = []
  for (const rid of rootIds) {
    const hubs = (kids.get(rid) ?? []).filter((h) => hubIds.includes(h) && byId.has(h))
    for (const h of hubs) claimed.add(h)
    groups.push({ root: rid, hubs })
  }
  const unclaimed = hubIds.filter((h) => !claimed.has(h))
  if (unclaimed.length) groups.push({ root: null, hubs: unclaimed })

  // Per platform: hubs at wedge positions in local (root-at-origin) coords;
  // the hub's orbs move with it. Members recorded so the platform shifts as
  // one block once the root has its super-ring position.
  const members = new Map<string | null, Set<string>>()
  const platformRadius = new Map<string | null, number>()
  for (const { root, hubs } of groups) {
    const set = new Set<string>(hubs)
    members.set(root, set)
    for (const hid of hubs) {
      for (const c of kids.get(hid) ?? []) if (byId.has(c)) set.add(c)
    }
    if (hubs.length === 0) { platformRadius.set(root, 200); continue }

    const orderedL = hubs.slice().sort(byLabelStrict)
    const radiiL = orderedL.map((h) => Math.max(90, (hubRadius.get(h) ?? 0) + 60))
    let R = 300
    const fitsL = (r: number) =>
      radiiL.reduce((a, x) => a + Math.asin(Math.min(0.97, x / r)), 0) <= Math.PI * 0.88
    let guardL = 0
    while (!fitsL(R) && guardL++ < 60) R *= 1.12
    // The slot must hold the whole constellation, not just the hub ring: an
    // orb sits at hub-distance plus its ring, so the true extent is the max
    // hub distance to its outermost ring, not R alone.
    let extent = R
    for (const hid of orderedL) {
      const lp = pos.get(hid)
      if (!lp) continue
      const dist = Math.hypot(lp.x, lp.y)
      extent = Math.max(extent, dist + (hubRadius.get(hid) ?? 0))
    }
    platformRadius.set(root, extent)

    let cursorL = -Math.PI / 2
    orderedL.forEach((hid, i) => {
      const half = Math.asin(Math.min(0.97, radiiL[i] / R))
      const a = cursorL + half           // centre this hub inside its own wedge
      cursorL += half * 2
      const cxL = Math.cos(a) * R
      const cyL = Math.sin(a) * R
      pos.set(hid, { x: cxL, y: cyL })
      const kidsSet = new Set(kids.get(hid) ?? [])
      for (const [id, p] of pos) {
        if (kidsSet.has(id)) pos.set(id, { x: p.x + cxL, y: p.y + cyL })
      }
    })
  }

  // ── Super-ring: one slot per platform, radius big enough for its cluster ──
  const slots: { key: string; radius: number }[] = []
  // +150, not +extent-padding: the extent measures to an orb's ring centre,
  // but an orb's cell sticks out half its width (~56px) past that. Leaving
  // 90 shows up as a few-pixel sliver overlap between neighbouring roots.
  for (const rid of rootIds) slots.push({ key: rid, radius: (platformRadius.get(rid) ?? 300) + 150 })
  if (members.has(null) && (members.get(null)?.size ?? 0) > 0) {
    slots.push({ key: '__virtual__', radius: (platformRadius.get(null) ?? 300) + 150 })
  }
  slots.sort((a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0))

  let RS = 300
  const fitsS = (r: number) =>
    slots.reduce((a, x) => a + Math.asin(Math.min(0.97, x.radius / r)), 0) <= Math.PI * 0.88
  let guardS = 0
  while (!fitsS(RS) && guardS++ < 60) RS *= 1.12

  const slotPos = new Map<string, { x: number; y: number }>()
  let cursorS = -Math.PI / 2
  slots.forEach((s, i) => {
    const half = Math.asin(Math.min(0.97, s.radius / RS))
    const a = cursorS + half
    cursorS += half * 2
    slotPos.set(s.key, { x: Math.cos(a) * RS, y: Math.sin(a) * RS })
  })
  // A single platform keeps its root exactly at the origin — the canvas reads
  // as one map, not one of several.
  if (slots.length === 1) slotPos.set(rootIds[0] ?? slots[0].key, { x: 0, y: 0 })

  // Move every constellation onto its slot.
  for (const { root } of groups) {
    const off = slotPos.get(root ?? '__virtual__')
    if (!off) continue
    const movable = members.get(root)
    if (!movable) continue
    for (const id of movable) {
      const p = pos.get(id)
      if (p) pos.set(id, { x: p.x + off.x, y: p.y + off.y })
    }
  }
  for (const rid of rootIds) pos.set(rid, slotPos.get(rid) ?? { x: 0, y: 0 })

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
