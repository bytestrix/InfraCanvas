'use client'

import { useState, useMemo } from 'react'
import { X, Search } from 'lucide-react'
import { type GroupInfo } from '@/lib/graphPreprocess'
import { getNodeColor, getNodeIcon, type NodeHealth } from '@/types'

interface GroupDrawerProps {
  group: GroupInfo
  onClose: () => void
  onSelectNode: (nodeId: string) => void
}

const HEALTH_ORDER: Record<NodeHealth | string, number> = {
  unhealthy: 0,
  degraded: 1,
  healthy: 2,
  unknown: 3,
}

const HEALTH_COLOR: Record<string, string> = {
  healthy: '#10b981',
  degraded: '#f59e0b',
  unhealthy: '#ef4444',
  unknown: '#475569',
}

const HEALTH_BG: Record<string, string> = {
  healthy: 'rgba(16,185,129,0.1)',
  degraded: 'rgba(245,158,11,0.1)',
  unhealthy: 'rgba(239,68,68,0.1)',
  unknown: 'rgba(71,85,105,0.1)',
}

export default function GroupDrawer({ group, onClose, onSelectNode }: GroupDrawerProps) {
  const [query, setQuery] = useState('')
  const [healthFilter, setHealthFilter] = useState<string | null>(null)

  const color = getNodeColor(group.type)
  const icon = getNodeIcon(group.type)

  const filtered = useMemo(() => {
    let nodes = [...group.nodes]

    if (healthFilter) {
      nodes = nodes.filter((n) => n.health === healthFilter)
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
  }, [group.nodes, query, healthFilter])

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
        width: 340,
        background: '#0a0a16',
        borderLeft: '1px solid #1e1e3a',
        display: 'flex',
        flexDirection: 'column',
        zIndex: 30,
        boxShadow: '-8px 0 32px rgba(0,0,0,0.5)',
      }}
    >
      {/* ── Header ── */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '12px 16px',
          borderBottom: '1px solid #1e1e3a',
          flexShrink: 0,
        }}
      >
        <div
          style={{
            width: 32,
            height: 32,
            borderRadius: 8,
            background: `${color}18`,
            border: `1px solid ${color}28`,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 15,
            color,
            flexShrink: 0,
          }}
        >
          {icon}
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <p style={{ fontSize: 13, fontWeight: 600, color: '#e2e8f0', lineHeight: 1 }}>
            {group.label}
          </p>
          <p style={{ fontSize: 11, color: '#475569', marginTop: 2 }}>
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
            color: '#475569',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.background = '#13131f'
            e.currentTarget.style.color = '#94a3b8'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.background = 'transparent'
            e.currentTarget.style.color = '#475569'
          }}
        >
          <X size={14} />
        </button>
      </div>

      {/* ── Health summary bar ── */}
      <div style={{ padding: '10px 16px', borderBottom: '1px solid #1e1e3a', flexShrink: 0 }}>
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
                background: HEALTH_COLOR[h] ?? '#334155',
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
              background: healthFilter === null ? '#e2e8f0' : 'transparent',
              color: healthFilter === null ? '#070711' : '#475569',
              borderColor: healthFilter === null ? '#e2e8f0' : '#1e1e3a',
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
                color: healthFilter === h ? HEALTH_COLOR[h] : '#475569',
                borderColor: healthFilter === h ? `${HEALTH_COLOR[h]}40` : '#1e1e3a',
                fontWeight: healthFilter === h ? 600 : 400,
              }}
            >
              {h} ({c})
            </button>
          ))}
        </div>
      </div>

      {/* ── Search ── */}
      <div style={{ padding: '8px 16px', borderBottom: '1px solid #1e1e3a', flexShrink: 0 }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 8,
            background: '#070711',
            border: '1px solid #1e1e3a',
            borderRadius: 8,
            padding: '6px 10px',
          }}
        >
          <Search size={12} color="#334155" />
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
              color: '#e2e8f0',
            }}
          />
        </div>
      </div>

      {/* ── Node list ── */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '6px 0' }}>
        {filtered.length === 0 ? (
          <p style={{ fontSize: 12, color: '#334155', padding: '16px', textAlign: 'center' }}>
            No matches
          </p>
        ) : (
          filtered.map((node) => {
            const hc = HEALTH_COLOR[node.health] ?? '#475569'
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
                  borderBottom: '1px solid #0f0f1e',
                  cursor: 'pointer',
                  textAlign: 'left',
                }}
                onMouseEnter={(e) => { e.currentTarget.style.background = '#0f0f20' }}
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
                      color: '#cbd5e1',
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
                        color: '#334155',
                        fontFamily: 'JetBrains Mono, monospace',
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
          borderTop: '1px solid #1e1e3a',
          flexShrink: 0,
        }}
      >
        <p style={{ fontSize: 11, color: '#334155' }}>
          Showing {filtered.length} of {group.count}
        </p>
      </div>
    </div>
  )
}
