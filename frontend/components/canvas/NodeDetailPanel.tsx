'use client'

import { useState, useEffect, useRef } from 'react'
import {
  X, RotateCw, Square, Play, Tag, Layers, Trash2,
  CheckCircle2, XCircle, Loader2, ChevronDown, ChevronRight,
  AlertTriangle, FileText, Terminal, Eye, EyeOff,
  type LucideIcon,
} from 'lucide-react'
import { type GraphNode, getNodeColor, getNodeIcon } from '@/types'
import { sendAction, sendCommand, subscribeActionResult, subscribeActionProgress } from '@/lib/wsManager'

// ─── Types ────────────────────────────────────────────────────────────────────

interface FormFieldOption {
  value: string
  label: string
  // Optional payload bag that other fields can read when this option is picked
  // (e.g. selecting a container should pre-fill the image field with its current image).
  prefill?: Record<string, string>
}

interface FormField {
  key: string
  label: string
  placeholder?: string
  type?: 'text' | 'number' | 'select'
  defaultValue?: (node: GraphNode) => string
  // For 'select' fields. Returning an empty list falls back to a text input so
  // the action stays usable even when container metadata is missing.
  options?: (node: GraphNode) => FormFieldOption[]
}

interface ActionDef {
  id: string
  label: string
  Icon: LucideIcon
  color: string
  danger?: boolean
  confirm?: boolean
  form?: FormField[]
  buildPayload: (node: GraphNode, vals: Record<string, string>) => object
}

// ─── Action registry ─────────────────────────────────────────────────────────

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
        { key: 'image', label: 'New Image:Tag', placeholder: 'registry/name:newtag', defaultValue: (n) => n.metadata.repository && n.metadata.tag ? `${n.metadata.repository}:${n.metadata.tag}` : n.label },
      ],
      buildPayload: (n, v) => ({ action_id: `pull-${Date.now()}`, type: 'docker_pull_image', target: { layer: 'docker', entity_type: 'image', entity_id: n.id }, parameters: { image: v.image } }),
    },
  ],
  deployment: [
    { id: 'scale', label: 'Scale', Icon: Layers, color: '#10b981', form: [{ key: 'replicas', label: 'Replicas', placeholder: '3', type: 'number', defaultValue: (n) => String(n.metadata.replicas ?? '1') }], buildPayload: (n, v) => ({ action_id: `scale-${Date.now()}`, type: 'k8s_scale_deployment', target: { layer: 'kubernetes', entity_type: 'deployment', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: { replicas: parseInt(v.replicas, 10) } }) },
    { id: 'restart', label: 'Rolling Restart', Icon: RotateCw, color: '#6366f1', confirm: true, buildPayload: (n) => ({ action_id: `restart-${Date.now()}`, type: 'k8s_restart_deployment', target: { layer: 'kubernetes', entity_type: 'deployment', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: {} }) },
    {
      id: 'update_image', label: 'Update Image', Icon: Tag, color: '#8b5cf6',
      form: [
        {
          key: 'container', label: 'Container', type: 'select',
          defaultValue: (n) => (n.metadata.containers?.[0]?.name ?? ''),
          options: (n) => (n.metadata.containers ?? []).map((c: any) => ({
            value: c.name,
            label: `${c.name}  —  ${c.image}`,
            prefill: { image: c.image },
          })),
        },
        {
          key: 'image', label: 'New Image:Tag', placeholder: 'registry/name:v2.0',
          defaultValue: (n) => (n.metadata.containers?.[0]?.image ?? ''),
        },
      ],
      buildPayload: (n, v) => ({ action_id: `upd-img-${Date.now()}`, type: 'k8s_update_image', target: { layer: 'kubernetes', entity_type: 'deployment', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: { image: v.image, container: v.container } }),
    },
  ],
  statefulset: [
    { id: 'scale', label: 'Scale', Icon: Layers, color: '#10b981', form: [{ key: 'replicas', label: 'Replicas', placeholder: '3', type: 'number', defaultValue: (n) => String(n.metadata.replicas ?? '1') }], buildPayload: (n, v) => ({ action_id: `scale-${Date.now()}`, type: 'k8s_scale_statefulset', target: { layer: 'kubernetes', entity_type: 'statefulset', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: { replicas: parseInt(v.replicas, 10) } }) },
    { id: 'restart', label: 'Rolling Restart', Icon: RotateCw, color: '#6366f1', confirm: true, buildPayload: (n) => ({ action_id: `restart-${Date.now()}`, type: 'k8s_restart_statefulset', target: { layer: 'kubernetes', entity_type: 'statefulset', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: {} }) },
  ],
  daemonset: [
    { id: 'restart', label: 'Rolling Restart', Icon: RotateCw, color: '#6366f1', confirm: true, buildPayload: (n) => ({ action_id: `restart-${Date.now()}`, type: 'k8s_restart_daemonset', target: { layer: 'kubernetes', entity_type: 'daemonset', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: {} }) },
  ],
  pod: [
    { id: 'delete', label: 'Delete / Restart', Icon: Trash2, color: '#ef4444', danger: true, confirm: true, buildPayload: (n) => ({ action_id: `del-${Date.now()}`, type: 'k8s_delete_pod', target: { layer: 'kubernetes', entity_type: 'pod', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: {} }) },
  ],
  node: [
    { id: 'cordon', label: 'Cordon', Icon: Square, color: '#f59e0b', confirm: true, buildPayload: (n) => ({ action_id: `cordon-${Date.now()}`, type: 'k8s_cordon_node', target: { layer: 'kubernetes', entity_type: 'node', entity_id: n.metadata.name ?? n.id }, parameters: {} }) },
    { id: 'drain', label: 'Drain', Icon: Layers, color: '#ef4444', danger: true, confirm: true, buildPayload: (n) => ({ action_id: `drain-${Date.now()}`, type: 'k8s_drain_node', target: { layer: 'kubernetes', entity_type: 'node', entity_id: n.metadata.name ?? n.id }, parameters: { ignore_daemonsets: 'true', delete_emptydir_data: 'true' } }) },
  ],
  job: [
    { id: 'delete', label: 'Delete Job', Icon: Trash2, color: '#ef4444', danger: true, confirm: true, buildPayload: (n) => ({ action_id: `del-${Date.now()}`, type: 'k8s_delete_job', target: { layer: 'kubernetes', entity_type: 'job', entity_id: n.metadata.name ?? n.id, namespace: n.metadata.namespace ?? 'default' }, parameters: {} }) },
  ],
}

const KEY_META_FIELDS: Record<string, string[]> = {
  container:   ['state', 'image', 'restart_count'],
  image:       ['tag', 'registry', 'size'],
  deployment:  ['namespace', 'replicas', 'ready_replicas', 'updated_replicas', 'strategy', 'image', 'service_account', 'helm_release', 'chart_version'],
  statefulset: ['namespace', 'replicas', 'ready_replicas', 'service_name', 'image'],
  daemonset:   ['namespace', 'desired', 'ready', 'image'],
  pod:         ['namespace', 'node', 'phase', 'ip'],
  node:        ['roles', 'status', 'kernel_version', 'os_image'],
  host:        ['os', 'kernel', 'cpu_cores', 'memory_total'],
  k8s_service: ['namespace', 'type', 'cluster_ip', 'ports'],
  ingress:     ['namespace', 'host', 'tls'],
  pvc:         ['namespace', 'storage_class', 'capacity', 'access_modes'],
  volume:      ['driver', 'mountpoint'],
  network:     ['driver', 'scope', 'subnet'],
  cluster:     ['version', 'node_count'],
  job:         ['namespace', 'completions', 'active', 'succeeded'],
  cronjob:     ['namespace', 'schedule', 'last_run'],
}

const HEALTH_COLOR: Record<string, string> = {
  healthy: '#10b981', degraded: '#f59e0b', unhealthy: '#ef4444', unknown: '#64748b',
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (!bytes) return '—'
  if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(1)} GB`
  if (bytes >= 1e6) return `${(bytes / 1e6).toFixed(1)} MB`
  if (bytes >= 1e3) return `${(bytes / 1e3).toFixed(1)} KB`
  return `${bytes} B`
}

const SECRET_PATTERN = /password|secret|token|key|auth|credential|api_key|passwd|private/i

// ─── Sub-panels ───────────────────────────────────────────────────────────────

function SectionHeader({ title, count, open, onToggle }: { title: string; count?: number; open: boolean; onToggle: () => void }) {
  return (
    <button
      onClick={onToggle}
      style={{ width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '9px 16px', background: 'transparent', border: 'none', borderBottom: open ? '1px solid #0f0f1e' : 'none', cursor: 'pointer', color: '#334155' }}
      onMouseEnter={(e) => { e.currentTarget.style.background = '#0c0c18' }}
      onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <span style={{ fontSize: 10, fontWeight: 600, letterSpacing: '0.08em', textTransform: 'uppercase' as const }}>{title}</span>
        {count != null && <span style={{ fontSize: 9, padding: '1px 5px', borderRadius: 10, background: '#1e1e3a', color: '#475569' }}>{count}</span>}
      </div>
      {open ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
    </button>
  )
}

function EnvVarsPanel({ env }: { env: Record<string, string> }) {
  const [showSecrets, setShowSecrets] = useState(false)
  const entries = Object.entries(env)
  if (entries.length === 0) return <p style={{ fontSize: 11, color: '#334155', padding: '8px 16px' }}>No environment variables</p>

  return (
    <div style={{ padding: '6px 12px 10px' }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 6 }}>
        <button
          onClick={() => setShowSecrets(v => !v)}
          style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 10, color: '#475569', background: 'transparent', border: '1px solid #1e1e3a', borderRadius: 4, padding: '2px 7px', cursor: 'pointer' }}
        >
          {showSecrets ? <EyeOff size={9} /> : <Eye size={9} />}
          {showSecrets ? 'Hide secrets' : 'Show secrets'}
        </button>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {entries.map(([k, v]) => {
          const isSecret = SECRET_PATTERN.test(k) || v === '[REDACTED]'
          const display = isSecret && !showSecrets ? '••••••••' : v
          return (
            <div key={k} style={{ display: 'flex', background: '#07070f', borderRadius: 4, overflow: 'hidden', marginBottom: 2 }}>
              <span style={{ fontSize: 10, fontFamily: 'JetBrains Mono, monospace', color: '#6366f1', padding: '3px 6px', background: '#0d0d1e', flexShrink: 0, maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={k}>{k}</span>
              <span style={{ fontSize: 10, fontFamily: 'JetBrains Mono, monospace', color: isSecret && !showSecrets ? '#334155' : '#94a3b8', padding: '3px 6px', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={display}>{display}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function PortsPanel({ ports }: { ports: any[] }) {
  if (!ports || ports.length === 0) return <p style={{ fontSize: 11, color: '#334155', padding: '8px 16px' }}>No port mappings</p>
  return (
    <div style={{ padding: '8px 12px 10px' }}>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr auto 1fr', gap: '3px 8px', alignItems: 'center' }}>
        <span style={{ fontSize: 9, color: '#334155', fontWeight: 600 }}>HOST</span>
        <span />
        <span style={{ fontSize: 9, color: '#334155', fontWeight: 600 }}>CONTAINER</span>
        {ports.map((p, i) => (
          <>
            <span key={`h${i}`} style={{ fontSize: 11, fontFamily: 'JetBrains Mono, monospace', color: '#34d399', background: '#07140e', borderRadius: 4, padding: '2px 6px', textAlign: 'right' }}>
              {p.hostIP && p.hostIP !== '0.0.0.0' ? `${p.hostIP}:` : ''}{p.hostPort}
            </span>
            <span key={`a${i}`} style={{ fontSize: 9, color: '#475569', textAlign: 'center' }}>→</span>
            <span key={`c${i}`} style={{ fontSize: 11, fontFamily: 'JetBrains Mono, monospace', color: '#89b4fa', background: '#070d14', borderRadius: 4, padding: '2px 6px' }}>
              {p.containerPort}<span style={{ color: '#334155' }}>/{p.protocol}</span>
            </span>
          </>
        ))}
      </div>
    </div>
  )
}

function MountsPanel({ mounts }: { mounts: Array<{ source: string; destination: string; mode: string; type: string }> }) {
  if (!mounts || mounts.length === 0) return <p style={{ fontSize: 11, color: '#334155', padding: '8px 16px' }}>No mounts</p>
  const typeColor: Record<string, string> = { volume: '#a78bfa', bind: '#f59e0b', tmpfs: '#06b6d4' }
  return (
    <div style={{ padding: '8px 12px 10px', display: 'flex', flexDirection: 'column', gap: 6 }}>
      {mounts.map((m, i) => (
        <div key={i} style={{ background: '#07070f', borderRadius: 6, padding: '6px 8px', border: '1px solid #1e1e3a' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 3 }}>
            <span style={{ fontSize: 9, padding: '1px 5px', borderRadius: 3, background: `${typeColor[m.type] ?? '#64748b'}18`, color: typeColor[m.type] ?? '#64748b', fontWeight: 600 }}>{m.type}</span>
            <span style={{ fontSize: 9, color: m.mode === 'ro' ? '#f59e0b' : '#334155' }}>{m.mode === 'ro' ? 'read-only' : 'rw'}</span>
          </div>
          <div style={{ fontSize: 10, fontFamily: 'JetBrains Mono, monospace', color: '#94a3b8', marginBottom: 2, wordBreak: 'break-all', lineHeight: 1.4 }}>{m.source || '(anonymous)'}</div>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 4 }}>
            <span style={{ fontSize: 9, color: '#334155', flexShrink: 0, paddingTop: 1 }}>→</span>
            <span style={{ fontSize: 10, fontFamily: 'JetBrains Mono, monospace', color: '#6366f1', wordBreak: 'break-all', lineHeight: 1.4 }}>{m.destination}</span>
          </div>
        </div>
      ))}
    </div>
  )
}

function WorkloadContainersPanel({ containers }: { containers: any[] }) {
  if (!containers || containers.length === 0) return <p style={{ fontSize: 11, color: '#334155', padding: '8px 16px' }}>No containers</p>
  return (
    <div style={{ padding: '8px 12px 10px', display: 'flex', flexDirection: 'column', gap: 8 }}>
      {containers.map((c, i) => {
        const req = c.requests ?? {}
        const lim = c.limits ?? {}
        const ports: any[] = c.ports ?? []
        const envKeys: string[] = c.envKeys ?? []
        const envFrom: string[] = c.envFrom ?? []
        return (
          <div key={i} style={{ background: '#07070f', borderRadius: 6, padding: '8px 10px', border: '1px solid #1e1e3a' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
              <span style={{ fontSize: 11, fontWeight: 600, color: '#e2e8f0' }}>{c.name}</span>
            </div>
            <div style={{ fontSize: 10, fontFamily: 'JetBrains Mono, monospace', color: '#94a3b8', wordBreak: 'break-all', marginBottom: 6, padding: '3px 6px', background: '#0d0d1e', borderRadius: 4 }} title={c.image}>{c.image}</div>
            {(req.cpu || req.memory || lim.cpu || lim.memory) && (
              <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr auto 1fr', gap: '2px 6px', fontSize: 10, marginBottom: 6 }}>
                <span style={{ color: '#475569' }}>req cpu</span><span style={{ fontFamily: 'JetBrains Mono, monospace', color: '#94a3b8' }}>{req.cpu ?? '—'}</span>
                <span style={{ color: '#475569' }}>req mem</span><span style={{ fontFamily: 'JetBrains Mono, monospace', color: '#94a3b8' }}>{req.memory ?? '—'}</span>
                <span style={{ color: '#475569' }}>lim cpu</span><span style={{ fontFamily: 'JetBrains Mono, monospace', color: '#94a3b8' }}>{lim.cpu ?? '—'}</span>
                <span style={{ color: '#475569' }}>lim mem</span><span style={{ fontFamily: 'JetBrains Mono, monospace', color: '#94a3b8' }}>{lim.memory ?? '—'}</span>
              </div>
            )}
            {ports.length > 0 && (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginBottom: 6 }}>
                {ports.map((p, pi) => (
                  <span key={pi} style={{ fontSize: 10, fontFamily: 'JetBrains Mono, monospace', padding: '1px 5px', background: '#070d14', color: '#89b4fa', borderRadius: 3 }}>
                    {p.name ? `${p.name}:` : ''}{p.containerPort}/{p.protocol || 'TCP'}
                  </span>
                ))}
              </div>
            )}
            {envKeys.length > 0 && (
              <div style={{ fontSize: 10, color: '#475569', marginBottom: envFrom.length > 0 ? 4 : 0 }}>
                env: <span style={{ color: '#64748b', fontFamily: 'JetBrains Mono, monospace' }}>{envKeys.slice(0, 6).join(', ')}{envKeys.length > 6 ? ` +${envKeys.length - 6}` : ''}</span>
              </div>
            )}
            {envFrom.length > 0 && (
              <div style={{ fontSize: 10, color: '#475569' }}>
                envFrom: <span style={{ color: '#a78bfa', fontFamily: 'JetBrains Mono, monospace' }}>{envFrom.join(', ')}</span>
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

function ImageDetailsPanel({ node }: { node: GraphNode }) {
  const usedBy: string[] = node.metadata.usedByContainers ?? []
  return (
    <div style={{ padding: '8px 12px 10px', display: 'flex', flexDirection: 'column', gap: 6 }}>
      {[
        ['Registry', node.metadata.registry || 'docker.io'],
        ['Repository', node.metadata.repository],
        ['Tag', node.metadata.tag || 'latest'],
        ['Size', formatBytes(node.metadata.size)],
        ['Created', node.metadata.created ? new Date(node.metadata.created).toLocaleDateString() : '—'],
        ['Digest', node.metadata.digest ? node.metadata.digest.slice(0, 19) + '…' : '—'],
      ].filter(([, v]) => v && v !== '—').map(([k, v]) => (
        <div key={k as string} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
          <span style={{ fontSize: 10, color: '#475569' }}>{k}</span>
          <span style={{ fontSize: 11, fontFamily: 'JetBrains Mono, monospace', color: '#94a3b8', maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{v as string}</span>
        </div>
      ))}
      {usedBy.length > 0 && (
        <div style={{ marginTop: 4 }}>
          <p style={{ fontSize: 10, color: '#475569', marginBottom: 4 }}>Used by {usedBy.length} container{usedBy.length !== 1 ? 's' : ''}</p>
          {usedBy.slice(0, 5).map((id: string) => (
            <div key={id} style={{ fontSize: 10, fontFamily: 'JetBrains Mono, monospace', color: '#64748b', padding: '2px 6px', background: '#07070f', borderRadius: 3, marginBottom: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{id}</div>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Component ────────────────────────────────────────────────────────────────

interface NodeDetailPanelProps {
  node: GraphNode
  vmCode: string
  onClose: () => void
  onShowLogs?: () => void
  onShowTerminal?: () => void
}

type ActionStatus = 'idle' | 'confirming' | 'running' | 'success' | 'error'
const TOOLS_NODES = new Set(['container', 'pod', 'host'])

export default function NodeDetailPanel({ node, vmCode, onClose, onShowLogs, onShowTerminal }: NodeDetailPanelProps) {
  const color = getNodeColor(node.type, node.health)
  const icon = getNodeIcon(node.type)
  const hc = HEALTH_COLOR[node.health] ?? '#64748b'
  const actions = ACTIONS[node.type] ?? []

  const [activeActionId, setActiveActionId] = useState<string | null>(null)
  const [formValues, setFormValues] = useState<Record<string, string>>({})
  const [actionStatus, setActionStatus] = useState<ActionStatus>('idle')
  const [actionMsg, setActionMsg] = useState('')
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Collapsible section states
  const [openEnv,    setOpenEnv]    = useState(true)
  const [openPorts,  setOpenPorts]  = useState(true)
  const [openMounts, setOpenMounts] = useState(false)
  const [openImage,  setOpenImage]  = useState(true)
  const [openContainers, setOpenContainers] = useState(true)
  const [openMeta,   setOpenMeta]   = useState(false)

  useEffect(() => {
    setActiveActionId(null)
    setActionStatus('idle')
    setActionMsg('')
    setFormValues({})
  }, [node.id])

  useEffect(() => {
    const unsubResult = subscribeActionResult((data) => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current)
      if (data.success === false || data.status === 'failed') {
        setActionStatus('error')
        setActionMsg(data.message ?? data.error ?? 'Action failed')
      } else {
        setActionStatus('success')
        setActionMsg(data.message ?? 'Done')
        // Mutating actions (image update, scale, restart, delete...) change cluster
        // state — kick a discovery refresh immediately and again a few seconds later
        // to catch rollout progress (new replicaset, pod phase transitions). Without
        // this the panel still shows the pre-action snapshot until the next periodic
        // scan or a manual refresh click.
        sendCommand(vmCode, 'refresh')
        setTimeout(() => sendCommand(vmCode, 'refresh'), 3000)
        setTimeout(() => sendCommand(vmCode, 'refresh'), 10000)
      }
    })
    const unsubProgress = subscribeActionProgress((data) => {
      setActionMsg(data.message ?? `${data.progress ?? 0}%`)
    })
    return () => { unsubResult(); unsubProgress() }
  }, [vmCode])

  function openAction(action: ActionDef) {
    if (activeActionId === action.id) { setActiveActionId(null); setActionStatus('idle'); setActionMsg(''); return }
    const defaults: Record<string, string> = {}
    for (const f of action.form ?? []) defaults[f.key] = f.defaultValue ? f.defaultValue(node) : ''
    setFormValues(defaults)
    setActiveActionId(action.id)
    setActionStatus(action.confirm ? 'confirming' : 'idle')
    setActionMsg('')
  }

  function handleSubmit(action: ActionDef) {
    setActionStatus('running')
    setActionMsg('Sending…')
    sendAction(vmCode, action.buildPayload(node, formValues))
    timeoutRef.current = setTimeout(() => {
      setActionStatus('error')
      setActionMsg('No response from agent — check agent logs')
    }, 20_000)
  }

  // Derive inspection data from metadata (backend uses camelCase)
  const env: Record<string, string>   = node.metadata.environment ?? {}
  const ports: any[]                  = node.metadata.portMappings ?? []
  const mounts: any[]                 = node.metadata.mounts ?? []
  const envCount                      = Object.keys(env).length
  const keyFields                     = KEY_META_FIELDS[node.type] ?? []
  const keyMeta                       = keyFields.filter((k) => node.metadata[k] != null && node.metadata[k] !== '').map((k) => [k, node.metadata[k]] as [string, any])
  const allMetaEntries                = Object.entries(node.metadata)

  return (
    <div style={{ position: 'absolute', right: 0, top: 0, bottom: 0, width: 340, background: '#0a0a16', borderLeft: '1px solid #1e1e3a', display: 'flex', flexDirection: 'column', zIndex: 30, boxShadow: '-8px 0 32px rgba(0,0,0,0.5)' }}>

      {/* Header */}
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

      {/* Body */}
      <div style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column' }}>

        {/* Key metadata */}
        {keyMeta.length > 0 && (
          <div style={{ padding: '10px 16px', borderBottom: '1px solid #0f0f1e' }}>
            {keyMeta.map(([k, v]) => {
              const str = typeof v === 'object' ? JSON.stringify(v) : String(v ?? '')
              const display = k === 'size' ? formatBytes(Number(v)) : str
              const isPath = k === 'mountpoint' || k === 'image'
              return (
                <div key={k} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 8, marginBottom: 5 }}>
                  <span style={{ fontSize: 10, color: '#475569', flexShrink: 0 }}>{k}</span>
                  <span style={{ fontSize: 11, fontFamily: 'JetBrains Mono, monospace', color: '#94a3b8', textAlign: 'right', ...(isPath ? { wordBreak: 'break-all' as const, whiteSpace: 'normal' as const } : { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const, maxWidth: 190 }) }} title={str}>{display}</span>
                </div>
              )
            })}
          </div>
        )}

        {/* Tools */}
        {TOOLS_NODES.has(node.type) && (onShowLogs || onShowTerminal) && (
          <div style={{ padding: '10px 16px', borderBottom: '1px solid #1e1e3a' }}>
            <p style={{ fontSize: 10, fontWeight: 600, color: '#334155', letterSpacing: '0.08em', textTransform: 'uppercase', marginBottom: 8 }}>Tools</p>
            <div style={{ display: 'flex', gap: 6 }}>
              {onShowLogs && (
                <button onClick={onShowLogs} style={TOOL_BTN}
                  onMouseEnter={(e) => { e.currentTarget.style.borderColor = '#10b981'; e.currentTarget.style.color = '#10b981'; e.currentTarget.style.background = '#10b98110' }}
                  onMouseLeave={(e) => { e.currentTarget.style.borderColor = '#1e1e3a'; e.currentTarget.style.color = '#64748b'; e.currentTarget.style.background = '#070711' }}>
                  <FileText size={11} /> Logs
                </button>
              )}
              {onShowTerminal && (
                <button onClick={onShowTerminal} style={TOOL_BTN}
                  onMouseEnter={(e) => { e.currentTarget.style.borderColor = '#6366f1'; e.currentTarget.style.color = '#6366f1'; e.currentTarget.style.background = '#6366f110' }}
                  onMouseLeave={(e) => { e.currentTarget.style.borderColor = '#1e1e3a'; e.currentTarget.style.color = '#64748b'; e.currentTarget.style.background = '#070711' }}>
                  <Terminal size={11} /> Terminal
                </button>
              )}
            </div>
          </div>
        )}

        {/* Image details section */}
        {node.type === 'image' && (
          <div style={{ borderBottom: '1px solid #1e1e3a' }}>
            <SectionHeader title="Image Details" open={openImage} onToggle={() => setOpenImage(v => !v)} />
            {openImage && <ImageDetailsPanel node={node} />}
          </div>
        )}

        {/* Env vars — containers */}
        {node.type === 'container' && envCount > 0 && (
          <div style={{ borderBottom: '1px solid #1e1e3a' }}>
            <SectionHeader title="Environment" count={envCount} open={openEnv} onToggle={() => setOpenEnv(v => !v)} />
            {openEnv && <EnvVarsPanel env={env} />}
          </div>
        )}

        {/* Port mappings — containers */}
        {node.type === 'container' && (
          <div style={{ borderBottom: '1px solid #1e1e3a' }}>
            <SectionHeader title="Ports" count={ports.length} open={openPorts} onToggle={() => setOpenPorts(v => !v)} />
            {openPorts && <PortsPanel ports={ports} />}
          </div>
        )}

        {/* Mounts — containers */}
        {node.type === 'container' && (
          <div style={{ borderBottom: '1px solid #1e1e3a' }}>
            <SectionHeader title="Mounts" count={mounts.length} open={openMounts} onToggle={() => setOpenMounts(v => !v)} />
            {openMounts && <MountsPanel mounts={mounts} />}
          </div>
        )}

        {/* Containers — workloads */}
        {(node.type === 'deployment' || node.type === 'statefulset' || node.type === 'daemonset') && Array.isArray(node.metadata.containers) && node.metadata.containers.length > 0 && (
          <div style={{ borderBottom: '1px solid #1e1e3a' }}>
            <SectionHeader title="Containers" count={node.metadata.containers.length} open={openContainers} onToggle={() => setOpenContainers(v => !v)} />
            {openContainers && <WorkloadContainersPanel containers={node.metadata.containers} />}
          </div>
        )}

        {/* Actions */}
        {actions.length > 0 && (
          <div style={{ padding: '12px 16px', borderBottom: '1px solid #1e1e3a' }}>
            <p style={{ fontSize: 10, fontWeight: 600, color: '#334155', letterSpacing: '0.08em', textTransform: 'uppercase', marginBottom: 10 }}>Actions</p>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: activeActionId ? 12 : 0 }}>
              {actions.map((action) => {
                const isOpen = activeActionId === action.id
                return (
                  <button key={action.id} onClick={() => openAction(action)}
                    style={{ display: 'flex', alignItems: 'center', gap: 5, padding: '5px 11px', borderRadius: 6, fontSize: 11, fontWeight: 500, border: `1px solid ${isOpen ? action.color : '#1e1e3a'}`, background: isOpen ? `${action.color}18` : '#070711', color: isOpen ? action.color : '#64748b', cursor: 'pointer', transition: 'all 0.15s' }}
                    onMouseEnter={(e) => { if (!isOpen) { e.currentTarget.style.borderColor = action.color; e.currentTarget.style.color = action.color; e.currentTarget.style.background = `${action.color}10` } }}
                    onMouseLeave={(e) => { if (!isOpen) { e.currentTarget.style.borderColor = '#1e1e3a'; e.currentTarget.style.color = '#64748b'; e.currentTarget.style.background = '#070711' } }}
                  >
                    <action.Icon size={11} />{action.label}
                    {isOpen ? <ChevronDown size={10} style={{ opacity: 0.5 }} /> : <ChevronRight size={10} style={{ opacity: 0.3 }} />}
                  </button>
                )
              })}
            </div>

            {activeActionId && (() => {
              const action = actions.find((a) => a.id === activeActionId)!
              return (
                <div style={{ background: '#070711', border: '1px solid #1e1e3a', borderRadius: 8, padding: 12 }}>
                  {action.form && actionStatus === 'idle' && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 12 }}>
                      {action.form.map((field) => {
                        const baseStyle: React.CSSProperties = { width: '100%', boxSizing: 'border-box' as const, background: '#0a0a16', border: '1px solid #1e1e3a', borderRadius: 6, padding: '6px 10px', fontSize: 12, fontFamily: 'JetBrains Mono, monospace', color: '#e2e8f0', outline: 'none' }
                        const opts = field.type === 'select' ? (field.options?.(node) ?? []) : []
                        const useSelect = field.type === 'select' && opts.length > 0
                        return (
                          <div key={field.key}>
                            <label style={{ fontSize: 10, color: '#475569', display: 'block', marginBottom: 4 }}>{field.label}</label>
                            {useSelect ? (
                              <select
                                value={formValues[field.key] ?? ''}
                                onChange={(e) => {
                                  const picked = opts.find((o) => o.value === e.target.value)
                                  setFormValues((prev) => ({ ...prev, [field.key]: e.target.value, ...(picked?.prefill ?? {}) }))
                                }}
                                style={baseStyle}
                              >
                                {opts.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                              </select>
                            ) : (
                              <input
                                type={field.type === 'number' ? 'number' : 'text'}
                                value={formValues[field.key] ?? ''}
                                onChange={(e) => setFormValues((prev) => ({ ...prev, [field.key]: e.target.value }))}
                                placeholder={field.placeholder}
                                style={baseStyle}
                                onFocus={(e) => { e.currentTarget.style.borderColor = action.color }}
                                onBlur={(e) => { e.currentTarget.style.borderColor = '#1e1e3a' }}
                              />
                            )}
                          </div>
                        )
                      })}
                    </div>
                  )}
                  {action.confirm && actionStatus === 'confirming' && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 10 }}>
                      {action.danger && <AlertTriangle size={12} color="#f59e0b" />}
                      <span style={{ fontSize: 11, color: action.danger ? '#fcd34d' : '#94a3b8' }}>
                        {action.danger ? 'This is a destructive action. Confirm?' : `Run "${action.label}" on ${node.label}?`}
                      </span>
                    </div>
                  )}
                  {actionStatus === 'running' && <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}><Loader2 size={13} color="#6366f1" style={{ animation: 'spin 1s linear infinite' }} /><span style={{ fontSize: 11, color: '#64748b' }}>{actionMsg}</span></div>}
                  {actionStatus === 'success' && <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}><CheckCircle2 size={13} color="#10b981" /><span style={{ fontSize: 11, color: '#6ee7b7' }}>{actionMsg}</span></div>}
                  {actionStatus === 'error' && <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8, marginBottom: 10 }}><XCircle size={13} color="#ef4444" style={{ flexShrink: 0, marginTop: 1 }} /><span style={{ fontSize: 11, color: '#fca5a5', wordBreak: 'break-word' }}>{actionMsg}</span></div>}
                  {(actionStatus === 'idle' || actionStatus === 'confirming') && (
                    <div style={{ display: 'flex', gap: 6 }}>
                      <button onClick={() => handleSubmit(action)} style={{ flex: 1, padding: '6px 10px', borderRadius: 6, background: action.danger ? '#ef444420' : `${action.color}20`, color: action.danger ? '#ef4444' : action.color, fontSize: 11, fontWeight: 600, cursor: 'pointer', border: `1px solid ${action.danger ? '#ef444440' : `${action.color}40`}` } as React.CSSProperties} onMouseEnter={(e) => { e.currentTarget.style.opacity = '0.8' }} onMouseLeave={(e) => { e.currentTarget.style.opacity = '1' }}>
                        {action.confirm ? `Confirm ${action.label}` : `Apply ${action.label}`}
                      </button>
                      <button onClick={() => { setActiveActionId(null); setActionStatus('idle') }} style={{ padding: '6px 10px', borderRadius: 6, border: '1px solid #1e1e3a', background: 'transparent', color: '#475569', fontSize: 11, cursor: 'pointer' }}>Cancel</button>
                    </div>
                  )}
                  {(actionStatus === 'success' || actionStatus === 'error') && (
                    <button onClick={() => { setActiveActionId(null); setActionStatus('idle'); setActionMsg('') }} style={{ width: '100%', padding: '6px', borderRadius: 6, border: '1px solid #1e1e3a', background: 'transparent', color: '#475569', fontSize: 11, cursor: 'pointer', marginTop: 4 }}>Dismiss</button>
                  )}
                </div>
              )
            })()}
          </div>
        )}

        {/* All Metadata (collapsible) */}
        <div style={{ padding: '0 0 8px' }}>
          <SectionHeader title={`All Metadata (${allMetaEntries.length})`} open={openMeta} onToggle={() => setOpenMeta(v => !v)} />
          {openMeta && (
            <div style={{ padding: '8px 16px' }}>
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

// ─── Styles ───────────────────────────────────────────────────────────────────

const ICON_BTN: React.CSSProperties = { width: 28, height: 28, borderRadius: 7, border: 'none', background: 'transparent', color: '#475569', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }
const TOOL_BTN: React.CSSProperties = { display: 'flex', alignItems: 'center', gap: 5, padding: '5px 11px', borderRadius: 6, fontSize: 11, fontWeight: 500, border: '1px solid #1e1e3a', background: '#070711', color: '#64748b', cursor: 'pointer' }
