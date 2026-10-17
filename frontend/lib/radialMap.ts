/**
 * radialMap.ts
 *
 * Radial "hub and spokes" layout for the node map.
 *
 * The grouped canvas is a nested tree: namespaces own groups, groups own
 * counts, and the inventory is only reachable by drilling in. The map view is
 * the opposite — every resource is its own node, and the shape of the graph is
 * the shape on screen.
 *
 * A force simulation (d3-force) is the obvious tool but is the wrong one here:
 * the graph re-renders every 15s as the agent pushes snapshots, and a force
 * layout that restarts from scratch each time makes the whole map jump. This
 * is a deterministic BFS-from-the-hub layout instead — same input, same
 * output, every refresh — so nodes stay put and only their state changes.
 */

import { Node, Edge } from 'reactflow'

export const MAP_NODE_WIDTH = 190
export const MAP_NODE_HEIGHT = 46

/**
 * Minimum distance between the centres of two nodes on the same ring.
 *
 * Must stay wider than MAP_NODE_WIDTH or nodes overlap: the map node is a disc
 * plus a label to its right, so its real footprint is the full card width, not
 * the 40px circle.
 */
const NODE_SLOT_W = MAP_NODE_WIDTH + 24
const NODE_SLOT_H = MAP_NODE_HEIGHT + 20
/** Gap between successive rings, before the "fit the crowd" adjustment. */
const RING_GAP = 190
/** Levels beyond this share the outermost ring — see applyRadialMapLayout. */
const MAX_RINGS = 5


interface Adj {
  neighbours: Map<string, Set<string>>
}

function buildAdjacency(nodes: Node[], edges: Edge[]): Adj {
  const ids = new Set(nodes.map((n) => n.id))
  const neighbours = new Map<string, Set<string>>()
  for (const id of ids) neighbours.set(id, new Set())
  for (const e of edges) {
    if (!ids.has(e.source) || !ids.has(e.target)) continue
    if (e.source === e.target) continue
    neighbours.get(e.source)!.add(e.target)
    neighbours.get(e.target)!.add(e.source)
  }
  return { neighbours }
}

/** Connected components, largest first. */
function components(adj: Adj): string[][] {
  const seen = new Set<string>()
  const comps: string[][] = []
  for (const start of adj.neighbours.keys()) {
    if (seen.has(start)) continue
    const comp: string[] = []
    const queue = [start]
    seen.add(start)
    while (queue.length) {
      const cur = queue.shift()!
      comp.push(cur)
      for (const nb of adj.neighbours.get(cur) ?? []) {
        if (seen.has(nb)) continue
        seen.add(nb)
        queue.push(nb)
      }
    }
    comps.push(comp)
  }
  return comps.sort((a, b) => b.length - a.length)
}

/**
 * Place every node on concentric rings around a hub.
 *
 * Ring membership is graph distance from the hub, so an edge almost always
 * joins two adjacent rings and the result reads as a hierarchy without being
 * one. Within a ring, nodes are ordered by their parent's angle so siblings
 * stay together and edges run roughly outward instead of crossing the middle.
 */
export function applyRadialMapLayout(
  nodes: Node[],
  edges: Edge[],
): { nodes: Node[]; edges: Edge[] } {
  if (nodes.length === 0) return { nodes, edges }

  const adj = buildAdjacency(nodes, edges)
  const comps = components(adj)
  const main = comps[0] ?? []

  // Hub = the most connected node in the largest component. Ties break on id
  // so the map does not re-centre between refreshes when degrees are equal.
  let hub = main[0]
  let bestDeg = -1
  for (const id of main) {
    const deg = adj.neighbours.get(id)?.size ?? 0
    if (deg > bestDeg || (deg === bestDeg && id < hub)) {
      bestDeg = deg
      hub = id
    }
  }

  // BFS levels + the parent each node was discovered from.
  const level = new Map<string, number>([[hub, 0]])
  const parent = new Map<string, string>()
  const order: string[] = [hub]
  const queue = [hub]
  while (queue.length) {
    const cur = queue.shift()!
    const d = level.get(cur)!
    for (const nb of adj.neighbours.get(cur) ?? []) {
      if (level.has(nb)) continue
      level.set(nb, d + 1)
      parent.set(nb, cur)
      order.push(nb)
      queue.push(nb)
    }
  }

  const pos = new Map<string, { x: number; y: number }>()
  pos.set(hub, { x: 0, y: 0 })

  const angleOf = new Map<string, number>()
  angleOf.set(hub, -Math.PI / 2)

  // Cap the ring count. BFS depth is unbounded — a host → runtime → namespace
  // → deployment → pod chain is already 5+ hops, so a real graph reaches
  // depth 20-30 and `level * RING_GAP` pushed the outermost ring thousands of
  // pixels out, at which point fit-view zooms out to ~10% and the map is
  // unreadable. Levels past the cap share the outermost ring instead.
  const ringOf = (d: number) => Math.min(d, MAX_RINGS)
  const rings = new Map<number, string[]>()
  for (const id of order) {
    const r = ringOf(level.get(id)!)
    rings.set(r, [...(rings.get(r) ?? []), id])
  }

  for (const d of [...rings.keys()].sort((a, b) => a - b)) {
    const ring = rings.get(d)!

    // Stable ordering: group by parent, and order parents by their own angle
    // so each cluster of children sits under its parent.
    const parentAngle = (id: string) => angleOf.get(parent.get(id) ?? hub) ?? 0
    ring.sort((a, b) => {
      const pa = parentAngle(a)
      const pb = parentAngle(b)
      if (pa !== pb) return pa - pb
      return a < b ? -1 : 1
    })

    // Grow the ring until its circumference can hold every node, so a level
    // with 30 members spreads out instead of stacking on top of each other.
    const need = ring.length * NODE_SLOT_W
    const radius = Math.max(d * RING_GAP, need / (2 * Math.PI))

    ring.forEach((id, i) => {
      // Offset each ring by half a step so nodes on adjacent rings do not line
      // up radially and look like a grid.
      const theta = (i / ring.length) * Math.PI * 2 - Math.PI / 2 + (d % 2 ? Math.PI / ring.length : 0)
      angleOf.set(id, theta)
      pos.set(id, { x: Math.cos(theta) * radius, y: Math.sin(theta) * radius })
    })
  }

  // Disconnected components (no path to the hub at all) would otherwise be
  // dropped on the floor. Park them in a wide band below the map, in rows.
  const orphanCount = nodes.length - pos.size
  let oy = Math.max(0, ...[...pos.values()].map((p) => p.y)) + RING_GAP
  const perRow = Math.max(1, Math.floor(1400 / NODE_SLOT_W))
  const orphans = nodes.map((n) => n.id).filter((id) => !pos.has(id)).sort()
  orphans.forEach((id, i) => {
    const col = i % perRow
    const row = Math.floor(i / perRow)
    pos.set(id, { x: col * NODE_SLOT_W - (perRow * NODE_SLOT_W) / 2, y: oy + row * NODE_SLOT_H })
  })

  const raw = nodes.map((n) => ({ id: n.id, x: pos.get(n.id)?.x ?? 0, y: pos.get(n.id)?.y ?? 0 }))

  // Resolve residual collisions.
  //
  // Ring radius is chosen so a *full* ring has enough circumference, but
  // nodes are boxes, not points: two on adjacent rings, or one on the outer
  // ring and one in the band below, can still land within a box-width of each
  // other. A fixed number of relaxation passes nudges overlapping pairs apart
  // along the line between them. It is deterministic — same input, same
  // output — so it does not reintroduce the jitter that made a force sim
  // unusable on a canvas that re-renders every 15s.
  const MIN_DX = NODE_SLOT_W
  const MIN_DY = NODE_SLOT_H
  for (let pass = 0; pass < 60; pass++) {
    let moved = false
    for (let i = 0; i < raw.length; i++) {
      for (let j = i + 1; j < raw.length; j++) {
        const a = raw[i]
        const b = raw[j]
        let dx = b.x - a.x
        let dy = b.y - a.y
        if (Math.abs(dx) >= MIN_DX || Math.abs(dy) >= MIN_DY) continue
        // Separating axis is whichever gap is smaller — push along it.
        const penX = MIN_DX - Math.abs(dx)
        const penY = MIN_DY - Math.abs(dy)
        if (penX < penY) {
          const dir = dx >= 0 ? 1 : -1
          a.x -= (penX / 2) * dir
          b.x += (penX / 2) * dir
        } else {
          const dir = dy >= 0 ? 1 : -1
          a.y -= (penY / 2) * dir
          b.y += (penY / 2) * dir
        }
        moved = true
      }
    }
    if (!moved) break
  }

  // Centre the result on the hub so the map is not off in a corner.
  const hubPt = raw.find((r) => r.id === hub) ?? { x: 0, y: 0 }
  const out = nodes.map((n) => {
    const r = raw.find((q) => q.id === n.id)
    return {
      ...n,
      position: {
        x: (r?.x ?? hubPt.x) - hubPt.x - MAP_NODE_WIDTH / 2,
        y: (r?.y ?? hubPt.y) - hubPt.y - MAP_NODE_HEIGHT / 2,
      },
    }
  })

  void orphanCount
  return { nodes: out, edges }
}
