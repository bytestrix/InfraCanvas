'use client'

import { useState, useMemo, useCallback } from 'react'
import { X, Search, Trash2, CheckSquare, Square, AlertTriangle, Loader2 } from 'lucide-react'
import { type GroupInfo } from '@/lib/graphPreprocess'
import { type NodeHealth } from '@/types'
import { sendAction, sendCommand } from '@/lib/wsManager'
import NodeSvgIcon from './NodeSvgIcon'

// Node types that support bulk delete and their action builders
const BULK_DELETE_SUPPORT: Partial<Record<string, (name: string, namespace: string) => object>> = {
  job: (name, namespace) => ({
    action_id: `del-${Date.now()}-${name}`,
    type: 'k8s_delete_job',
    target: { layer: 'kubernetes', entity_type: 'job', entity_id: name, namespace },
    parameters: {},
  }),
  pod: (name, namespace) => ({
    action_id: `del-${Date.now()}-${name}`,
    type: 'k8s_delete_pod',
    target: { layer: 'kubernetes', entity_type: 'pod', entity_id: name, namespace },
    parameters: {},
  }),
}

interface GroupDrawerProps {
  group: GroupInfo
  vmCode: string
  initialHealthFilter?: string
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
  healthy: '#22c55e',
  degraded: '#f59e0b',
  unhealthy: '#ef4444',
  unknown: '#6b7280',
}

const HEALTH_BG: Record<string, string> = {
  healthy: 'rgba(34,197,94,0.1)',
  degraded: 'rgba(245,158,11,0.1)',
  unhealthy: 'rgba(239,68,68,0.1)',
  unknown: 'rgba(107,114,128,0.1)',
}

const MONO = "var(--font-geist-mono,'Geist Mono','JetBrains Mono',ui-monospace,monospace)"

export default function GroupDrawer({ group, vmCode, initialHealthFilter, onClose, onSelectNode }: GroupDrawerProps) {
  const [query, setQuery] = useState('')
  const [healthFilter, setHealthFilter] = useState<string | null>(initialHealthFilter ?? null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [bulkStatus, setBulkStatus] = useState<'idle' | 'confirming' | 'running' | 'done'>('idle')
  const [deletedCount, setDeletedCount] = useState(0)

  const bulkDeleteFn = BULK_DELETE_SUPPORT[group.type]

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

    nodes.sort((a, b) => {
      const ha = HEALTH_ORDER[a.health] ?? 3
      const hb = HEALTH_ORDER[b.health] ?? 3
      if (ha !== hb) return ha - hb
      return a.label.localeCompare(b.label)
    })

    return nodes
  }, [group.nodes, query, healthFilter])

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

  const toggleSelect = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const selectAllFiltered = useCallback(() => {
    setSelectedIds(new Set(filtered.map((n) => n.id)))
  }, [filtered])

  const selectAllDegraded = useCallback(() => {
    const ids = group.nodes
      .filter((n) => n.health === 'degraded' || n.health === 'unhealthy')
      .map((n) => n.id)
    setSelectedIds(new Set(ids))
    setHealthFilter(null)
  }, [group.nodes])

  const clearSelection = useCallback(() => {
    setSelectedIds(new Set())
    setBulkStatus('idle')
    setDeletedCount(0)
  }, [])

  async function runBulkDelete() {
    if (!bulkDeleteFn) return
    const toDelete = group.nodes.filter((n) => selectedIds.has(n.id))
    setBulkStatus('running')
    setDeletedCount(0)
    let count = 0
    for (const node of toDelete) {
      const name = node.metadata.name ?? node.id
      const ns = node.metadata.namespace ?? 'default'
      sendAction(vmCode, bulkDeleteFn(name, ns))
      count++
      setDeletedCount(count)
      await new Promise((r) => setTimeout(r, 200))
    }
    setBulkStatus('done')
    sendCommand(vmCode, 'refresh')
    setTimeout(() => sendCommand(vmCode, 'refresh'), 4000)
    setTimeout(() => {
      clearSelection()
    }, 3000)
  }

  const selectedCount = selectedIds.size
  const degradedCount = group.nodes.filter(
    (n) => n.health === 'degraded' || n.health === 'unhealthy'
  ).length

  return (
    <div
      style={{
        position: 'absolute',
        right: 0,
        top: 0,
        bottom: 0,
        width: 'min(360px, 100%)',
        background: 'var(--bg)',
        borderLeft: '1px solid var(--line)',
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
            {degradedCount > 0 && (
              <span style={{ color: '#f59e0b', marginLeft: 6 }}>· {degradedCount} degraded/failed</span>
            )}
          </p>
        </div>
        <button
          onClick={onClose}
          style={{
            width: 28, height: 28, borderRadius: 7, border: 'none',
            background: 'transparent', color: 'var(--ink4)', cursor: 'pointer',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}
          onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--surface-2)'; e.currentTarget.style.color = 'var(--ink2)' }}
          onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--ink4)' }}
        >
          <X size={14} />
        </button>
      </div>

      {/* ── Bulk action bar (job/pod types only) ── */}
      {bulkDeleteFn && (
        <div style={{
          padding: '8px 12px',
          borderBottom: '1px solid var(--line)',
          flexShrink: 0,
          background: selectedCount > 0 ? 'rgba(239,68,68,0.04)' : 'var(--bg)',
        }}>
          {bulkStatus === 'done' ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: '#22c55e' }}>
              <span>✓ Deleted {deletedCount} {group.type}s — refreshing…</span>
            </div>
          ) : bulkStatus === 'running' ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: 'var(--ink2)' }}>
              <Loader2 size={13} style={{ animation: 'spin 1s linear infinite' }} />
              <span>Deleting {deletedCount}/{selectedCount}…</span>
            </div>
          ) : bulkStatus === 'confirming' ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12 }}>
                <AlertTriangle size={13} color="#f59e0b" />
                <span style={{ color: '#f59e0b' }}>
                  Delete {selectedCount} {group.type}{selectedCount !== 1 ? 's' : ''}? This cannot be undone.
                </span>
              </div>
              <div style={{ display: 'flex', gap: 6 }}>
                <button
                  onClick={runBulkDelete}
                  style={{
                    flex: 1, padding: '5px 0', borderRadius: 6, border: 'none',
                    background: '#ef4444', color: '#fff', fontSize: 12,
                    fontWeight: 600, cursor: 'pointer',
                  }}
                >
                  Confirm Delete
                </button>
                <button
                  onClick={() => setBulkStatus('idle')}
                  style={{
                    padding: '5px 14px', borderRadius: 6,
                    border: '1px solid var(--line)', background: 'transparent',
                    color: 'var(--ink3)', fontSize: 12, cursor: 'pointer',
                  }}
                >
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
              {degradedCount > 0 && selectedCount === 0 && (
                <button
                  onClick={selectAllDegraded}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 5,
                    padding: '4px 10px', borderRadius: 6,
                    border: '1px solid rgba(245,158,11,0.3)',
                    background: 'rgba(245,158,11,0.08)',
                    color: '#f59e0b', fontSize: 11, cursor: 'pointer',
                  }}
                >
                  <CheckSquare size={12} />
                  Select all degraded ({degradedCount})
                </button>
              )}
              {filtered.length > 0 && selectedCount < filtered.length && (
                <button
                  onClick={selectAllFiltered}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 5,
                    padding: '4px 10px', borderRadius: 6,
                    border: '1px solid var(--line)',
                    background: 'transparent',
                    color: 'var(--ink3)', fontSize: 11, cursor: 'pointer',
                  }}
                >
                  <CheckSquare size={12} />
                  {healthFilter ? `Select all ${healthFilter}` : 'Select all'} ({filtered.length})
                </button>
              )}
              {selectedCount > 0 && (
                <>
                  <button
                    onClick={clearSelection}
                    style={{
                      display: 'flex', alignItems: 'center', gap: 5,
                      padding: '4px 10px', borderRadius: 6,
                      border: '1px solid var(--line)', background: 'transparent',
                      color: 'var(--ink3)', fontSize: 11, cursor: 'pointer',
                    }}
                  >
                    <Square size={12} />
                    Clear
                  </button>
                  <button
                    onClick={() => setBulkStatus('confirming')}
                    style={{
                      display: 'flex', alignItems: 'center', gap: 5,
                      padding: '4px 10px', borderRadius: 6,
                      border: '1px solid rgba(239,68,68,0.4)',
                      background: 'rgba(239,68,68,0.1)',
                      color: '#ef4444', fontSize: 11,
                      fontWeight: 600, cursor: 'pointer',
                      marginLeft: 'auto',
                    }}
                  >
                    <Trash2 size={12} />
                    Delete {selectedCount} selected
                  </button>
                </>
              )}
              {selectedCount === 0 && degradedCount === 0 && (
                <span style={{ fontSize: 11, color: 'var(--ink4)' }}>Select items to perform bulk actions</span>
              )}
            </div>
          )}
        </div>
      )}

      {/* ── Health summary bar ── */}
      <div style={{ padding: '10px 16px', borderBottom: '1px solid var(--line)', flexShrink: 0 }}>
        <div
          style={{
            display: 'flex', height: 6, borderRadius: 3,
            overflow: 'hidden', gap: 1.5, marginBottom: 8,
          }}
        >
          {healthOptions.map(([h, c]) => (
            <div
              key={h}
              title={`${c} ${h}`}
              style={{
                width: `${(c / group.count) * 100}%`,
                background: HEALTH_COLOR[h] ?? 'var(--line2)',
                borderRadius: 2,
              }}
            />
          ))}
        </div>
        <div style={{ display: 'flex', gap: 5, flexWrap: 'wrap' }}>
          <button
            onClick={() => setHealthFilter(null)}
            style={{
              padding: '2px 9px', borderRadius: 20, border: '1px solid',
              fontSize: 11, cursor: 'pointer',
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
                padding: '2px 9px', borderRadius: 20, border: '1px solid',
                fontSize: 11, cursor: 'pointer',
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

      {/* ── Search ── */}
      <div style={{ padding: '8px 16px', borderBottom: '1px solid var(--line)', flexShrink: 0 }}>
        <div
          style={{
            display: 'flex', alignItems: 'center', gap: 8,
            background: 'var(--bg)', border: '1px solid var(--line)',
            borderRadius: 8, padding: '6px 10px',
          }}
        >
          <Search size={12} color="var(--ink4)" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filter by name…"
            style={{
              flex: 1, background: 'transparent', border: 'none',
              outline: 'none', fontSize: 12, color: 'var(--ink)', fontFamily: 'inherit',
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
            const hc = HEALTH_COLOR[node.health] ?? '#6b7280'
            const keyMeta = getKeyMeta(node)
            const isSelected = selectedIds.has(node.id)
            return (
              <div
                key={node.id}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  borderBottom: '1px solid var(--surface)',
                  background: isSelected ? 'rgba(239,68,68,0.06)' : 'transparent',
                }}
              >
                {/* Checkbox (bulk-delete types only) */}
                {bulkDeleteFn && (
                  <button
                    onClick={() => toggleSelect(node.id)}
                    style={{
                      flexShrink: 0, width: 36, display: 'flex', alignItems: 'center',
                      justifyContent: 'center', background: 'transparent',
                      border: 'none', cursor: 'pointer',
                      color: isSelected ? '#ef4444' : 'var(--ink4)',
                      padding: '8px 0',
                    }}
                    onMouseEnter={(e) => { e.currentTarget.style.color = isSelected ? '#fca5a5' : 'var(--ink3)' }}
                    onMouseLeave={(e) => { e.currentTarget.style.color = isSelected ? '#ef4444' : 'var(--ink4)' }}
                  >
                    {isSelected ? <CheckSquare size={14} /> : <Square size={14} />}
                  </button>
                )}
                {/* Row button */}
                <button
                  onClick={() => onSelectNode(node.id)}
                  style={{
                    flex: 1, display: 'flex', alignItems: 'center', gap: 10,
                    padding: bulkDeleteFn ? '8px 16px 8px 4px' : '8px 16px',
                    background: 'transparent', border: 'none',
                    cursor: 'pointer', textAlign: 'left',
                  }}
                  onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--surface-2)' }}
                  onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
                >
                  <span
                    style={{
                      width: 7, height: 7, borderRadius: '50%',
                      background: hc, flexShrink: 0,
                    }}
                  />
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <p
                      style={{
                        fontSize: 12, fontWeight: 500, color: 'var(--ink2)',
                        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                      }}
                      title={node.label}
                    >
                      {node.label}
                    </p>
                    {keyMeta && (
                      <p
                        style={{
                          fontSize: 10, color: 'var(--ink4)',
                          fontFamily: MONO,
                          overflow: 'hidden', textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap', marginTop: 1,
                        }}
                      >
                        {keyMeta}
                      </p>
                    )}
                  </div>
                  <span
                    style={{
                      fontSize: 10, padding: '2px 7px', borderRadius: 20,
                      background: HEALTH_BG[node.health] ?? 'rgba(107,114,128,0.1)',
                      color: hc, flexShrink: 0, fontWeight: 500,
                      fontFamily: MONO,
                    }}
                  >
                    {node.health}
                  </span>
                </button>
              </div>
            )
          })
        )}
      </div>

      {/* ── Footer ── */}
      <div style={{ padding: '8px 16px', borderTop: '1px solid var(--line)', flexShrink: 0 }}>
        <p style={{ fontSize: 11, color: 'var(--ink4)', fontFamily: MONO }}>
          Showing {filtered.length} of {group.count}
          {selectedCount > 0 && (
            <span style={{ color: '#ef4444', marginLeft: 8 }}>· {selectedCount} selected</span>
          )}
        </p>
      </div>
    </div>
  )
}
