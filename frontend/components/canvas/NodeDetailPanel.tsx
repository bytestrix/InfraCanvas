'use client'

import { useState, useEffect, useRef } from 'react'
import {
  X, RotateCw, Square, Play, Tag, Layers, Trash2,
  CheckCircle2, XCircle, Loader2, ChevronDown, ChevronRight,
  AlertTriangle, type LucideIcon,
} from 'lucide-react'
import { type GraphNode, getNodeColor, getNodeIcon } from '@/types'
import { sendAction, subscribeActionResult, subscribeActionProgress } from '@/lib/wsManager'

// ─── Types ────────────────────────────────────────────────────────────────────

interface FormField {
  key: string
  label: string
  placeholder?: string
  type?: 'text' | 'number'
  defaultValue?: (node: GraphNode) => string
}

interface ActionDef {
  id: string
  label: string
  Icon: LucideIcon
  color: string
  danger?: boolean
  confirm?: boolean           // direct confirm — no fields
  form?: FormField[]          // show input fields
  buildPayload: (node: GraphNode, vals: Record<string, string>) => object
}

// ─── Action registry by node type ────────────────────────────────────────────

const ACTIONS: Record<string, ActionDef[]> = {
  container: [
    {
      id: 'restart', label: 'Restart', Icon: RotateCw, color: '#6366f1', confirm: true,
      buildPayload: (n) => ({ action_id: `restart-${Date.now()}`, type: 'docker_restart_container', target: { layer: 'docker', entity_type: 'container', entity_id: n.id }, parameters: {} }),
    },
    {
      id: 'stop', label: 'Stop', Icon: Square, color: '#f59e0b', confirm: true,
      buildPayload: (n) => ({ action_id: `stop-${Date.now()}`, type: 'docker_stop_container', target: { layer: 'docker', entity_type: 'container', entity_id: n.id }, parameters: {} }),
    },
    {
      id: 'start', label: 'Start', Icon: Play, color: '#10b981', confirm: true,
      buildPayload: (n) => ({ action_id: `start-${Date.now()}`, type: 'docker_start_container', target: { layer: 'docker', entity_type: 'container', entity_id: n.id }, parameters: {} }),
    },
    {
      id: 'update_image', label: 'Update Image', Icon: Tag, color: '#8b5cf6',
      form: [
        { key: 'image', label: 'New Image:Tag', placeholder: 'nginx:1.25', defaultValue: (n) => n.metadata.image ?? '' },
      ],
      buildPayload: (n, v) => ({ action_id: `update-img-${Date.now()}`, type: 'docker_update_container_image', target: { layer: 'docker', entity_type: 'container', entity_id: n.id }, parameters: { image: v.image } }),
    },
  ],

  image: [
    {
      id: 'change_tag', label: 'Change Tag', Icon: Tag, color: '#6366f1',
      form: [
        {
          key: 'image',
          label: 'New Image:Tag',
          placeholder: 'registry/name:newtag',
          defaultValue: (n) =>
            n.metadata.fullName ??
            (n.metadata.repository && n.metadata.tag
              ? `${n.metadata.repository}:${n.metadata.tag}`
              : n.label),
        },
      ],
      buildPayload: (n, v) => ({ action_id: `pull-${Date.now()}`, type: 'docker_pull_image', target: { layer: 'docker', entity_type: 'image', entity_id: n.id }, parameters: { image: v.image } }),
    },
  ],

  deployment: [
    {
      id: 'scale', label: 'Scale', Icon: Layers, color: '#10b981',
      form: [
        { key: 'replicas', label: 'Replicas', placeholder: '3', type: 'number', defaultValue: (n) => String(n.metadata.replicas ?? n.metadata.ready_replicas ?? '1') },
      ],
      buildPayload: (n, v) => ({ action_id: `scale-${Date.now()}`, type: 'k8s_scale_deployment', target: { layer: 'kubernetes', entity_type: 'deployment', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: { replicas: parseInt(v.replicas, 10) } }),
    },
    {
      id: 'restart', label: 'Rolling Restart', Icon: RotateCw, color: '#6366f1', confirm: true,
      buildPayload: (n) => ({ action_id: `restart-${Date.now()}`, type: 'k8s_restart_deployment', target: { layer: 'kubernetes', entity_type: 'deployment', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: {} }),
    },
    {
      id: 'update_image', label: 'Update Image', Icon: Tag, color: '#8b5cf6',
      form: [
        { key: 'container', label: 'Container (optional)', placeholder: 'leave blank for first', defaultValue: () => '' },
        { key: 'image', label: 'New Image:Tag', placeholder: 'registry/name:v2.0', defaultValue: () => '' },
      ],
      buildPayload: (n, v) => ({ action_id: `upd-img-${Date.now()}`, type: 'k8s_update_image', target: { layer: 'kubernetes', entity_type: 'deployment', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: { image: v.image, container: v.container } }),
    },
  ],

  statefulset: [
    {
      id: 'scale', label: 'Scale', Icon: Layers, color: '#10b981',
      form: [
        { key: 'replicas', label: 'Replicas', placeholder: '3', type: 'number', defaultValue: (n) => String(n.metadata.replicas ?? '1') },
      ],
      buildPayload: (n, v) => ({ action_id: `scale-${Date.now()}`, type: 'k8s_scale_statefulset', target: { layer: 'kubernetes', entity_type: 'statefulset', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: { replicas: parseInt(v.replicas, 10) } }),
    },
    {
      id: 'restart', label: 'Rolling Restart', Icon: RotateCw, color: '#6366f1', confirm: true,
      buildPayload: (n) => ({ action_id: `restart-${Date.now()}`, type: 'k8s_restart_statefulset', target: { layer: 'kubernetes', entity_type: 'statefulset', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: {} }),
    },
  ],

  daemonset: [
    {
      id: 'restart', label: 'Rolling Restart', Icon: RotateCw, color: '#6366f1', confirm: true,
      buildPayload: (n) => ({ action_id: `restart-${Date.now()}`, type: 'k8s_restart_daemonset', target: { layer: 'kubernetes', entity_type: 'daemonset', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: {} }),
    },
  ],

  pod: [
    {
      id: 'delete', label: 'Delete / Restart', Icon: Trash2, color: '#ef4444', danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `del-${Date.now()}`, type: 'k8s_delete_pod', target: { layer: 'kubernetes', entity_type: 'pod', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: {} }),
    },
  ],

  node: [
    {
      id: 'cordon', label: 'Cordon', Icon: Square, color: '#f59e0b', confirm: true,
      buildPayload: (n) => ({ action_id: `cordon-${Date.now()}`, type: 'k8s_cordon_node', target: { layer: 'kubernetes', entity_type: 'node', entity_id: n.metadata.name ?? n.id }, parameters: {} }),
    },
    {
      id: 'drain', label: 'Drain', Icon: Layers, color: '#ef4444', danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `drain-${Date.now()}`, type: 'k8s_drain_node', target: { layer: 'kubernetes', entity_type: 'node', entity_id: n.metadata.name ?? n.id }, parameters: { ignore_daemonsets: 'true', delete_emptydir_data: 'true' } }),
    },
  ],

  job: [
    {
      id: 'delete', label: 'Delete Job', Icon: Trash2, color: '#ef4444', danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `del-${Date.now()}`, type: 'k8s_delete_job', target: { layer: 'kubernetes', entity_type: 'job', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: {} }),
    },
  ],
}

// ─── Key metadata fields shown prominently per node type ──────────────────────

const KEY_META_FIELDS: Record<string, string[]> = {
  container:  ['state', 'image', 'ports'],
  image:      ['tag', 'size', 'created'],
  deployment: ['namespace', 'replicas', 'ready_replicas', 'strategy'],
  statefulset:['namespace', 'replicas', 'ready_replicas'],
  daemonset:  ['namespace', 'desired', 'ready'],
  pod:        ['namespace', 'node', 'phase', 'ip'],
  node:       ['roles', 'status', 'kernel_version', 'os_image'],
  host:       ['os', 'kernel', 'cpu_cores', 'memory_total'],
  k8s_service:['namespace', 'type', 'cluster_ip', 'ports'],
  ingress:    ['namespace', 'host', 'tls'],
  pvc:        ['namespace', 'storage_class', 'capacity', 'access_modes'],
  volume:     ['driver', 'mountpoint'],
  network:    ['driver', 'scope', 'subnet'],
  cluster:    ['version', 'node_count'],
  job:        ['namespace', 'completions', 'active', 'succeeded'],
  cronjob:    ['namespace', 'schedule', 'last_run'],
}

const HEALTH_COLOR: Record<string, string> = {
  healthy: '#10b981', degraded: '#f59e0b', unhealthy: '#ef4444', unknown: '#64748b',
}

// ─── Component ────────────────────────────────────────────────────────────────

interface NodeDetailPanelProps {
  node: GraphNode
  vmCode: string
  onClose: () => void
}

type ActionStatus = 'idle' | 'confirming' | 'running' | 'success' | 'error'

export default function NodeDetailPanel({ node, vmCode, onClose }: NodeDetailPanelProps) {
  const color = getNodeColor(node.type, node.health)
  const icon = getNodeIcon(node.type)
  const hc = HEALTH_COLOR[node.health] ?? '#64748b'
  const actions = ACTIONS[node.type] ?? []

  const [activeActionId, setActiveActionId] = useState<string | null>(null)
  const [formValues, setFormValues] = useState<Record<string, string>>({})
  const [actionStatus, setActionStatus] = useState<ActionStatus>('idle')
  const [actionMsg, setActionMsg] = useState('')
  const [metaOpen, setMetaOpen] = useState(true)
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Reset action state when node changes
  useEffect(() => {
    setActiveActionId(null)
    setActionStatus('idle')
    setActionMsg('')
    setFormValues({})
  }, [node.id])

  // Subscribe to action results
  useEffect(() => {
    const unsubResult = subscribeActionResult((data) => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current)
      if (data.success === false || data.status === 'failed') {
        setActionStatus('error')
        setActionMsg(data.message ?? data.error ?? 'Action failed')
      } else {
        setActionStatus('success')
        setActionMsg(data.message ?? 'Done')
      }
    })
    const unsubProgress = subscribeActionProgress((data) => {
      setActionMsg(data.message ?? `${data.progress ?? 0}%`)
    })
    return () => { unsubResult(); unsubProgress() }
  }, [])

  function openAction(action: ActionDef) {
    if (activeActionId === action.id) {
      setActiveActionId(null)
      setActionStatus('idle')
      setActionMsg('')
      return
    }
    // Pre-fill form with defaults from node metadata
    const defaults: Record<string, string> = {}
    for (const f of action.form ?? []) {
      defaults[f.key] = f.defaultValue ? f.defaultValue(node) : ''
    }
    setFormValues(defaults)
    setActiveActionId(action.id)
    setActionStatus(action.confirm ? 'confirming' : 'idle')
    setActionMsg('')
  }

  function handleSubmit(action: ActionDef) {
    setActionStatus('running')
    setActionMsg('Sending…')

    const payload = action.buildPayload(node, formValues)
    sendAction(vmCode, payload)

    // Timeout fallback if agent doesn't respond
    timeoutRef.current = setTimeout(() => {
      if (actionStatus === 'running') {
        setActionStatus('error')
        setActionMsg('No response from agent — check agent logs')
      }
    }, 20_000)
  }

  // Key metadata: show priority fields prominently, rest behind toggle
  const keyFields = KEY_META_FIELDS[node.type] ?? []
  const keyMeta = keyFields
    .filter((k) => node.metadata[k] != null && node.metadata[k] !== '')
    .map((k) => [k, node.metadata[k]] as [string, any])
  const allMetaEntries = Object.entries(node.metadata)
  const extraMeta = allMetaEntries.filter(([k]) => !keyFields.includes(k))

  return (
    <div style={{
      position: 'absolute', right: 0, top: 0, bottom: 0, width: 340,
      background: '#0a0a16', borderLeft: '1px solid #1e1e3a',
      display: 'flex', flexDirection: 'column', zIndex: 30,
      boxShadow: '-8px 0 32px rgba(0,0,0,0.5)',
    }}>

      {/* ── Header ── */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', padding: '12px 16px', borderBottom: '1px solid #1e1e3a', flexShrink: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
          <div style={{ width: 34, height: 34, borderRadius: 8, background: `${color}18`, border: `1px solid ${color}28`, color, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 15, flexShrink: 0 }}>{icon}</div>
          <div style={{ minWidth: 0 }}>
            <p style={{ fontSize: 13, fontWeight: 600, color: '#e2e8f0', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={node.label}>{node.label}</p>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 3 }}>
              <span style={{ fontSize: 10, padding: '1px 6px', borderRadius: 4, background: `${color}18`, color, border: `1px solid ${color}28`, fontFamily: 'monospace' }}>{node.type}</span>
              <span style={{ fontSize: 10, color: hc, display: 'flex', alignItems: 'center', gap: 3 }}>
                <span style={{ width: 5, height: 5, borderRadius: '50%', background: hc, display: 'inline-block' }} />{node.health}
              </span>
            </div>
          </div>
        </div>
        <button onClick={onClose} style={ICON_BTN}
          onMouseEnter={(e) => { e.currentTarget.style.background = '#13131f'; e.currentTarget.style.color = '#94a3b8' }}
          onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = '#475569' }}>
          <X size={14} />
        </button>
      </div>

      {/* ── Scrollable body ── */}
      <div style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column' }}>

        {/* ── Key metadata ── */}
        {keyMeta.length > 0 && (
          <div style={{ padding: '10px 16px', borderBottom: '1px solid #0f0f1e' }}>
            {keyMeta.map(([k, v]) => {
              const str = typeof v === 'object' ? JSON.stringify(v) : String(v ?? '')
              return (
                <div key={k} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 8, marginBottom: 5 }}>
                  <span style={{ fontSize: 10, color: '#475569', flexShrink: 0 }}>{k}</span>
                  <span style={{ fontSize: 11, fontFamily: 'JetBrains Mono, monospace', color: '#94a3b8', textAlign: 'right', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 190 }} title={str}>{str}</span>
                </div>
              )
            })}
          </div>
        )}

        {/* ── Actions ── */}
        {actions.length > 0 && (
          <div style={{ padding: '12px 16px', borderBottom: '1px solid #1e1e3a' }}>
            <p style={{ fontSize: 10, fontWeight: 600, color: '#334155', letterSpacing: '0.08em', textTransform: 'uppercase', marginBottom: 10 }}>Actions</p>

            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: activeActionId ? 12 : 0 }}>
              {actions.map((action) => {
                const isOpen = activeActionId === action.id
                return (
                  <button
                    key={action.id}
                    onClick={() => openAction(action)}
                    style={{
                      display: 'flex', alignItems: 'center', gap: 5,
                      padding: '5px 11px', borderRadius: 6, fontSize: 11, fontWeight: 500,
                      border: `1px solid ${isOpen ? action.color : '#1e1e3a'}`,
                      background: isOpen ? `${action.color}18` : '#070711',
                      color: isOpen ? action.color : '#64748b',
                      cursor: 'pointer', transition: 'all 0.15s',
                    }}
                    onMouseEnter={(e) => {
                      if (!isOpen) {
                        e.currentTarget.style.borderColor = action.color
                        e.currentTarget.style.color = action.color
                        e.currentTarget.style.background = `${action.color}10`
                      }
                    }}
                    onMouseLeave={(e) => {
                      if (!isOpen) {
                        e.currentTarget.style.borderColor = '#1e1e3a'
                        e.currentTarget.style.color = '#64748b'
                        e.currentTarget.style.background = '#070711'
                      }
                    }}
                  >
                    <action.Icon size={11} />
                    {action.label}
                    {isOpen
                      ? <ChevronDown size={10} style={{ opacity: 0.5 }} />
                      : <ChevronRight size={10} style={{ opacity: 0.3 }} />}
                  </button>
                )
              })}
            </div>

            {/* ── Active action form ── */}
            {activeActionId && (() => {
              const action = actions.find((a) => a.id === activeActionId)!
              return (
                <div style={{ background: '#070711', border: '1px solid #1e1e3a', borderRadius: 8, padding: 12 }}>

                  {/* Form fields */}
                  {action.form && actionStatus === 'idle' && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 12 }}>
                      {action.form.map((field) => (
                        <div key={field.key}>
                          <label style={{ fontSize: 10, color: '#475569', display: 'block', marginBottom: 4 }}>{field.label}</label>
                          <input
                            type={field.type ?? 'text'}
                            value={formValues[field.key] ?? ''}
                            onChange={(e) => setFormValues((prev) => ({ ...prev, [field.key]: e.target.value }))}
                            placeholder={field.placeholder}
                            style={{
                              width: '100%', boxSizing: 'border-box',
                              background: '#0a0a16', border: '1px solid #1e1e3a',
                              borderRadius: 6, padding: '6px 10px',
                              fontSize: 12, fontFamily: 'JetBrains Mono, monospace',
                              color: '#e2e8f0', outline: 'none',
                            }}
                            onFocus={(e) => { e.currentTarget.style.borderColor = action.color }}
                            onBlur={(e) => { e.currentTarget.style.borderColor = '#1e1e3a' }}
                          />
                        </div>
                      ))}
                    </div>
                  )}

                  {/* Confirm warning for dangerous / no-form actions */}
                  {action.confirm && actionStatus === 'confirming' && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 10 }}>
                      {action.danger && <AlertTriangle size={12} color="#f59e0b" />}
                      <span style={{ fontSize: 11, color: action.danger ? '#fcd34d' : '#94a3b8' }}>
                        {action.danger ? 'This is a destructive action. Confirm?' : `Run "${action.label}" on ${node.label}?`}
                      </span>
                    </div>
                  )}

                  {/* Running */}
                  {actionStatus === 'running' && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                      <Loader2 size={13} color="#6366f1" style={{ animation: 'spin 1s linear infinite' }} />
                      <span style={{ fontSize: 11, color: '#64748b' }}>{actionMsg}</span>
                    </div>
                  )}

                  {/* Success */}
                  {actionStatus === 'success' && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                      <CheckCircle2 size={13} color="#10b981" />
                      <span style={{ fontSize: 11, color: '#6ee7b7' }}>{actionMsg}</span>
                    </div>
                  )}

                  {/* Error */}
                  {actionStatus === 'error' && (
                    <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8, marginBottom: 10 }}>
                      <XCircle size={13} color="#ef4444" style={{ flexShrink: 0, marginTop: 1 }} />
                      <span style={{ fontSize: 11, color: '#fca5a5', wordBreak: 'break-word' }}>{actionMsg}</span>
                    </div>
                  )}

                  {/* Buttons */}
                  {(actionStatus === 'idle' || actionStatus === 'confirming') && (
                    <div style={{ display: 'flex', gap: 6 }}>
                      <button
                        onClick={() => handleSubmit(action)}
                        style={{
                          flex: 1, padding: '6px 10px', borderRadius: 6,
                          background: action.danger ? '#ef444420' : `${action.color}20`,
                          color: action.danger ? '#ef4444' : action.color,
                          fontSize: 11, fontWeight: 600, cursor: 'pointer',
                          border: `1px solid ${action.danger ? '#ef444440' : `${action.color}40`}`,
                        } as React.CSSProperties}
                        onMouseEnter={(e) => { e.currentTarget.style.opacity = '0.8' }}
                        onMouseLeave={(e) => { e.currentTarget.style.opacity = '1' }}
                      >
                        {action.confirm ? `Confirm ${action.label}` : `Apply ${action.label}`}
                      </button>
                      <button
                        onClick={() => { setActiveActionId(null); setActionStatus('idle') }}
                        style={{ padding: '6px 10px', borderRadius: 6, border: '1px solid #1e1e3a', background: 'transparent', color: '#475569', fontSize: 11, cursor: 'pointer' }}
                      >
                        Cancel
                      </button>
                    </div>
                  )}

                  {/* Reset after done */}
                  {(actionStatus === 'success' || actionStatus === 'error') && (
                    <button
                      onClick={() => { setActiveActionId(null); setActionStatus('idle'); setActionMsg('') }}
                      style={{ width: '100%', padding: '6px', borderRadius: 6, border: '1px solid #1e1e3a', background: 'transparent', color: '#475569', fontSize: 11, cursor: 'pointer', marginTop: 4 }}
                    >
                      Dismiss
                    </button>
                  )}
                </div>
              )
            })()}
          </div>
        )}

        {/* ── Full Metadata (collapsible) ── */}
        <div style={{ padding: '0 0 8px' }}>
          <button
            onClick={() => setMetaOpen((v) => !v)}
            style={{ width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px 16px', background: 'transparent', border: 'none', borderBottom: metaOpen ? '1px solid #0f0f1e' : 'none', cursor: 'pointer', color: '#334155' }}
            onMouseEnter={(e) => { e.currentTarget.style.background = '#0c0c18' }}
            onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
          >
            <span style={{ fontSize: 10, fontWeight: 600, letterSpacing: '0.08em', textTransform: 'uppercase' }}>
              All Metadata ({allMetaEntries.length})
            </span>
            {metaOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
          </button>

          {metaOpen && (
            <div style={{ padding: '8px 16px' }}>
              {/* ID */}
              <div style={{ marginBottom: 8 }}>
                <p style={{ fontSize: 10, color: '#334155', marginBottom: 2 }}>id</p>
                <p style={{ fontSize: 10, fontFamily: 'JetBrains Mono, monospace', color: '#475569', wordBreak: 'break-all' }}>{node.id}</p>
              </div>
              {allMetaEntries.length === 0
                ? <p style={{ fontSize: 12, color: '#334155' }}>No metadata</p>
                : allMetaEntries.map(([k, v]) => {
                  const str = typeof v === 'object' ? JSON.stringify(v, null, 2) : String(v ?? '')
                  const isLong = str.length > 38 || str.includes('\n')
                  return (
                    <div key={k} style={{ marginBottom: 8 }}>
                      <p style={{ fontSize: 10, color: '#334155', marginBottom: 2 }}>{k}</p>
                      {isLong
                        ? <pre style={{ fontSize: 10, background: '#070711', color: '#64748b', border: '1px solid #1e1e3a', borderRadius: 5, padding: '5px 7px', fontFamily: 'JetBrains Mono, monospace', whiteSpace: 'pre-wrap', wordBreak: 'break-all', margin: 0 }}>{str}</pre>
                        : <p style={{ fontSize: 11, fontFamily: 'JetBrains Mono, monospace', color: '#94a3b8' }}>{str}</p>
                      }
                    </div>
                  )
                })
              }
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ─── Micro-styles ─────────────────────────────────────────────────────────────

const ICON_BTN: React.CSSProperties = {
  width: 28, height: 28, borderRadius: 7, border: 'none',
  background: 'transparent', color: '#475569', cursor: 'pointer',
  display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0,
}
