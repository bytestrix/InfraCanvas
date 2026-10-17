/**
 * structuredMap.ts
 *
 * Structured node layout for the map view.
 *
 * The first version of the map arranged everything on concentric rings around
 * the most-connected node. With one host that produced a legible hub-and-spokes
 * diagram; with the 189-node kind cluster it produced a uniform circle of 189
 * identical discs where a Pod, a Service and a PVC were indistinguishable. A
 * free radial layout has no structure to read once the graph gets big.
 *
 * The grouped canvas *did* have structure — namespaces owning resource types —
 * but it spent that structure on count cards, so you still had to drill in to
 * learn anything.
 *
 * This keeps both: nodes are bucketed by the thing that owns them (namespace
 * for k8s resources, type otherwise), each bucket gets its own labelled block
 * laid out as a grid of individual nodes, and edges run between the real nodes.
 * So the shape on screen is the shape of the cluster, and every single resource
 * in it is visible and named.
 *
 * Deterministic throughout — the agent pushes a snapshot every 15s, so the
 * layout must not reshuffle between refreshes.
 */

import { Node, Edge } from 'reactflow'
import { MAP_NODE_WIDTH, MAP_NODE_HEIGHT } from './radialMap'

const CELL_W = MAP_NODE_WIDTH + 16
const CELL_H = MAP_NODE_HEIGHT + 14
const PAD = 18
const HEADER_H = 34
const BLOCK_GAP = 34

/** Owner key: the namespace for namespaced k8s resources, else the type. */
export function bucketKey(n: Node): string {
  const d = n.data as { metadata?: Record<string, unknown>; nodeType?: string }
  const ns = d?.metadata?.namespace
  const type = d?.nodeType ?? n.type ?? 'other'
  if (typeof ns === 'string' && ns) return `ns:${ns}`
  return `type:${type}`
}

export function bucketLabel(key: string): string {
  const [kind, rest] = key.split(':')
  if (kind === 'ns') return rest
  return rest.charAt(0).toUpperCase() + rest.slice(1)
}

/** Plural label used on the block header, e.g. "8 pods". */
export function bucketPlural(key: string, count: number): string {
  const [kind, rest] = key.split(':')
  if (kind === 'ns') return `${count} resource${count === 1 ? '' : 's'}`
  const s = rest.endsWith('s') ? rest : rest + 's'
  return `${count} ${s}`
}

export function applyStructuredMapLayout(
  nodes: Node[],
  edges: Edge[],
): { nodes: Node[]; edges: Edge[] } {
  if (nodes.length === 0) return { nodes, edges }

  // 1. Bucket by owner, preserving the order they appear in.
  const buckets = new Map<string, Node[]>()
  for (const n of nodes) {
    const k = bucketKey(n)
    buckets.set(k, [...(buckets.get(k) ?? []), n])
  }

  // 2. Sort each bucket by type then name so related resources sit together
  //    and the block is stable between refreshes.
  for (const [, list] of buckets) {
    list.sort((a, b) => {
      const ta = String((a.data as { nodeType?: string })?.nodeType ?? '')
      const tb = String((b.data as { nodeType?: string })?.nodeType ?? '')
      if (ta !== tb) return ta < tb ? -1 : 1
      const la = String((a.data as { label?: string })?.label ?? a.id)
      const lb = String((b.data as { label?: string })?.label ?? b.id)
      return la < lb ? -1 : 1
    })
  }

  // 3. Namespace blocks first and largest-to-smallest, then type blocks.
  //    Namespaces are what a Kubernetes user navigates by, so they get the top
  //    band and the widest columns.
  const keys = [...buckets.keys()].sort((a, b) => {
    const na = a.startsWith('ns:') ? 0 : 1
    const nb = b.startsWith('ns:') ? 0 : 1
    if (na !== nb) return na - nb
    return buckets.get(b)!.length - buckets.get(a)!.length
  })

  // 4. Wrap blocks into rows of at most MAX_BLOCK_W, keeping each block's
  //    height so a row's height is the tallest block in it.
  // How wide one block may get, and how wide a row of blocks may get before
  // wrapping. These are separate: capping the row at the block width meant one
  // block per row, which stacked every namespace into a single tall column and
  // wasted all the horizontal space.
  const MAX_BLOCK_W = 1180
  const ROW_W = MAX_BLOCK_W * 2
  interface Placed { key: string; list: Node[]; cols: number; rows: number; w: number; h: number }
  const placed: Placed[] = []

  // Widest a single block may be, so one large bucket cannot blow out the row.
  const maxCols = Math.max(1, Math.floor((MAX_BLOCK_W - PAD * 2) / CELL_W))

  for (const key of keys) {
    const list = buckets.get(key)!
    // Aim for a roughly 16:9 block: cols ≈ sqrt(n * 1.8), then clamp so a
    // 155-resource namespace becomes a tall readable column rather than a
    // single 3500px-wide strip that pushes everything else off screen.
    const cols = Math.min(maxCols, Math.max(1, Math.ceil(Math.sqrt(list.length * 1.8))))
    const rows = Math.ceil(list.length / cols)
    const w = Math.max(cols * CELL_W + PAD * 2, 300)
    const h = HEADER_H + PAD + rows * CELL_H + PAD
    placed.push({ key, list, cols, rows, w, h })
  }

  const out: Node[] = []
  let x = 0
  let y = 0
  let rowH = 0

  for (const b of placed) {
    // Wrap when this block would overflow the row width and the row has content.
    if (x > 0 && x + b.w > ROW_W) {
      x = 0
      y += rowH + BLOCK_GAP
      rowH = 0
    }

    // Header for the block.
    out.push({
      id: `__bucket__${b.key}`,
      type: 'mapLabel',
      position: { x, y },
      data: {
        label: bucketLabel(b.key),
        sub: bucketPlural(b.key, b.list.length),
        kind: b.key.startsWith('ns:') ? 'ns' : 'type',
      },
      width: b.w,
      height: HEADER_H,
      selectable: false,
      draggable: false,
      zIndex: 1,
    } as Node)

    // Nodes in a grid beneath the header.
    b.list.forEach((n, i) => {
      const col = i % b.cols
      const row = Math.floor(i / b.cols)
      out.push({
        ...n,
        position: {
          x: x + PAD + col * CELL_W,
          y: y + HEADER_H + PAD + row * CELL_H,
        },
      })
    })

    x += b.w + BLOCK_GAP
    rowH = Math.max(rowH, b.h)
  }

  return { nodes: out, edges }
}
