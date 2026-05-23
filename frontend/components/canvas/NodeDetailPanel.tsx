'use client'

import { useState, useEffect, useRef } from 'react'
import {
  X, RotateCw, Square, Play, Tag, Layers, Trash2, Undo2,
  CheckCircle2, XCircle, Loader2, ChevronDown, ChevronRight,
  AlertTriangle, FileText, Terminal, Eye, EyeOff, Download,
  ShieldOff, Shield,
  type LucideIcon,
} from 'lucide-react'
import { type GraphNode } from '@/types'
import { sendAction, sendCommand, subscribeActionResult, subscribeActionProgress } from '@/lib/wsManager'
import NodeSvgIcon from './NodeSvgIcon'

// ─── Types ────────────────────────────────────────────────────────────────────

interface FormFieldOption {
  value: string
  label: string
  prefill?: Record<string, string>
}

interface FormField {
  key: string
  label: string
  placeholder?: string
  type?: 'text' | 'number' | 'select'
  defaultValue?: (node: GraphNode) => string
  options?: (node: GraphNode) => FormFieldOption[]
}

interface ActionDef {
  id: string
  label: string
  Icon: LucideIcon
  danger?: boolean
  confirm?: boolean
  form?: FormField[]
  buildPayload: (node: GraphNode, vals: Record<string, string>) => object
}

// ─── Action registry ─────────────────────────────────────────────────────────

// ─── Helpers ─────────────────────────────────────────────────────────────────

function k8sTarget(entityType: string, n: any, namespace?: string) {
  return { layer: 'kubernetes', entity_type: entityType, entity_id: n.metadata?.name ?? n.id, namespace: namespace ?? n.metadata?.namespace ?? 'default' }
}
function dockerTarget(n: any) {
  return { layer: 'docker', entity_type: 'container', entity_id: n.id }
}
function nodeTarget(n: any) {
  return { layer: 'kubernetes', entity_type: 'node', entity_id: n.metadata?.name ?? n.id }
}

const ACTIONS: Record<string, ActionDef[]> = {
  container: [
    { id: 'restart', label: 'Restart', Icon: RotateCw, confirm: true,
      buildPayload: (n) => ({ action_id: `restart-${Date.now()}`, type: 'docker_restart_container', target: dockerTarget(n), parameters: {} }) },
    { id: 'stop', label: 'Stop', Icon: Square, confirm: true,
      buildPayload: (n) => ({ action_id: `stop-${Date.now()}`, type: 'docker_stop_container', target: dockerTarget(n), parameters: {} }) },
    { id: 'start', label: 'Start', Icon: Play,
      buildPayload: (n) => ({ action_id: `start-${Date.now()}`, type: 'docker_start_container', target: dockerTarget(n), parameters: {} }) },
    { id: 'update_image', label: 'Update Image', Icon: Tag,
      form: [{ key: 'image', label: 'New Image:Tag', placeholder: 'nginx:1.25', defaultValue: (n) => n.metadata?.image ?? '' }],
      buildPayload: (n, v) => ({ action_id: `upd-img-${Date.now()}`, type: 'docker_update_container_image', target: dockerTarget(n), parameters: { image: v.image } }) },
  ],
  image: [
    { id: 'pull', label: 'Pull / Re-pull', Icon: Download,
      form: [{ key: 'image', label: 'Image:Tag', placeholder: 'nginx:latest', defaultValue: (n) => n.label }],
      buildPayload: (n, v) => ({ action_id: `pull-${Date.now()}`, type: 'docker_pull_image', target: { layer: 'docker', entity_type: 'image', entity_id: n.id }, parameters: { image: v.image } }) },
    { id: 'remove', label: 'Remove Image', Icon: Trash2, danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `rmimg-${Date.now()}`, type: 'docker_remove_image', target: { layer: 'docker', entity_type: 'image', entity_id: n.id }, parameters: {} }) },
  ],
  deployment: [
    { id: 'scale', label: 'Scale', Icon: Layers,
      form: [{ key: 'replicas', label: 'Replicas', placeholder: '3', type: 'number', defaultValue: (n) => String(n.metadata?.replicas ?? '1') }],
      buildPayload: (n, v) => ({ action_id: `scale-${Date.now()}`, type: 'k8s_scale_deployment', target: k8sTarget('deployment', n), parameters: { replicas: v.replicas } }) },
    { id: 'restart', label: 'Rolling Restart', Icon: RotateCw, confirm: true,
      buildPayload: (n) => ({ action_id: `restart-${Date.now()}`, type: 'k8s_restart_deployment', target: k8sTarget('deployment', n), parameters: {} }) },
    { id: 'rollback', label: 'Rollback', Icon: Undo2, confirm: true,
      buildPayload: (n) => ({ action_id: `undo-${Date.now()}`, type: 'k8s_rollout_undo', target: k8sTarget('deployment', n), parameters: {} }) },
    { id: 'update_image', label: 'Update Image', Icon: Tag,
      form: [
        { key: 'container', label: 'Container', type: 'select',
          defaultValue: (n) => n.metadata?.containers?.[0]?.name ?? '',
          options: (n) => (n.metadata?.containers ?? []).map((c: any) => ({ value: c.name, label: `${c.name} — ${c.image}`, prefill: { image: c.image } })) },
        { key: 'image', label: 'New Image:Tag', placeholder: 'registry/name:v2.0', defaultValue: (n) => n.metadata?.containers?.[0]?.image ?? '' },
      ],
      buildPayload: (n, v) => ({ action_id: `upd-img-${Date.now()}`, type: 'k8s_update_image', target: k8sTarget('deployment', n), parameters: { image: v.image, container: v.container } }) },
    { id: 'delete', label: 'Delete Deployment', Icon: Trash2, danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `del-${Date.now()}`, type: 'k8s_delete_deployment', target: k8sTarget('deployment', n), parameters: {} }) },
  ],
  statefulset: [
    { id: 'scale', label: 'Scale', Icon: Layers,
      form: [{ key: 'replicas', label: 'Replicas', placeholder: '3', type: 'number', defaultValue: (n) => String(n.metadata?.replicas ?? '1') }],
      buildPayload: (n, v) => ({ action_id: `scale-${Date.now()}`, type: 'k8s_scale_statefulset', target: k8sTarget('statefulset', n), parameters: { replicas: v.replicas } }) },
    { id: 'restart', label: 'Rolling Restart', Icon: RotateCw, confirm: true,
      buildPayload: (n) => ({ action_id: `restart-${Date.now()}`, type: 'k8s_restart_statefulset', target: k8sTarget('statefulset', n), parameters: {} }) },
    { id: 'rollback', label: 'Rollback', Icon: Undo2, confirm: true,
      buildPayload: (n) => ({ action_id: `undo-${Date.now()}`, type: 'k8s_rollout_undo', target: k8sTarget('statefulset', n), parameters: {} }) },
    { id: 'update_image', label: 'Update Image', Icon: Tag,
      form: [
        { key: 'container', label: 'Container', type: 'select',
          defaultValue: (n) => n.metadata?.containers?.[0]?.name ?? '',
          options: (n) => (n.metadata?.containers ?? []).map((c: any) => ({ value: c.name, label: `${c.name} — ${c.image}`, prefill: { image: c.image } })) },
        { key: 'image', label: 'New Image:Tag', placeholder: 'registry/name:v2.0', defaultValue: (n) => n.metadata?.containers?.[0]?.image ?? '' },
      ],
      buildPayload: (n, v) => ({ action_id: `upd-img-${Date.now()}`, type: 'k8s_update_image', target: k8sTarget('statefulset', n), parameters: { image: v.image, container: v.container } }) },
  ],
  daemonset: [
    { id: 'restart', label: 'Rolling Restart', Icon: RotateCw, confirm: true,
      buildPayload: (n) => ({ action_id: `restart-${Date.now()}`, type: 'k8s_restart_daemonset', target: k8sTarget('daemonset', n), parameters: {} }) },
    { id: 'update_image', label: 'Update Image', Icon: Tag,
      form: [
        { key: 'container', label: 'Container', type: 'select',
          defaultValue: (n) => n.metadata?.containers?.[0]?.name ?? '',
          options: (n) => (n.metadata?.containers ?? []).map((c: any) => ({ value: c.name, label: `${c.name} — ${c.image}`, prefill: { image: c.image } })) },
        { key: 'image', label: 'New Image:Tag', placeholder: 'registry/name:v2.0', defaultValue: (n) => n.metadata?.containers?.[0]?.image ?? '' },
      ],
      buildPayload: (n, v) => ({ action_id: `upd-img-${Date.now()}`, type: 'k8s_update_image', target: k8sTarget('daemonset', n), parameters: { image: v.image, container: v.container } }) },
  ],
  pod: [
    { id: 'delete', label: 'Delete / Restart', Icon: Trash2, danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `del-${Date.now()}`, type: 'k8s_delete_pod', target: k8sTarget('pod', n), parameters: {} }) },
  ],
  node: [
    { id: 'cordon', label: 'Cordon', Icon: ShieldOff, confirm: true,
      buildPayload: (n) => ({ action_id: `cordon-${Date.now()}`, type: 'k8s_cordon_node', target: nodeTarget(n), parameters: {} }) },
    { id: 'uncordon', label: 'Uncordon', Icon: Shield,
      buildPayload: (n) => ({ action_id: `uncordon-${Date.now()}`, type: 'k8s_uncordon_node', target: nodeTarget(n), parameters: {} }) },
    { id: 'drain', label: 'Drain', Icon: Layers, danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `drain-${Date.now()}`, type: 'k8s_drain_node', target: nodeTarget(n), parameters: {} }) },
  ],
  job: [
    { id: 'delete', label: 'Delete Job', Icon: Trash2, danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `del-${Date.now()}`, type: 'k8s_delete_job', target: k8sTarget('job', n), parameters: {} }) },
  ],
  k8s_service: [
    { id: 'delete', label: 'Delete Service', Icon: Trash2, danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `del-${Date.now()}`, type: 'k8s_delete_service', target: k8sTarget('service', n), parameters: {} }) },
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
  healthy: '#22c55e', degraded: '#f59e0b', unhealthy: '#ef4444', unknown: '#6b7280',
}

const MONO = "var(--font-geist-mono,'Geist Mono','JetBrains Mono',ui-monospace,monospace)"

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
      style={{ width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '9px 16px', background: 'transparent', border: 'none', borderBottom: open ? '1px solid var(--surface)' : 'none', cursor: 'pointer', color: 'var(--ink4)' }}
      onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--surface)' }}
      onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <span style={{ fontSize: 10, fontWeight: 600, letterSpacing: '0.08em', textTransform: 'uppercase' as const }}>{title}</span>
        {count != null && <span style={{ fontSize: 9, padding: '1px 5px', borderRadius: 10, background: 'var(--line)', color: 'var(--ink3)' }}>{count}</span>}
      </div>
      {open ? <ChevronDown size={11} /> : <ChevronRight size={11} />}
    </button>
  )
}

function EnvVarsPanel({ env }: { env: Record<string, string> }) {
  const [showSecrets, setShowSecrets] = useState(false)
  const entries = Object.entries(env)
  if (entries.length === 0) return <p style={{ fontSize: 11, color: 'var(--ink4)', padding: '8px 16px' }}>No environment variables</p>

  return (
    <div style={{ padding: '6px 12px 10px' }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 6 }}>
        <button
          onClick={() => setShowSecrets(v => !v)}
          style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 10, color: 'var(--ink3)', background: 'transparent', border: '1px solid var(--line)', borderRadius: 4, padding: '2px 7px', cursor: 'pointer' }}
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
            <div key={k} style={{ display: 'flex', background: 'var(--bg)', borderRadius: 4, overflow: 'hidden', marginBottom: 2 }}>
              <span style={{ fontSize: 10, fontFamily: MONO, color: 'var(--ink2)', padding: '3px 6px', background: 'var(--surface)', flexShrink: 0, maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={k}>{k}</span>
              <span style={{ fontSize: 10, fontFamily: MONO, color: isSecret && !showSecrets ? 'var(--ink4)' : 'var(--ink3)', padding: '3px 6px', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={display}>{display}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function PortsPanel({ ports }: { ports: any[] }) {
  if (!ports || ports.length === 0) return <p style={{ fontSize: 11, color: 'var(--ink4)', padding: '8px 16px' }}>No port mappings</p>
  return (
    <div style={{ padding: '8px 12px 10px' }}>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr auto 1fr', gap: '3px 8px', alignItems: 'center' }}>
        <span style={{ fontSize: 9, color: 'var(--ink4)', fontWeight: 600 }}>HOST</span>
        <span />
        <span style={{ fontSize: 9, color: 'var(--ink4)', fontWeight: 600 }}>CONTAINER</span>
        {ports.map((p, i) => (
          <>
            <span key={`h${i}`} style={{ fontSize: 11, fontFamily: MONO, color: 'var(--ink2)', background: 'var(--surface)', borderRadius: 4, padding: '2px 6px', textAlign: 'right' }}>
              {p.hostIP && p.hostIP !== '0.0.0.0' ? `${p.hostIP}:` : ''}{p.hostPort}
            </span>
            <span key={`a${i}`} style={{ fontSize: 9, color: 'var(--ink4)', textAlign: 'center' }}>→</span>
            <span key={`c${i}`} style={{ fontSize: 11, fontFamily: MONO, color: 'var(--ink2)', background: 'var(--surface)', borderRadius: 4, padding: '2px 6px' }}>
              {p.containerPort}<span style={{ color: 'var(--ink4)' }}>/{p.protocol}</span>
            </span>
          </>
        ))}
      </div>
    </div>
  )
}

function MountsPanel({ mounts }: { mounts: Array<{ source: string; destination: string; mode: string; type: string }> }) {
  if (!mounts || mounts.length === 0) return <p style={{ fontSize: 11, color: 'var(--ink4)', padding: '8px 16px' }}>No mounts</p>
  return (
    <div style={{ padding: '8px 12px 10px', display: 'flex', flexDirection: 'column', gap: 6 }}>
      {mounts.map((m, i) => (
        <div key={i} style={{ background: 'var(--bg)', borderRadius: 6, padding: '6px 8px', border: '1px solid var(--line)' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 3 }}>
            <span style={{ fontSize: 9, padding: '1px 5px', borderRadius: 3, background: 'var(--line)', color: 'var(--ink2)', fontWeight: 600 }}>{m.type}</span>
            <span style={{ fontSize: 9, color: m.mode === 'ro' ? '#f59e0b' : 'var(--ink4)' }}>{m.mode === 'ro' ? 'read-only' : 'rw'}</span>
          </div>
          <div style={{ fontSize: 10, fontFamily: MONO, color: 'var(--ink3)', marginBottom: 2, wordBreak: 'break-all', lineHeight: 1.4 }}>{m.source || '(anonymous)'}</div>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 4 }}>
            <span style={{ fontSize: 9, color: 'var(--ink4)', flexShrink: 0, paddingTop: 1 }}>→</span>
            <span style={{ fontSize: 10, fontFamily: MONO, color: 'var(--ink2)', wordBreak: 'break-all', lineHeight: 1.4 }}>{m.destination}</span>
          </div>
        </div>
      ))}
    </div>
  )
}

function WorkloadContainersPanel({ containers }: { containers: any[] }) {
  if (!containers || containers.length === 0) return <p style={{ fontSize: 11, color: 'var(--ink4)', padding: '8px 16px' }}>No containers</p>
  return (
    <div style={{ padding: '8px 12px 10px', display: 'flex', flexDirection: 'column', gap: 8 }}>
      {containers.map((c, i) => {
        const req = c.requests ?? {}
        const lim = c.limits ?? {}
        const ports: any[] = c.ports ?? []
        const envKeys: string[] = c.envKeys ?? []
        const envFrom: string[] = c.envFrom ?? []
        return (
          <div key={i} style={{ background: 'var(--bg)', borderRadius: 6, padding: '8px 10px', border: '1px solid var(--line)' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
              <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--ink)' }}>{c.name}</span>
            </div>
            <div style={{ fontSize: 10, fontFamily: MONO, color: 'var(--ink3)', wordBreak: 'break-all', marginBottom: 6, padding: '3px 6px', background: 'var(--surface)', borderRadius: 4 }} title={c.image}>{c.image}</div>
            {(req.cpu || req.memory || lim.cpu || lim.memory) && (
              <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr auto 1fr', gap: '2px 6px', fontSize: 10, marginBottom: 6 }}>
                <span style={{ color: 'var(--ink4)' }}>req cpu</span><span style={{ fontFamily: MONO, color: 'var(--ink3)' }}>{req.cpu ?? '—'}</span>
                <span style={{ color: 'var(--ink4)' }}>req mem</span><span style={{ fontFamily: MONO, color: 'var(--ink3)' }}>{req.memory ?? '—'}</span>
                <span style={{ color: 'var(--ink4)' }}>lim cpu</span><span style={{ fontFamily: MONO, color: 'var(--ink3)' }}>{lim.cpu ?? '—'}</span>
                <span style={{ color: 'var(--ink4)' }}>lim mem</span><span style={{ fontFamily: MONO, color: 'var(--ink3)' }}>{lim.memory ?? '—'}</span>
              </div>
            )}
            {ports.length > 0 && (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginBottom: 6 }}>
                {ports.map((p, pi) => (
                  <span key={pi} style={{ fontSize: 10, fontFamily: MONO, padding: '1px 5px', background: 'var(--surface)', color: 'var(--ink2)', borderRadius: 3 }}>
                    {p.name ? `${p.name}:` : ''}{p.containerPort}/{p.protocol || 'TCP'}
                  </span>
                ))}
              </div>
            )}
            {envKeys.length > 0 && (
              <div style={{ fontSize: 10, color: 'var(--ink4)', marginBottom: envFrom.length > 0 ? 4 : 0 }}>
                env: <span style={{ color: 'var(--ink3)', fontFamily: MONO }}>{envKeys.slice(0, 6).join(', ')}{envKeys.length > 6 ? ` +${envKeys.length - 6}` : ''}</span>
              </div>
            )}
            {envFrom.length > 0 && (
              <div style={{ fontSize: 10, color: 'var(--ink4)' }}>
                envFrom: <span style={{ color: 'var(--ink2)', fontFamily: MONO }}>{envFrom.join(', ')}</span>
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
          <span style={{ fontSize: 10, color: 'var(--ink4)' }}>{k}</span>
          <span style={{ fontSize: 11, fontFamily: MONO, color: 'var(--ink3)', maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{v as string}</span>
        </div>
      ))}
      {usedBy.length > 0 && (
        <div style={{ marginTop: 4 }}>
          <p style={{ fontSize: 10, color: 'var(--ink4)', marginBottom: 4 }}>Used by {usedBy.length} container{usedBy.length !== 1 ? 's' : ''}</p>
          {usedBy.slice(0, 5).map((id: string) => (
            <div key={id} style={{ fontSize: 10, fontFamily: MONO, color: 'var(--ink3)', padding: '2px 6px', background: 'var(--bg)', borderRadius: 3, marginBottom: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{id}</div>
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

// Action button accent colors (functional — kept for UX clarity)
const ACTION_COLORS: Record<string, string> = {
  restart: '#6366f1',
  stop: '#f59e0b',
  start: '#22c55e',
  update_image: '#8b5cf6',
  change_tag: '#6366f1',
  scale: '#22c55e',
  cordon: '#f59e0b',
  drain: '#ef4444',
  delete: '#ef4444',
}

function getActionColor(id: string, danger?: boolean): string {
  if (danger) return '#ef4444'
  return ACTION_COLORS[id] ?? 'var(--ink2)'
}

export default function NodeDetailPanel({ node, vmCode, onClose, onShowLogs, onShowTerminal }: NodeDetailPanelProps) {
  const hc = HEALTH_COLOR[node.health] ?? '#6b7280'
  const actions = ACTIONS[node.type] ?? []

  const [activeActionId, setActiveActionId] = useState<string | null>(null)
  const [formValues, setFormValues] = useState<Record<string, string>>({})
  const [actionStatus, setActionStatus] = useState<ActionStatus>('idle')
  const [actionMsg, setActionMsg] = useState('')
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const [openEnv,        setOpenEnv]        = useState(true)
  const [openPorts,      setOpenPorts]      = useState(true)
  const [openMounts,     setOpenMounts]     = useState(false)
  const [openImage,      setOpenImage]      = useState(true)
  const [openContainers, setOpenContainers] = useState(true)
  const [openMeta,       setOpenMeta]       = useState(false)

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

  const env: Record<string, string>   = node.metadata.environment ?? {}
  const ports: any[]                  = node.metadata.portMappings ?? []
  const mounts: any[]                 = node.metadata.mounts ?? []
  const envCount                      = Object.keys(env).length
  const keyFields                     = KEY_META_FIELDS[node.type] ?? []
  const keyMeta                       = keyFields.filter((k) => node.metadata[k] != null && node.metadata[k] !== '').map((k) => [k, node.metadata[k]] as [string, any])
  const allMetaEntries                = Object.entries(node.metadata)

  return (
    <div style={{ position: 'absolute', right: 0, top: 0, bottom: 0, width: 340, background: 'var(--surface)', borderLeft: '1px solid var(--line)', display: 'flex', flexDirection: 'column', zIndex: 30, boxShadow: '-8px 0 32px rgba(0,0,0,0.5)' }}>

      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', padding: '12px 16px', borderBottom: '1px solid var(--line)', flexShrink: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
          <div style={{ width: 34, height: 34, borderRadius: 8, background: 'var(--line)', border: '1px solid var(--line2)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}>
            <NodeSvgIcon type={node.type} size={16} />
          </div>
          <div style={{ minWidth: 0 }}>
            <p style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={node.label}>{node.label}</p>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 3 }}>
              <span style={{ fontSize: 10, padding: '1px 6px', borderRadius: 4, background: 'var(--line)', color: 'var(--ink3)', border: '1px solid var(--line2)', fontFamily: MONO }}>{node.type}</span>
              <span style={{ fontSize: 10, color: hc, display: 'flex', alignItems: 'center', gap: 3 }}>
                <span style={{ width: 5, height: 5, borderRadius: '50%', background: hc, display: 'inline-block' }} />{node.health}
              </span>
            </div>
          </div>
        </div>
        <button onClick={onClose} style={ICON_BTN}
          onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--surface-2)'; e.currentTarget.style.color = 'var(--ink2)' }}
          onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--ink4)' }}>
          <X size={14} />
        </button>
      </div>

      {/* Body */}
      <div style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column' }}>

        {/* Key metadata */}
        {keyMeta.length > 0 && (
          <div style={{ padding: '10px 16px', borderBottom: '1px solid var(--surface)' }}>
            {keyMeta.map(([k, v]) => {
              const str = typeof v === 'object' ? JSON.stringify(v) : String(v ?? '')
              const display = k === 'size' ? formatBytes(Number(v)) : str
              const isPath = k === 'mountpoint' || k === 'image'
              return (
                <div key={k} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 8, marginBottom: 5 }}>
                  <span style={{ fontSize: 10, color: 'var(--ink4)', flexShrink: 0 }}>{k}</span>
                  <span style={{ fontSize: 11, fontFamily: MONO, color: 'var(--ink3)', textAlign: 'right', ...(isPath ? { wordBreak: 'break-all' as const, whiteSpace: 'normal' as const } : { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const, maxWidth: 190 }) }} title={str}>{display}</span>
                </div>
              )
            })}
          </div>
        )}

        {/* Tools */}
        {TOOLS_NODES.has(node.type) && (onShowLogs || onShowTerminal) && (
          <div style={{ padding: '10px 16px', borderBottom: '1px solid var(--line)' }}>
            <p style={{ fontSize: 10, fontWeight: 600, color: 'var(--ink4)', letterSpacing: '0.08em', textTransform: 'uppercase', marginBottom: 8 }}>Tools</p>
            <div style={{ display: 'flex', gap: 6 }}>
              {onShowLogs && (
                <button onClick={onShowLogs} style={TOOL_BTN}
                  onMouseEnter={(e) => { e.currentTarget.style.borderColor = 'var(--line2)'; e.currentTarget.style.color = 'var(--ink)'; e.currentTarget.style.background = 'var(--line)' }}
                  onMouseLeave={(e) => { e.currentTarget.style.borderColor = 'var(--line)'; e.currentTarget.style.color = 'var(--ink3)'; e.currentTarget.style.background = 'var(--bg)' }}>
                  <FileText size={11} /> Logs
                </button>
              )}
              {onShowTerminal && (
                <button onClick={onShowTerminal} style={TOOL_BTN}
                  onMouseEnter={(e) => { e.currentTarget.style.borderColor = 'var(--line2)'; e.currentTarget.style.color = 'var(--ink)'; e.currentTarget.style.background = 'var(--line)' }}
                  onMouseLeave={(e) => { e.currentTarget.style.borderColor = 'var(--line)'; e.currentTarget.style.color = 'var(--ink3)'; e.currentTarget.style.background = 'var(--bg)' }}>
                  <Terminal size={11} /> Terminal
                </button>
              )}
            </div>
          </div>
        )}

        {/* Image details section */}
        {node.type === 'image' && (
          <div style={{ borderBottom: '1px solid var(--line)' }}>
            <SectionHeader title="Image Details" open={openImage} onToggle={() => setOpenImage(v => !v)} />
            {openImage && <ImageDetailsPanel node={node} />}
          </div>
        )}

        {/* Env vars — containers */}
        {node.type === 'container' && envCount > 0 && (
          <div style={{ borderBottom: '1px solid var(--line)' }}>
            <SectionHeader title="Environment" count={envCount} open={openEnv} onToggle={() => setOpenEnv(v => !v)} />
            {openEnv && <EnvVarsPanel env={env} />}
          </div>
        )}

        {/* Port mappings — containers */}
        {node.type === 'container' && (
          <div style={{ borderBottom: '1px solid var(--line)' }}>
            <SectionHeader title="Ports" count={ports.length} open={openPorts} onToggle={() => setOpenPorts(v => !v)} />
            {openPorts && <PortsPanel ports={ports} />}
          </div>
        )}

        {/* Mounts — containers */}
        {node.type === 'container' && (
          <div style={{ borderBottom: '1px solid var(--line)' }}>
            <SectionHeader title="Mounts" count={mounts.length} open={openMounts} onToggle={() => setOpenMounts(v => !v)} />
            {openMounts && <MountsPanel mounts={mounts} />}
          </div>
        )}

        {/* Containers — workloads */}
        {(node.type === 'deployment' || node.type === 'statefulset' || node.type === 'daemonset') && Array.isArray(node.metadata.containers) && node.metadata.containers.length > 0 && (
          <div style={{ borderBottom: '1px solid var(--line)' }}>
            <SectionHeader title="Containers" count={node.metadata.containers.length} open={openContainers} onToggle={() => setOpenContainers(v => !v)} />
            {openContainers && <WorkloadContainersPanel containers={node.metadata.containers} />}
          </div>
        )}

        {/* Actions */}
        {actions.length > 0 && (
          <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--line)' }}>
            <p style={{ fontSize: 10, fontWeight: 600, color: 'var(--ink4)', letterSpacing: '0.08em', textTransform: 'uppercase', marginBottom: 10 }}>Actions</p>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: activeActionId ? 12 : 0 }}>
              {actions.map((action) => {
                const isOpen = activeActionId === action.id
                const ac = getActionColor(action.id, action.danger)
                return (
                  <button key={action.id} onClick={() => openAction(action)}
                    style={{ display: 'flex', alignItems: 'center', gap: 5, padding: '5px 11px', borderRadius: 6, fontSize: 11, fontWeight: 500, border: `1px solid ${isOpen ? ac : 'var(--line)'}`, background: isOpen ? `${ac}18` : 'var(--bg)', color: isOpen ? ac : 'var(--ink3)', cursor: 'pointer', transition: 'all 0.15s' }}
                    onMouseEnter={(e) => { if (!isOpen) { e.currentTarget.style.borderColor = ac; e.currentTarget.style.color = ac; e.currentTarget.style.background = `${ac}10` } }}
                    onMouseLeave={(e) => { if (!isOpen) { e.currentTarget.style.borderColor = 'var(--line)'; e.currentTarget.style.color = 'var(--ink3)'; e.currentTarget.style.background = 'var(--bg)' } }}
                  >
                    <action.Icon size={11} />{action.label}
                    {isOpen ? <ChevronDown size={10} style={{ opacity: 0.5 }} /> : <ChevronRight size={10} style={{ opacity: 0.3 }} />}
                  </button>
                )
              })}
            </div>

            {activeActionId && (() => {
              const action = actions.find((a) => a.id === activeActionId)!
              const ac = getActionColor(action.id, action.danger)
              return (
                <div style={{ background: 'var(--bg)', border: '1px solid var(--line)', borderRadius: 8, padding: 12 }}>
                  {action.form && actionStatus === 'idle' && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 12 }}>
                      {action.form.map((field) => {
                        const baseStyle: React.CSSProperties = { width: '100%', boxSizing: 'border-box' as const, background: 'var(--surface)', border: '1px solid var(--line)', borderRadius: 6, padding: '6px 10px', fontSize: 12, fontFamily: MONO, color: 'var(--ink)', outline: 'none' }
                        const opts = field.type === 'select' ? (field.options?.(node) ?? []) : []
                        const useSelect = field.type === 'select' && opts.length > 0
                        return (
                          <div key={field.key}>
                            <label style={{ fontSize: 10, color: 'var(--ink4)', display: 'block', marginBottom: 4 }}>{field.label}</label>
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
                                onFocus={(e) => { e.currentTarget.style.borderColor = 'var(--line2)' }}
                                onBlur={(e) => { e.currentTarget.style.borderColor = 'var(--line)' }}
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
                      <span style={{ fontSize: 11, color: action.danger ? '#f59e0b' : 'var(--ink2)' }}>
                        {action.danger ? 'This is a destructive action. Confirm?' : `Run "${action.label}" on ${node.label}?`}
                      </span>
                    </div>
                  )}
                  {actionStatus === 'running' && <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}><Loader2 size={13} color="var(--ink2)" style={{ animation: 'spin 1s linear infinite' }} /><span style={{ fontSize: 11, color: 'var(--ink3)' }}>{actionMsg}</span></div>}
                  {actionStatus === 'success' && <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}><CheckCircle2 size={13} color="#22c55e" /><span style={{ fontSize: 11, color: '#22c55e' }}>{actionMsg}</span></div>}
                  {actionStatus === 'error' && <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8, marginBottom: 10 }}><XCircle size={13} color="#ef4444" style={{ flexShrink: 0, marginTop: 1 }} /><span style={{ fontSize: 11, color: '#fca5a5', wordBreak: 'break-word' }}>{actionMsg}</span></div>}
                  {(actionStatus === 'idle' || actionStatus === 'confirming') && (
                    <div style={{ display: 'flex', gap: 6 }}>
                      <button onClick={() => handleSubmit(action)} style={{ flex: 1, padding: '6px 10px', borderRadius: 6, background: `${ac}20`, color: ac, fontSize: 11, fontWeight: 600, cursor: 'pointer', border: `1px solid ${ac}40` } as React.CSSProperties} onMouseEnter={(e) => { e.currentTarget.style.opacity = '0.8' }} onMouseLeave={(e) => { e.currentTarget.style.opacity = '1' }}>
                        {action.confirm ? `Confirm ${action.label}` : `Apply ${action.label}`}
                      </button>
                      <button onClick={() => { setActiveActionId(null); setActionStatus('idle') }} style={{ padding: '6px 10px', borderRadius: 6, border: '1px solid var(--line)', background: 'transparent', color: 'var(--ink3)', fontSize: 11, cursor: 'pointer' }}>Cancel</button>
                    </div>
                  )}
                  {(actionStatus === 'success' || actionStatus === 'error') && (
                    <button onClick={() => { setActiveActionId(null); setActionStatus('idle'); setActionMsg('') }} style={{ width: '100%', padding: '6px', borderRadius: 6, border: '1px solid var(--line)', background: 'transparent', color: 'var(--ink3)', fontSize: 11, cursor: 'pointer', marginTop: 4 }}>Dismiss</button>
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
                <p style={{ fontSize: 10, color: 'var(--ink4)', marginBottom: 2 }}>id</p>
                <p style={{ fontSize: 10, fontFamily: MONO, color: 'var(--ink3)', wordBreak: 'break-all' }}>{node.id}</p>
              </div>
              {allMetaEntries.length === 0
                ? <p style={{ fontSize: 12, color: 'var(--ink4)' }}>No metadata</p>
                : allMetaEntries.map(([k, v]) => {
                  const str = typeof v === 'object' ? JSON.stringify(v, null, 2) : String(v ?? '')
                  const isLong = str.length > 38 || str.includes('\n')
                  return (
                    <div key={k} style={{ marginBottom: 8 }}>
                      <p style={{ fontSize: 10, color: 'var(--ink4)', marginBottom: 2 }}>{k}</p>
                      {isLong
                        ? <pre style={{ fontSize: 10, background: 'var(--bg)', color: 'var(--ink3)', border: '1px solid var(--line)', borderRadius: 5, padding: '5px 7px', fontFamily: MONO, whiteSpace: 'pre-wrap', wordBreak: 'break-all', margin: 0 }}>{str}</pre>
                        : <p style={{ fontSize: 11, fontFamily: MONO, color: 'var(--ink3)' }}>{str}</p>
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

const ICON_BTN: React.CSSProperties = { width: 28, height: 28, borderRadius: 7, border: 'none', background: 'transparent', color: 'var(--ink4)', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }
const TOOL_BTN: React.CSSProperties = { display: 'flex', alignItems: 'center', gap: 5, padding: '5px 11px', borderRadius: 6, fontSize: 11, fontWeight: 500, border: '1px solid var(--line)', background: 'var(--bg)', color: 'var(--ink3)', cursor: 'pointer' }
