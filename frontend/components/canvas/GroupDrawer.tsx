'use client'

import { useState, useMemo, useCallback } from 'react'
import { X, Search, Trash2, CheckSquare, Square, AlertTriangle, Loader2 } from 'lucide-react'
import { type GroupInfo } from '@/lib/graphPreprocess'
import { getNodeColor, getNodeIcon, type NodeHealth } from '@/types'
import { sendAction, sendCommand } from '@/lib/wsManager'

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

export default function GroupDrawer({ group, vmCode, initialHealthFilter, onClose, onSelectNode }: GroupDrawerProps) {
  const [query, setQuery] = useState('')
  const [healthFilter, setHealthFilter] = useState<string | null>(initialHealthFilter ?? null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [bulkStatus, setBulkStatus] = useState<'idle' | 'confirming' | 'running' | 'done'>('idle')
  const [deletedCount, setDeletedCount] = useState(0)

  const color = getNodeColor(group.type)
  const icon = getNodeIcon(group.type)
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
      // 200ms stagger — avoid hammering k8s API
      await new Promise((r) => setTimeout(r, 200))
    }
    setBulkStatus('done')
    // Trigger refresh so canvas updates
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
        width: 360,
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
            {degradedCount > 0 && (
              <span style={{ color: '#f59e0b', marginLeft: 6 }}>· {degradedCount} degraded/failed</span>
            )}
          </p>
        </div>
        <button
          onClick={onClose}
          style={{
            width: 28, height: 28, borderRadius: 7, border: 'none',
            background: 'transparent', color: '#475569', cursor: 'pointer',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}
          onMouseEnter={(e) => { e.currentTarget.style.background = '#13131f'; e.currentTarget.style.color = '#94a3b8' }}
          onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = '#475569' }}
        >
          <X size={14} />
        </button>
      </div>

      {/* ── Bulk action bar (job/pod types only) ── */}
      {bulkDeleteFn && (
        <div style={{
          padding: '8px 12px',
          borderBottom: '1px solid #1e1e3a',
          flexShrink: 0,
          background: selectedCount > 0 ? 'rgba(239,68,68,0.04)' : '#08080e',
        }}>
          {bulkStatus === 'done' ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: '#10b981' }}>
              <span>✓ Deleted {deletedCount} {group.type}s — refreshing…</span>
            </div>
          ) : bulkStatus === 'running' ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: '#94a3b8' }}>
              <Loader2 size={13} style={{ animation: 'spin 1s linear infinite' }} />
              <span>Deleting {deletedCount}/{selectedCount}…</span>
            </div>
          ) : bulkStatus === 'confirming' ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12 }}>
                <AlertTriangle size={13} color="#f59e0b" />
                <span style={{ color: '#fbbf24' }}>
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
                    border: '1px solid #1e1e3a', background: 'transparent',
                    color: '#475569', fontSize: 12, cursor: 'pointer',
                  }}
                >
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
              {/* Select shortcuts */}
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
                    border: '1px solid #1e1e3a',
                    background: 'transparent',
                    color: '#475569', fontSize: 11, cursor: 'pointer',
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
                      border: '1px solid #1e1e3a', background: 'transparent',
                      color: '#475569', fontSize: 11, cursor: 'pointer',
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
                <span style={{ fontSize: 11, color: '#334155' }}>Select items to perform bulk actions</span>
              )}
            </div>
          )}
        </div>
      )}

      {/* ── Health summary bar ── */}
      <div style={{ padding: '10px 16px', borderBottom: '1px solid #1e1e3a', flexShrink: 0 }}>
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
                background: HEALTH_COLOR[h] ?? '#334155',
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
                padding: '2px 9px', borderRadius: 20, border: '1px solid',
                fontSize: 11, cursor: 'pointer',
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
            display: 'flex', alignItems: 'center', gap: 8,
            background: '#070711', border: '1px solid #1e1e3a',
            borderRadius: 8, padding: '6px 10px',
          }}
        >
          <Search size={12} color="#334155" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filter by name…"
            style={{
              flex: 1, background: 'transparent', border: 'none',
              outline: 'none', fontSize: 12, color: '#e2e8f0',
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
            const isSelected = selectedIds.has(node.id)
            return (
              <div
                key={node.id}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  borderBottom: '1px solid #0f0f1e',
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
                      color: isSelected ? '#ef4444' : '#334155',
                      padding: '8px 0',
                    }}
                    onMouseEnter={(e) => { e.currentTarget.style.color = isSelected ? '#fca5a5' : '#475569' }}
                    onMouseLeave={(e) => { e.currentTarget.style.color = isSelected ? '#ef4444' : '#334155' }}
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
                  onMouseEnter={(e) => { e.currentTarget.style.background = '#0f0f20' }}
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
                        fontSize: 12, fontWeight: 500, color: '#cbd5e1',
                        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                      }}
                      title={node.label}
                    >
                      {node.label}
                    </p>
                    {keyMeta && (
                      <p
                        style={{
                          fontSize: 10, color: '#334155',
                          fontFamily: 'JetBrains Mono, monospace',
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
                      background: HEALTH_BG[node.health] ?? 'rgba(71,85,105,0.1)',
                      color: hc, flexShrink: 0, fontWeight: 500,
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
      <div style={{ padding: '8px 16px', borderTop: '1px solid #1e1e3a', flexShrink: 0 }}>
        <p style={{ fontSize: 11, color: '#334155' }}>
          Showing {filtered.length} of {group.count}
          {selectedCount > 0 && (
            <span style={{ color: '#ef4444', marginLeft: 8 }}>· {selectedCount} selected</span>
          )}
        </p>
      </div>
    </div>
  )
}
