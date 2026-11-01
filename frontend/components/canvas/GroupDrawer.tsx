'use client'

import { useState, useMemo, useRef, useEffect } from 'react'
import { X, Search } from 'lucide-react'
import { type GroupInfo } from '@/lib/graphPreprocess'
import { type NodeHealth } from '@/types'
import NodeSvgIcon from './NodeSvgIcon'

interface GroupDrawerProps {
  group: GroupInfo
  onClose: () => void
  onSelectNode: (nodeId: string) => void
  /** Dim the canvas to one namespace. Fired when a namespace row is picked. */
  onFocusNamespace?: (namespace: string | null) => void
}

const HEALTH_ORDER: Record<NodeHealth | string, number> = {
  unhealthy: 0,
  degraded: 1,
  healthy: 2,
  unknown: 3,
}

const HEALTH_COLOR: Record<string, string> = {
  healthy: '#22c55e',
  degraded: '#f59e0b',
  unhealthy: '#ef4444',
  unknown: 'var(--ink3)',
}

const HEALTH_BG: Record<string, string> = {
  healthy: 'rgba(34,197,94,0.1)',
  degraded: 'rgba(245,158,11,0.1)',
  unhealthy: 'rgba(239,68,68,0.1)',
  unknown: 'rgba(71,85,105,0.1)',
}

export default function GroupDrawer({ group, onClose, onSelectNode, onFocusNamespace }: GroupDrawerProps) {
  const [query, setQuery] = useState('')
  const [healthFilter, setHealthFilter] = useState<string | null>(null)
  // Multi-select namespace filter. Empty set = show every namespace. A group
  // of pods spans all namespaces, so without this the list is a wall of
  // unrelated workloads with no way to narrow it beyond typing a name.
  const [nsFilter, setNsFilter] = useState<Set<string>>(new Set())

  // Resizable panel width — shares the same persisted key as NodeDetailPanel
  // so both side panels stay the same width.
  const MIN_W = 320, MAX_W = 760
  const [panelWidth, setPanelWidth] = useState<number>(() => {
    if (typeof window === 'undefined') return 340
    const saved = Number(window.localStorage.getItem('ic-panel-width'))
    return saved >= MIN_W && saved <= MAX_W ? saved : 340
  })
  const panelWidthRef = useRef(panelWidth)
  useEffect(() => { panelWidthRef.current = panelWidth }, [panelWidth])
  const draggingRef = useRef(false)
  function startResize(e: React.MouseEvent) {
    e.preventDefault()
    draggingRef.current = true
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
    const onMove = (ev: MouseEvent) => {
      if (!draggingRef.current) return
      setPanelWidth(Math.min(MAX_W, Math.max(MIN_W, window.innerWidth - ev.clientX)))
    }
    const onUp = () => {
      draggingRef.current = false
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
      window.localStorage.setItem('ic-panel-width', String(panelWidthRef.current))
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }


  // Distinct namespaces present in this group, with counts. Most-populated
  // first so the useful ones lead. Empty when the group has no namespaced
  // nodes (containers, host processes, ...) — the filter row then hides.
  const namespaceOptions = useMemo(() => {
    const counts = new Map<string, number>()
    for (const n of group.nodes) {
      const ns = n.metadata?.namespace
      if (typeof ns === 'string' && ns) counts.set(ns, (counts.get(ns) ?? 0) + 1)
    }
    return [...counts.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
  }, [group.nodes])

  // A filter left over from a previous group would silently hide everything.
  useEffect(() => { setNsFilter(new Set()) }, [group.id])

  function toggleNamespace(ns: string) {
    setNsFilter((prev) => {
      const next = new Set(prev)
      if (next.has(ns)) next.delete(ns)
      else next.add(ns)
      return next
    })
  }

  const filtered = useMemo(() => {
    let nodes = [...group.nodes]

    if (healthFilter) {
      nodes = nodes.filter((n) => n.health === healthFilter)
    }

    if (nsFilter.size > 0) {
      nodes = nodes.filter((n) => nsFilter.has(String(n.metadata?.namespace ?? '')))
    }

    if (query.trim()) {
      const q = query.trim().toLowerCase()
      nodes = nodes.filter(
        (n) =>
          n.label.toLowerCase().includes(q) ||
          n.id.toLowerCase().includes(q)
      )
    }

    // Sort: worst health first, then alphabetical
    nodes.sort((a, b) => {
      const ha = HEALTH_ORDER[a.health] ?? 3
      const hb = HEALTH_ORDER[b.health] ?? 3
      if (ha !== hb) return ha - hb
      return a.label.localeCompare(b.label)
    })

    return nodes
  }, [group.nodes, query, healthFilter, nsFilter])

  // Build health filter options from actual data
  const healthOptions = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const n of group.nodes) {
      counts[n.health] = (counts[n.health] ?? 0) + 1
    }
    return Object.entries(counts)
      .sort(([a], [b]) => (HEALTH_ORDER[a] ?? 3) - (HEALTH_ORDER[b] ?? 3))
  }, [group.nodes])

  function getKeyMeta(node: typeof group.nodes[0]): string {
    const m = node.metadata
    if (m.namespace) return `ns: ${m.namespace}`
    if (m.state) return String(m.state)
    if (m.status) return String(m.status)
    if (m.image) return String(m.image).split('/').pop()?.split(':')[0] ?? ''
    return ''
  }

  return (
    <div
      style={{
        position: 'absolute',
        right: 0,
        top: 0,
        bottom: 0,
        width: `min(${panelWidth}px, 100%)`,
        background: 'var(--surface)',
        borderLeft: '1px solid var(--line2)',
        display: 'flex',
        flexDirection: 'column',
        zIndex: 30,
      }}
    >
      {/* Drag handle to resize the panel. */}
      <div
        onMouseDown={startResize}
        title="Drag to resize"
        style={{ position: 'absolute', left: -3, top: 0, bottom: 0, width: 6, cursor: 'col-resize', zIndex: 40 }}
        onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--line2)' }}
        onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
      />
      {/* ── Header ── */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '12px 16px',
          borderBottom: '1px solid var(--line)',
          flexShrink: 0,
        }}
      >
        <div
          style={{
            width: 32,
            height: 32,
            borderRadius: 8,
            background: 'var(--surface-2)',
            border: '1px solid var(--line2)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
          }}
        >
          <NodeSvgIcon type={group.type} size={16} />
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <p style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)', lineHeight: 1 }}>
            {group.label}
          </p>
          <p style={{ fontSize: 11, color: 'var(--ink3)', marginTop: 2 }}>
            {group.count} {group.count === 1 ? 'node' : 'nodes'}
          </p>
        </div>
        <button
          onClick={onClose}
          style={{
            width: 28,
            height: 28,
            borderRadius: 7,
            border: 'none',
            background: 'transparent',
            color: 'var(--ink3)',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.background = 'var(--surface)'
            e.currentTarget.style.color = 'var(--ink2)'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.background = 'transparent'
            e.currentTarget.style.color = 'var(--ink3)'
          }}
        >
          <X size={14} />
        </button>
      </div>

      {/* ── Health summary bar ── */}
      <div style={{ padding: '10px 16px', borderBottom: '1px solid var(--line)', flexShrink: 0 }}>
        <div
          style={{
            display: 'flex',
            height: 6,
            borderRadius: 3,
            overflow: 'hidden',
            gap: 1.5,
            marginBottom: 8,
          }}
        >
          {healthOptions.map(([h, c]) => (
            <div
              key={h}
              title={`${c} ${h}`}
              style={{
                width: `${(c / group.count) * 100}%`,
                background: HEALTH_COLOR[h] ?? 'var(--ink4)',
                borderRadius: 2,
              }}
            />
          ))}
        </div>
        {/* Health filter chips */}
        <div style={{ display: 'flex', gap: 5, flexWrap: 'wrap' }}>
          <button
            onClick={() => setHealthFilter(null)}
            style={{
              padding: '2px 9px',
              borderRadius: 20,
              border: '1px solid',
              fontSize: 11,
              cursor: 'pointer',
              background: healthFilter === null ? 'var(--ink)' : 'transparent',
              color: healthFilter === null ? 'var(--bg)' : 'var(--ink3)',
              borderColor: healthFilter === null ? 'var(--ink)' : 'var(--line)',
              fontWeight: healthFilter === null ? 600 : 400,
            }}
          >
            All
          </button>
          {healthOptions.map(([h, c]) => (
            <button
              key={h}
              onClick={() => setHealthFilter(healthFilter === h ? null : h)}
              style={{
                padding: '2px 9px',
                borderRadius: 20,
                border: '1px solid',
                fontSize: 11,
                cursor: 'pointer',
                background: healthFilter === h ? HEALTH_BG[h] : 'transparent',
                color: healthFilter === h ? HEALTH_COLOR[h] : 'var(--ink3)',
                borderColor: healthFilter === h ? `${HEALTH_COLOR[h]}40` : 'var(--line)',
                fontWeight: healthFilter === h ? 600 : 400,
              }}
            >
              {h} ({c})
            </button>
          ))}
        </div>
      </div>

      {/* ── Namespace filter ──
          A pod group spans every namespace in the cluster, so the drawer
          needs its own namespace filter; without it the only way to narrow
          38 pods is to type a substring. Multi-select, with a per-namespace
          count so you can see where the pods actually are. */}
      {namespaceOptions.length > 1 && (
        <div style={{ padding: '8px 16px', borderBottom: '1px solid var(--line)', flexShrink: 0 }}>
          <div style={{ display: 'flex', gap: 5, flexWrap: 'wrap' }}>
            <button
              onClick={() => setNsFilter(new Set())}
              title="Show every namespace"
              style={{
                padding: '2px 9px',
                borderRadius: 20,
                border: '1px solid',
                fontSize: 11,
                cursor: 'pointer',
                background: nsFilter.size === 0 ? 'var(--ink)' : 'transparent',
                color: nsFilter.size === 0 ? 'var(--bg)' : 'var(--ink3)',
                borderColor: nsFilter.size === 0 ? 'var(--ink)' : 'var(--line)',
                fontWeight: nsFilter.size === 0 ? 600 : 400,
              }}
            >
              All namespaces
            </button>
            {namespaceOptions.map(([ns, c]) => {
              const on = nsFilter.has(ns)
              return (
                <button
                  key={ns}
                  onClick={() => toggleNamespace(ns)}
                  // Pick the namespace in the drawer and dim the canvas to it,
                  // so the list and the topology agree on what you're looking at.
                  onDoubleClick={() => onFocusNamespace?.(ns)}
                  title={`${ns} — ${c} node${c === 1 ? '' : 's'}. Click to toggle, double-click to highlight on the canvas.`}
                  style={{
                    padding: '2px 9px',
                    borderRadius: 20,
                    border: '1px solid',
                    fontSize: 11,
                    cursor: 'pointer',
                    background: on ? 'var(--surface-2)' : 'transparent',
                    color: on ? 'var(--ink)' : 'var(--ink3)',
                    borderColor: on ? 'var(--ink)' : 'var(--line)',
                    fontWeight: on ? 600 : 400,
                  }}
                >
                  {ns} ({c})
                </button>
              )
            })}
          </div>
        </div>
      )}

      {/* ── Search ── */}
      <div style={{ padding: '8px 16px', borderBottom: '1px solid var(--line)', flexShrink: 0 }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 8,
            background: 'var(--bg)',
            border: '1px solid var(--line)',
            borderRadius: 8,
            padding: '6px 10px',
          }}
        >
          <Search size={12} color="var(--ink4)" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filter by name…"
            style={{
              flex: 1,
              background: 'transparent',
              border: 'none',
              outline: 'none',
              fontSize: 12,
              color: 'var(--ink)',
            }}
          />
        </div>
      </div>

      {/* ── Node list ── */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '6px 0' }}>
        {filtered.length === 0 ? (
          <p style={{ fontSize: 12, color: 'var(--ink4)', padding: '16px', textAlign: 'center' }}>
            No matches
          </p>
        ) : (
          filtered.map((node) => {
            const hc = HEALTH_COLOR[node.health] ?? 'var(--ink3)'
            const keyMeta = getKeyMeta(node)
            return (
              <button
                key={node.id}
                onClick={() => onSelectNode(node.id)}
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  padding: '8px 16px',
                  background: 'transparent',
                  border: 'none',
                  borderBottom: '1px solid var(--line)',
                  cursor: 'pointer',
                  textAlign: 'left',
                }}
                onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--surface-2)' }}
                onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
              >
                {/* Health dot */}
                <span
                  style={{
                    width: 7,
                    height: 7,
                    borderRadius: '50%',
                    background: hc,
                    flexShrink: 0,
                  }}
                />
                {/* Name + meta */}
                <div style={{ flex: 1, minWidth: 0 }}>
                  <p
                    style={{
                      fontSize: 12,
                      fontWeight: 500,
                      color: 'var(--ink2)',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                    title={node.label}
                  >
                    {node.label}
                  </p>
                  {keyMeta && (
                    <p
                      style={{
                        fontSize: 10,
                        color: 'var(--ink4)',
                        fontFamily: "var(--font-geist-mono,'Geist Mono',ui-monospace,monospace)",
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                        marginTop: 1,
                      }}
                    >
                      {keyMeta}
                    </p>
                  )}
                </div>
                {/* Health badge */}
                <span
                  style={{
                    fontSize: 10,
                    padding: '2px 7px',
                    borderRadius: 20,
                    background: HEALTH_BG[node.health] ?? 'rgba(71,85,105,0.1)',
                    color: hc,
                    flexShrink: 0,
                    fontWeight: 500,
                  }}
                >
                  {node.health}
                </span>
              </button>
            )
          })
        )}
      </div>

      {/* ── Footer count ── */}
      <div
        style={{
          padding: '8px 16px',
          borderTop: '1px solid var(--line)',
          flexShrink: 0,
        }}
      >
        <p style={{ fontSize: 11, color: 'var(--ink4)' }}>
          Showing {filtered.length} of {group.count}
        </p>
      </div>
    </div>
  )
}
