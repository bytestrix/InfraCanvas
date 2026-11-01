'use client'

import { useState, useEffect, useRef } from 'react'
import {
  X, RotateCw, Square, Play, Tag, Layers, Trash2, Undo2, Download,
  CheckCircle2, XCircle, Loader2, ChevronDown, ChevronRight,
  AlertTriangle, FileText, Terminal, Eye, EyeOff, KeyRound, Pencil,
  ShieldOff, Shield, ArrowRightLeft, Search, Activity, Copy,
  type LucideIcon,
} from 'lucide-react'
import { type GraphNode } from '@/types'
import NodeSvgIcon from './NodeSvgIcon'
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
  type?: 'text' | 'number' | 'select' | 'textarea' | 'password'
  defaultValue?: (node: GraphNode) => string
  // Optional helper text rendered under the field.
  hint?: (node: GraphNode) => string
  // For 'select' fields. Returning an empty list falls back to a text input so
  // the action stays usable even when container metadata is missing.
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

// Resolve the real k8s resource name. Some node types (service, ingress) only set
// node.label = name and never metadata.name, so n.id (e.g. "service/ns/name") must
// never be used as the name — fall back to label, then the last id segment.
function k8sName(n: any): string {
  return n.metadata?.name ?? n.label ?? String(n.id ?? '').split('/').pop() ?? ''
}
function k8sTarget(entityType: string, n: any, namespace?: string) {
  return { layer: 'kubernetes', entity_type: entityType, entity_id: k8sName(n), namespace: namespace ?? n.metadata?.namespace ?? 'default' }
}
function dockerTarget(n: any) {
  return { layer: 'docker', entity_type: 'container', entity_id: n.id }
}
function nodeTarget(n: any) {
  return { layer: 'kubernetes', entity_type: 'node', entity_id: k8sName(n) }
}
function serviceTarget(n: any) {
  return { layer: 'host', entity_type: 'service', entity_id: String(n.metadata?.unit ?? n.id) }
}
function processTarget(n: any) {
  return { layer: 'host', entity_type: 'process', entity_id: String(n.metadata?.pid ?? n.id) }
}

// Existing env var KEYS for a workload's containers. Values are redacted at
// discovery (never leave the host), so we can only show names as a reference.
function existingEnvKeys(n: any, containerName?: string): string[] {
  const cs = n.metadata?.containers ?? []
  const keys = new Set<string>()
  for (const c of cs) {
    if (containerName && c.name !== containerName) continue
    for (const k of (c.envKeys ?? [])) if (k) keys.add(String(k))
  }
  return [...keys]
}
function envHint(n: any): string {
  const keys = existingEnvKeys(n)
  const base = 'Adds or updates variables. Existing ones are kept (never deleted). Values are hidden for security.'
  return keys.length ? `${base} Existing: ${keys.join(', ')}` : base
}

// Action ids that only read state (no mutation, no input) — auto-run on click.
const READONLY_IDS = new Set(['get_yaml', 'get_events', 'inspect', 'get', 'disk_usage', 'top_processes', 'status'])
function isReadonly(a: ActionDef): boolean {
  return !a.form && !a.confirm && !a.danger && READONLY_IDS.has(a.id)
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
    { id: 'rename', label: 'Rename', Icon: Pencil,
      form: [{ key: 'name', label: 'New Name', placeholder: 'my-container' }],
      buildPayload: (n, v) => ({ action_id: `ren-${Date.now()}`, type: 'docker_rename_container', target: dockerTarget(n), parameters: { name: v.name } }) },
    { id: 'inspect', label: 'Inspect', Icon: Search,
      buildPayload: (n) => ({ action_id: `inspect-${Date.now()}`, type: 'docker_inspect_container', target: dockerTarget(n), parameters: {} }) },
    { id: 'remove', label: 'Remove', Icon: Trash2, danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `rm-${Date.now()}`, type: 'docker_remove_container', target: dockerTarget(n), parameters: { force: 'true' } }) },
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
    { id: 'set_env', label: 'Set Env Vars', Icon: Pencil,
      form: [
        { key: 'container', label: 'Container (blank = all)', placeholder: '', defaultValue: (n) => n.metadata?.containers?.[0]?.name ?? '' },
        { key: 'env', label: 'Env Vars (KEY=VAL per line)', type: 'textarea', placeholder: 'MY_VAR=value\nDB_HOST=localhost', hint: envHint },
      ],
      buildPayload: (n, v) => ({ action_id: `env-${Date.now()}`, type: 'k8s_set_env', target: k8sTarget('deployment', n), parameters: { kind: 'deployment', container: v.container, env: v.env } }) },
    { id: 'get_events', label: 'Events', Icon: AlertTriangle,
      buildPayload: (n) => ({ action_id: `ev-${Date.now()}`, type: 'k8s_get_events', target: k8sTarget('deployment', n), parameters: { kind: 'Deployment' } }) },
    { id: 'get_yaml', label: 'View YAML', Icon: FileText,
      buildPayload: (n) => ({ action_id: `yaml-${Date.now()}`, type: 'k8s_get_yaml', target: k8sTarget('deployment', n), parameters: { kind: 'deployment' } }) },
    { id: 'delete', label: 'Delete', Icon: Trash2, danger: true, confirm: true,
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
    { id: 'set_env', label: 'Set Env Vars', Icon: Pencil,
      form: [
        { key: 'container', label: 'Container (blank = all)', placeholder: '', defaultValue: (n) => n.metadata?.containers?.[0]?.name ?? '' },
        { key: 'env', label: 'Env Vars (KEY=VAL per line)', type: 'textarea', placeholder: 'MY_VAR=value\nDB_HOST=localhost', hint: envHint },
      ],
      buildPayload: (n, v) => ({ action_id: `env-${Date.now()}`, type: 'k8s_set_env', target: k8sTarget('statefulset', n), parameters: { kind: 'statefulset', container: v.container, env: v.env } }) },
    { id: 'get_events', label: 'Events', Icon: AlertTriangle,
      buildPayload: (n) => ({ action_id: `ev-${Date.now()}`, type: 'k8s_get_events', target: k8sTarget('statefulset', n), parameters: { kind: 'StatefulSet' } }) },
    { id: 'get_yaml', label: 'View YAML', Icon: FileText,
      buildPayload: (n) => ({ action_id: `yaml-${Date.now()}`, type: 'k8s_get_yaml', target: k8sTarget('statefulset', n), parameters: { kind: 'statefulset' } }) },
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
    { id: 'set_env', label: 'Set Env Vars', Icon: Pencil,
      form: [
        { key: 'container', label: 'Container (blank = all)', placeholder: '' },
        { key: 'env', label: 'Env Vars (KEY=VAL per line)', type: 'textarea', placeholder: 'MY_VAR=value', hint: envHint },
      ],
      buildPayload: (n, v) => ({ action_id: `env-${Date.now()}`, type: 'k8s_set_env', target: k8sTarget('daemonset', n), parameters: { kind: 'daemonset', container: v.container, env: v.env } }) },
    { id: 'get_events', label: 'Events', Icon: AlertTriangle,
      buildPayload: (n) => ({ action_id: `ev-${Date.now()}`, type: 'k8s_get_events', target: k8sTarget('daemonset', n), parameters: { kind: 'DaemonSet' } }) },
    { id: 'get_yaml', label: 'View YAML', Icon: FileText,
      buildPayload: (n) => ({ action_id: `yaml-${Date.now()}`, type: 'k8s_get_yaml', target: k8sTarget('daemonset', n), parameters: { kind: 'daemonset' } }) },
  ],
  pod: [
    { id: 'get_events', label: 'Events', Icon: AlertTriangle,
      buildPayload: (n) => ({ action_id: `ev-${Date.now()}`, type: 'k8s_get_events', target: k8sTarget('pod', n), parameters: { kind: 'Pod' } }) },
    { id: 'get_yaml', label: 'View YAML', Icon: FileText,
      buildPayload: (n) => ({ action_id: `yaml-${Date.now()}`, type: 'k8s_get_yaml', target: k8sTarget('pod', n), parameters: { kind: 'pod' } }) },
    { id: 'delete', label: 'Delete / Restart', Icon: Trash2, danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `del-${Date.now()}`, type: 'k8s_delete_pod', target: k8sTarget('pod', n), parameters: {} }) },
    { id: 'port_forward', label: 'Port Forward', Icon: ArrowRightLeft,
      form: [
        { key: 'remote_port', label: 'Pod Port', placeholder: '8080', type: 'number' },
        { key: 'local_port', label: 'Local Port (0=auto)', placeholder: '0', type: 'number', defaultValue: () => '0' },
      ],
      buildPayload: (n, v) => ({ action_id: `pf-${Date.now()}`, type: 'k8s_port_forward', target: k8sTarget('pod', n), parameters: { remote_port: v.remote_port, local_port: v.local_port || '0' } }) },
  ],
  node: [
    { id: 'get_events', label: 'Events', Icon: AlertTriangle,
      buildPayload: (n) => ({ action_id: `ev-${Date.now()}`, type: 'k8s_get_events', target: nodeTarget(n), parameters: { kind: 'Node' } }) },
    { id: 'get_yaml', label: 'View YAML', Icon: FileText,
      buildPayload: (n) => ({ action_id: `yaml-${Date.now()}`, type: 'k8s_get_yaml', target: { layer: 'kubernetes', entity_type: 'node', entity_id: k8sName(n), namespace: '_cluster' }, parameters: { kind: 'node' } }) },
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
    { id: 'get_yaml', label: 'View YAML', Icon: FileText,
      buildPayload: (n) => ({ action_id: `yaml-${Date.now()}`, type: 'k8s_get_yaml', target: k8sTarget('service', n), parameters: { kind: 'service' } }) },
    { id: 'delete', label: 'Delete Service', Icon: Trash2, danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `del-${Date.now()}`, type: 'k8s_delete_service', target: k8sTarget('service', n), parameters: {} }) },
  ],
  configmap: [
    { id: 'get', label: 'View Data', Icon: Search,
      buildPayload: (n) => ({ action_id: `get-cm-${Date.now()}`, type: 'k8s_get_configmap', target: k8sTarget('configmap', n), parameters: {} }) },
    { id: 'edit_yaml', label: 'Edit YAML', Icon: FileText,
      form: [{ key: 'manifest', label: 'YAML', type: 'textarea', placeholder: 'apiVersion: v1\nkind: ConfigMap\n...' }],
      buildPayload: (n, v) => ({ action_id: `edit-cm-${Date.now()}`, type: 'k8s_apply_manifest', target: k8sTarget('configmap', n), parameters: { manifest: v.manifest } }) },
    { id: 'update', label: 'Set Key', Icon: Pencil,
      form: [
        { key: 'key', label: 'Key', placeholder: 'MY_CONFIG_KEY' },
        { key: 'value', label: 'Value', placeholder: 'new-value' },
      ],
      buildPayload: (n, v) => ({ action_id: `upd-cm-${Date.now()}`, type: 'k8s_update_configmap', target: k8sTarget('configmap', n), parameters: { [v.key]: v.value } }) },
  ],
  secret: [
    { id: 'get', label: 'View Keys', Icon: KeyRound,
      buildPayload: (n) => ({ action_id: `get-sec-${Date.now()}`, type: 'k8s_get_secret', target: k8sTarget('secret', n), parameters: {} }) },
    { id: 'edit_yaml', label: 'Edit YAML', Icon: FileText,
      form: [{ key: 'manifest', label: 'YAML', type: 'textarea', placeholder: 'apiVersion: v1\nkind: Secret\n...' }],
      buildPayload: (n, v) => ({ action_id: `edit-sec-${Date.now()}`, type: 'k8s_apply_manifest', target: k8sTarget('secret', n), parameters: { manifest: v.manifest } }) },
    { id: 'update', label: 'Set Key', Icon: Pencil,
      form: [
        { key: 'key', label: 'Key', placeholder: 'MY_SECRET_KEY' },
        { key: 'value', label: 'Value', placeholder: 'new-secret-value' },
      ],
      buildPayload: (n, v) => ({ action_id: `upd-sec-${Date.now()}`, type: 'k8s_update_secret', target: k8sTarget('secret', n), parameters: { [v.key]: v.value } }) },
  ],
  cluster: [
    { id: 'create_deployment', label: 'Create Deployment', Icon: Play,
      form: [
        { key: 'name', label: 'Name', placeholder: 'my-app' },
        { key: 'image', label: 'Image:Tag', placeholder: 'nginx:latest' },
        { key: 'replicas', label: 'Replicas', type: 'number', placeholder: '1', defaultValue: () => '1' },
        { key: 'port', label: 'Container Port (optional)', placeholder: '8080' },
        { key: 'env', label: 'Env Vars (KEY=VAL per line)', type: 'textarea', placeholder: 'MY_VAR=value', hint: envHint },
      ],
      buildPayload: (n, v) => ({ action_id: `create-dep-${Date.now()}`, type: 'k8s_create_deployment', target: { layer: 'kubernetes', entity_type: 'cluster', entity_id: n.metadata?.name ?? n.id, namespace: n.metadata?.namespace ?? 'default' }, parameters: { name: v.name, image: v.image, replicas: v.replicas || '1', port: v.port, env: v.env } }) },
    { id: 'apply_manifest', label: 'Apply Manifest', Icon: FileText,
      form: [{ key: 'manifest', label: 'YAML Manifest', type: 'textarea', placeholder: 'apiVersion: apps/v1\nkind: Deployment\n...' }],
      buildPayload: (n, v) => ({ action_id: `apply-${Date.now()}`, type: 'k8s_apply_manifest', target: { layer: 'kubernetes', entity_type: 'cluster', entity_id: n.metadata?.name ?? n.id, namespace: 'default' }, parameters: { manifest: v.manifest } }) },
  ],
  host: [
    { id: 'create_container', label: 'Run Container', Icon: Play,
      form: [
        { key: 'image', label: 'Image:Tag', placeholder: 'nginx:latest' },
        { key: 'name', label: 'Container Name (optional)', placeholder: 'my-nginx' },
        { key: 'ports', label: 'Port Mappings (host:container,...)', placeholder: '8080:80,443:443' },
        { key: 'env', label: 'Env Vars (KEY=VAL per line)', type: 'textarea', placeholder: 'MY_VAR=value', hint: envHint },
        { key: 'volumes', label: 'Volumes (host:container,...)', placeholder: '/data:/app/data' },
        { key: 'restart', label: 'Restart Policy', type: 'select', defaultValue: () => 'unless-stopped',
          options: () => [
            { value: 'unless-stopped', label: 'unless-stopped' },
            { value: 'always', label: 'always' },
            { value: 'on-failure', label: 'on-failure' },
            { value: 'no', label: 'no' },
          ] },
      ],
      buildPayload: (n, v) => ({ action_id: `create-ctr-${Date.now()}`, type: 'docker_create_container', target: { layer: 'docker', entity_type: 'host', entity_id: n.id }, parameters: v }) },
    { id: 'disk_usage', label: 'Disk Usage', Icon: Layers,
      buildPayload: (n) => ({ action_id: `disk-${Date.now()}`, type: 'host_disk_usage', target: { layer: 'host', entity_type: 'host', entity_id: n.id }, parameters: {} }) },
    { id: 'top_processes', label: 'Top Processes', Icon: Layers,
      buildPayload: (n) => ({ action_id: `proc-${Date.now()}`, type: 'host_top_processes', target: { layer: 'host', entity_type: 'host', entity_id: n.id }, parameters: {} }) },
    { id: 'run_cmd', label: 'Run Command', Icon: Terminal,
      form: [{ key: 'command', label: 'Shell Command', placeholder: 'df -h' }],
      buildPayload: (n, v) => ({ action_id: `cmd-${Date.now()}`, type: 'host_run_command', target: { layer: 'host', entity_type: 'host', entity_id: n.id }, parameters: { command: v.command } }) },
  ],
  service: [
    { id: 'restart', label: 'Restart', Icon: RotateCw, confirm: true,
      buildPayload: (n) => ({ action_id: `svc-restart-${Date.now()}`, type: 'service_restart', target: serviceTarget(n), parameters: {} }) },
    { id: 'stop', label: 'Stop', Icon: Square, danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `svc-stop-${Date.now()}`, type: 'service_stop', target: serviceTarget(n), parameters: {} }) },
    { id: 'start', label: 'Start', Icon: Play,
      buildPayload: (n) => ({ action_id: `svc-start-${Date.now()}`, type: 'service_start', target: serviceTarget(n), parameters: {} }) },
    { id: 'status', label: 'Status', Icon: Activity,
      buildPayload: (n) => ({ action_id: `svc-status-${Date.now()}`, type: 'host_service_status', target: serviceTarget(n), parameters: { service: String(n.metadata?.unit ?? n.label) } }) },
  ],
  process: [
    { id: 'terminate', label: 'Terminate (SIGTERM)', Icon: Square, confirm: true,
      buildPayload: (n) => ({ action_id: `proc-term-${Date.now()}`, type: 'process_kill', target: processTarget(n), parameters: { signal: 'TERM' } }) },
    { id: 'kill', label: 'Force Kill (SIGKILL)', Icon: Trash2, danger: true, confirm: true,
      buildPayload: (n) => ({ action_id: `proc-kill-${Date.now()}`, type: 'process_kill', target: processTarget(n), parameters: { signal: 'KILL' } }) },
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
  service:     ['unit', 'status', 'enabled', 'ports', 'mainPid', 'restartCount', 'description'],
  process:     ['pid', 'user', 'ports', 'processType', 'memoryBytes', 'commandLine'],
  k8s_service: ['namespace', 'type', 'cluster_ip', 'ports'],
  ingress:     ['namespace', 'host', 'tls'],
  pvc:         ['namespace', 'storage_class', 'capacity', 'access_modes'],
  volume:      ['driver', 'mountpoint'],
  network:     ['driver', 'scope', 'subnet'],
  cluster:     ['version', 'node_count'],
  job:         ['namespace', 'completions', 'active', 'succeeded'],
  cronjob:     ['namespace', 'schedule', 'last_run'],
  configmap:   ['namespace', 'keys'],
  secret:      ['namespace', 'type', 'keys'],
}

const HEALTH_COLOR: Record<string, string> = {
  healthy: '#22c55e', degraded: '#f59e0b', unhealthy: '#ef4444', unknown: '#6b7280',
}

const MONO = "var(--font-geist-mono,'Geist Mono',ui-monospace,monospace)"
const BW = {
  bg: 'var(--bg)', surface: 'var(--surface)', line: 'var(--line)', line2: 'var(--line2)',
  ink: 'var(--ink)', ink2: 'var(--ink2)', ink3: 'var(--ink3)', ink4: 'var(--ink4)',
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
      style={{ width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 16px', background: 'transparent', border: 'none', borderBottom: open ? `1px solid ${BW.line}` : 'none', cursor: 'pointer', color: BW.ink4 }}
      onMouseEnter={(e) => { e.currentTarget.style.background = BW.surface }}
      onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        <span style={{ fontSize: 10, fontWeight: 600, letterSpacing: '0.08em', textTransform: 'uppercase' as const, color: BW.ink4 }}>{title}</span>
        {count != null && <span style={{ fontSize: 9, padding: '1px 5px', borderRadius: 10, background: BW.line, color: BW.ink3 }}>{count}</span>}
      </div>
      {open ? <ChevronDown size={10} /> : <ChevronRight size={10} />}
    </button>
  )
}

function EnvVarsPanel({ env }: { env: Record<string, string> }) {
  const [showSecrets, setShowSecrets] = useState(false)
  const entries = Object.entries(env)
  if (entries.length === 0) return <p style={{ fontSize: 11, color: BW.ink4, padding: '8px 16px' }}>No environment variables</p>

  return (
    <div style={{ padding: '6px 12px 10px' }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 6 }}>
        <button
          onClick={() => setShowSecrets(v => !v)}
          style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 10, color: BW.ink3, background: 'transparent', border: `1px solid ${BW.line}`, borderRadius: 4, padding: '2px 7px', cursor: 'pointer' }}
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
            <div key={k} style={{ display: 'flex', background: BW.bg, borderRadius: 4, overflow: 'hidden', marginBottom: 2 }}>
              <span style={{ fontSize: 10, fontFamily: MONO, color: BW.ink2, padding: '3px 6px', background: BW.surface, flexShrink: 0, maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={k}>{k}</span>
              <span style={{ fontSize: 10, fontFamily: MONO, color: isSecret && !showSecrets ? BW.line2 : BW.ink3, padding: '3px 6px', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={display}>{display}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function PortsPanel({ ports }: { ports: any[] }) {
  if (!ports || ports.length === 0) return <p style={{ fontSize: 11, color: BW.ink4, padding: '8px 16px' }}>No port mappings</p>
  return (
    <div style={{ padding: '8px 12px 10px' }}>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr auto 1fr', gap: '3px 8px', alignItems: 'center' }}>
        <span style={{ fontSize: 9, color: BW.ink4, fontWeight: 600 }}>HOST</span>
        <span />
        <span style={{ fontSize: 9, color: BW.ink4, fontWeight: 600 }}>CONTAINER</span>
        {ports.map((p, i) => (
          <>
            <span key={`h${i}`} style={{ fontSize: 11, fontFamily: MONO, color: BW.ink2, background: BW.surface, borderRadius: 4, padding: '2px 6px', textAlign: 'right' }}>
              {p.hostIP && p.hostIP !== '0.0.0.0' ? `${p.hostIP}:` : ''}{p.hostPort}
            </span>
            <span key={`a${i}`} style={{ fontSize: 9, color: BW.ink4, textAlign: 'center' }}>→</span>
            <span key={`c${i}`} style={{ fontSize: 11, fontFamily: MONO, color: BW.ink3, background: BW.surface, borderRadius: 4, padding: '2px 6px' }}>
              {p.containerPort}<span style={{ color: BW.ink4 }}>/{p.protocol}</span>
            </span>
          </>
        ))}
      </div>
    </div>
  )
}

function MountsPanel({ mounts }: { mounts: Array<{ source: string; destination: string; mode: string; type: string }> }) {
  if (!mounts || mounts.length === 0) return <p style={{ fontSize: 11, color: BW.ink4, padding: '8px 16px' }}>No mounts</p>
  return (
    <div style={{ padding: '8px 12px 10px', display: 'flex', flexDirection: 'column', gap: 5 }}>
      {mounts.map((m, i) => (
        <div key={i} style={{ background: BW.bg, borderRadius: 6, padding: '6px 8px', border: `1px solid ${BW.line}` }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 3 }}>
            <span style={{ fontSize: 9, padding: '1px 5px', borderRadius: 3, background: BW.line, color: BW.ink3, fontWeight: 600, fontFamily: MONO }}>{m.type}</span>
            <span style={{ fontSize: 9, color: m.mode === 'ro' ? BW.ink2 : BW.ink4 }}>{m.mode === 'ro' ? 'read-only' : 'rw'}</span>
          </div>
          <div style={{ fontSize: 10, fontFamily: MONO, color: BW.ink3, marginBottom: 2, wordBreak: 'break-all', lineHeight: 1.4 }}>{m.source || '(anonymous)'}</div>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 4 }}>
            <span style={{ fontSize: 9, color: BW.ink4, flexShrink: 0, paddingTop: 1 }}>→</span>
            <span style={{ fontSize: 10, fontFamily: MONO, color: BW.ink2, wordBreak: 'break-all', lineHeight: 1.4 }}>{m.destination}</span>
          </div>
        </div>
      ))}
    </div>
  )
}

function WorkloadContainersPanel({ containers }: { containers: any[] }) {
  if (!containers || containers.length === 0) return <p style={{ fontSize: 11, color: BW.ink4, padding: '8px 16px' }}>No containers</p>
  return (
    <div style={{ padding: '8px 12px 10px', display: 'flex', flexDirection: 'column', gap: 8 }}>
      {containers.map((c, i) => {
        const req = c.requests ?? {}
        const lim = c.limits ?? {}
        const ports: any[] = c.ports ?? []
        const envKeys: string[] = c.envKeys ?? []
        const envFrom: string[] = c.envFrom ?? []
        return (
          <div key={i} style={{ background: BW.bg, borderRadius: 6, padding: '8px 10px', border: `1px solid ${BW.line}` }}>
            <div style={{ marginBottom: 5 }}>
              <span style={{ fontSize: 11, fontWeight: 600, color: BW.ink }}>{c.name}</span>
            </div>
            <div style={{ fontSize: 10, fontFamily: MONO, color: BW.ink3, wordBreak: 'break-all', marginBottom: 6, padding: '3px 6px', background: BW.surface, borderRadius: 4 }} title={c.image}>{c.image}</div>
            {(req.cpu || req.memory || lim.cpu || lim.memory) && (
              <div style={{ display: 'grid', gridTemplateColumns: 'auto 1fr auto 1fr', gap: '2px 6px', fontSize: 10, marginBottom: 6 }}>
                <span style={{ color: BW.ink4 }}>req cpu</span><span style={{ fontFamily: MONO, color: BW.ink3 }}>{req.cpu ?? '—'}</span>
                <span style={{ color: BW.ink4 }}>req mem</span><span style={{ fontFamily: MONO, color: BW.ink3 }}>{req.memory ?? '—'}</span>
                <span style={{ color: BW.ink4 }}>lim cpu</span><span style={{ fontFamily: MONO, color: BW.ink3 }}>{lim.cpu ?? '—'}</span>
                <span style={{ color: BW.ink4 }}>lim mem</span><span style={{ fontFamily: MONO, color: BW.ink3 }}>{lim.memory ?? '—'}</span>
              </div>
            )}
            {ports.length > 0 && (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginBottom: 6 }}>
                {ports.map((p, pi) => (
                  <span key={pi} style={{ fontSize: 10, fontFamily: MONO, padding: '1px 5px', background: BW.line, color: BW.ink2, borderRadius: 3 }}>
                    {p.name ? `${p.name}:` : ''}{p.containerPort}/{p.protocol || 'TCP'}
                  </span>
                ))}
              </div>
            )}
            {envKeys.length > 0 && (
              <div style={{ fontSize: 10, color: BW.ink4, marginBottom: envFrom.length > 0 ? 4 : 0 }}>
                env: <span style={{ color: BW.ink3, fontFamily: MONO }}>{envKeys.slice(0, 6).join(', ')}{envKeys.length > 6 ? ` +${envKeys.length - 6}` : ''}</span>
              </div>
            )}
            {envFrom.length > 0 && (
              <div style={{ fontSize: 10, color: BW.ink4 }}>
                envFrom: <span style={{ color: BW.ink2, fontFamily: MONO }}>{envFrom.join(', ')}</span>
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
          <span style={{ fontSize: 10, color: BW.ink4 }}>{k}</span>
          <span style={{ fontSize: 11, fontFamily: MONO, color: BW.ink3, maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{v as string}</span>
        </div>
      ))}
      {usedBy.length > 0 && (
        <div style={{ marginTop: 4 }}>
          <p style={{ fontSize: 10, color: BW.ink4, marginBottom: 4 }}>Used by {usedBy.length} container{usedBy.length !== 1 ? 's' : ''}</p>
          {usedBy.slice(0, 5).map((id: string) => (
            <div key={id} style={{ fontSize: 10, fontFamily: MONO, color: BW.ink3, padding: '2px 6px', background: BW.bg, borderRadius: 3, marginBottom: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{id}</div>
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
  width?: number
}

type ActionStatus = 'idle' | 'confirming' | 'running' | 'success' | 'error'
const TOOLS_NODES = new Set(['container', 'pod', 'host', 'service', 'deployment', 'statefulset', 'daemonset'])

export default function NodeDetailPanel({ node, vmCode, onClose, onShowLogs, onShowTerminal, width = 340 }: NodeDetailPanelProps) {
  // OSS build has no auth layer; action affordances stay enabled and the
  // backend arbitrates.
  const userCanAct = true
  const hc = HEALTH_COLOR[node.health] ?? '#64748b'
  // LXC/LXD/Incus containers are discovered & visualized, but actions/terminal/logs
  // go through the Docker layer, which doesn't manage them. Gate those off.
  const ctrRuntime = node.type === 'container' ? (node.metadata?.runtime as string | undefined) : undefined
  const isLxdContainer = ctrRuntime === 'lxd' || ctrRuntime === 'incus'
  const actions = isLxdContainer ? [] : (ACTIONS[node.type] ?? [])

  // Resizable panel width — persisted so it sticks across sessions/nodes.
  const MIN_W = 320, MAX_W = 760
  const [panelWidth, setPanelWidth] = useState<number>(() => {
    if (typeof window === 'undefined') return width
    const saved = Number(window.localStorage.getItem('ic-panel-width'))
    return saved >= MIN_W && saved <= MAX_W ? saved : width
  })
  const draggingRef = useRef(false)
  function startResize(e: React.MouseEvent) {
    e.preventDefault()
    draggingRef.current = true
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
    const onMove = (ev: MouseEvent) => {
      if (!draggingRef.current) return
      // Panel is anchored to the right edge, so width grows as the cursor moves left.
      const w = Math.min(MAX_W, Math.max(MIN_W, window.innerWidth - ev.clientX))
      setPanelWidth(w)
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
  const panelWidthRef = useRef(panelWidth)
  useEffect(() => { panelWidthRef.current = panelWidth }, [panelWidth])

  const [activeActionId, setActiveActionId] = useState<string | null>(null)
  const [formValues, setFormValues] = useState<Record<string, string>>({})
  const [actionStatus, setActionStatus] = useState<ActionStatus>('idle')
  const [actionMsg, setActionMsg] = useState('')
  const [actionOutput, setActionOutput] = useState('')
  const [editedYaml, setEditedYaml] = useState<string | null>(null)
  const [applying, setApplying] = useState(false)
  const [confirmInput, setConfirmInput] = useState('')
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  // Mirror activeActionId into a ref so the (vmCode-scoped) result subscription
  // reads the current action, not the value captured when it subscribed.
  const activeActionIdRef = useRef<string | null>(null)
  useEffect(() => { activeActionIdRef.current = activeActionId }, [activeActionId])

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
    setActionOutput('')
    setEditedYaml(null)
    setApplying(false)
    setFormValues({})
  }, [node.id])

  useEffect(() => {
    const unsubResult = subscribeActionResult((data) => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current)
      const out = data.details?.output ?? data.output ?? ''
      // Backends sometimes send literal null/"null" for "no output" — don't render that.
      const trimmed = typeof out === 'string' ? out.trim() : out
      const clean = out == null || trimmed === '' || trimmed === 'null' || trimmed === 'undefined' ? '' : out
      setActionOutput(clean)
      setApplying(false)
      // Seed the editable YAML buffer on the initial View YAML fetch only.
      // After an Apply (activeActionId stays 'get_yaml'), keep the user's edits.
      if (activeActionIdRef.current === 'get_yaml') setEditedYaml(prev => prev === null ? clean : prev)
      if (data.success === false || data.status === 'failed') {
        setActionStatus('error')
        setActionMsg(data.error && data.error !== data.message ? `${data.message ?? 'Action failed'}: ${data.error}` : (data.message ?? data.error ?? 'Action failed'))
      } else {
        setActionStatus('success')
        // Rollout-style actions don't reflect instantly (k8s rolling update, service
        // restart). Tell the user the view will catch up so they don't think it failed.
        const ROLLOUT = new Set(['update_image', 'scale', 'restart', 'rollback', 'cordon', 'drain', 'create_deployment', 'apply_manifest'])
        const base = data.message ?? 'Done'
        setActionMsg(ROLLOUT.has(activeActionIdRef.current ?? '') ? `${base}, rolling out, the view updates in a few seconds.` : base)
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
    if (activeActionId === action.id) { setActiveActionId(null); setActionStatus('idle'); setActionMsg(''); setConfirmInput(''); return }
    const defaults: Record<string, string> = {}
    for (const f of action.form ?? []) defaults[f.key] = f.defaultValue ? f.defaultValue(node) : ''
    setFormValues(defaults)
    setActiveActionId(action.id)
    setActionMsg('')
    setConfirmInput('')
    setEditedYaml(null)
    // Read-only fetches (View YAML, Events, Inspect…) have no input and don't mutate —
    // run them immediately instead of making the user click an "Apply" button.
    if (isReadonly(action)) { setActionStatus('running'); setActionMsg('Loading…'); sendAction(vmCode, action.buildPayload(node, defaults)); armTimeout(); return }
    setActionStatus(action.confirm ? 'confirming' : 'idle')
  }

  function armTimeout() {
    timeoutRef.current = setTimeout(() => { setActionStatus('error'); setActionMsg('No response from agent, check agent logs') }, 60_000)
  }

  function handleSubmit(action: ActionDef) {
    setActionStatus('running')
    setActionMsg('Sending…')
    sendAction(vmCode, action.buildPayload(node, formValues))
    armTimeout()
  }

  // Apply edits made in the View YAML editor. applyManifest reads the
  // namespace/name from the manifest itself, so we only need to send the text.
  function applyYaml() {
    if (!editedYaml || !editedYaml.trim()) return
    setApplying(true)
    setActionStatus('running')
    setActionMsg('Applying…')
    sendAction(vmCode, {
      action_id: `apply-${Date.now()}`,
      type: 'k8s_apply_manifest',
      target: { layer: 'kubernetes', entity_type: node.type, entity_id: k8sName(node), namespace: node.metadata?.namespace ?? 'default' },
      parameters: { manifest: editedYaml },
    })
    armTimeout()
  }

  // Derive inspection data from metadata (backend uses camelCase)
  const env: Record<string, string>   = node.metadata.environment ?? {}
  const ports: any[]                  = node.metadata.portMappings ?? []
  const mounts: any[]                 = node.metadata.mounts ?? []
  const envCount                      = Object.keys(env).length
  const keyFields                     = KEY_META_FIELDS[node.type] ?? []
  const keyMeta                       = keyFields.filter((k) => node.metadata[k] != null && node.metadata[k] !== '').map((k) => [k, node.metadata[k]] as [string, any])
  const allMetaEntries                = Object.entries(node.metadata)

  // Architect insights — contextual warnings/hints based on metadata
  const insights: Array<{ level: 'warn' | 'info'; msg: string }> = []
  if (node.type === 'pod' || node.type === 'container') {
    const rc = Number(node.metadata.restartCount ?? node.metadata.restart_count ?? 0)
    if (rc >= 5) insights.push({ level: 'warn', msg: `${rc} restarts, likely CrashLoopBackOff. Check logs.` })
    else if (rc > 0) insights.push({ level: 'info', msg: `${rc} restart${rc > 1 ? 's' : ''} since creation.` })
  }
  if (node.type === 'deployment') {
    const desired = Number(node.metadata.replicas ?? 0)
    const ready = Number(node.metadata.readyReplicas ?? node.metadata.ready_replicas ?? 0)
    if (desired > 0 && ready < desired) insights.push({ level: 'warn', msg: `${ready}/${desired} replicas ready, rollout incomplete or degraded.` })
    if (desired === 0) insights.push({ level: 'warn', msg: 'Scaled to 0, deployment is inactive.' })
  }
  if (node.type === 'node') {
    const conditions: any[] = node.metadata.conditions ?? []
    const notReady = conditions.find((c: any) => c.type === 'Ready' && c.status !== 'True')
    if (notReady) insights.push({ level: 'warn', msg: 'Node NotReady, workloads may be evicted.' })
    const pressure = conditions.find((c: any) => ['MemoryPressure','DiskPressure','PIDPressure'].includes(c.type) && c.status === 'True')
    if (pressure) insights.push({ level: 'warn', msg: `${pressure.type} detected, risk of OOM eviction.` })
  }
  if (node.type === 'pvc' && node.metadata.status !== 'Bound') {
    insights.push({ level: 'warn', msg: `PVC ${node.metadata.status ?? 'unbound'}, pods may be stuck Pending.` })
  }
  if (node.type === 'job' && Number(node.metadata.failed ?? 0) > 0) {
    insights.push({ level: 'warn', msg: `${node.metadata.failed} failed pods, check backoff limit.` })
  }

  return (
    <div style={{ position: 'absolute', right: 0, top: 0, bottom: 0, width: `min(${panelWidth}px, 100%)`, background: BW.surface, borderLeft: `1px solid ${BW.line2}`, display: 'flex', flexDirection: 'column', zIndex: 30 }}>
      {/* Drag handle to resize the panel, like big-SaaS side panels. */}
      <div
        onMouseDown={startResize}
        title="Drag to resize"
        style={{ position: 'absolute', left: -3, top: 0, bottom: 0, width: 6, cursor: 'col-resize', zIndex: 40 }}
        onMouseEnter={(e) => { e.currentTarget.style.background = BW.line2 }}
        onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
      />

      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', padding: '12px 16px', borderBottom: `1px solid ${BW.line}`, flexShrink: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
          <div style={{ width: 32, height: 32, borderRadius: 8, background: BW.line, border: `1px solid ${BW.line2}`, display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }}><NodeSvgIcon type={node.type === 'container_runtime' && node.metadata?.runtime_type === 'podman' ? 'podman' : node.type} size={16} /></div>
          <div style={{ minWidth: 0 }}>
            <p style={{ fontSize: 13, fontWeight: 600, color: BW.ink, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', letterSpacing: '-0.015em' }} title={node.label}>{node.label}</p>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 3 }}>
              <span style={{ fontSize: 10, padding: '1px 6px', borderRadius: 4, background: BW.line, color: BW.ink3, border: `1px solid ${BW.line2}`, fontFamily: MONO }}>{node.type}</span>
              <span style={{ fontSize: 10, color: hc, display: 'flex', alignItems: 'center', gap: 3 }}>
                <span style={{ width: 4, height: 4, borderRadius: '50%', background: hc, display: 'inline-block' }} />{node.health}
              </span>
            </div>
          </div>
        </div>
        <button onClick={onClose} style={ICON_BTN}
          onMouseEnter={(e) => { e.currentTarget.style.background = BW.line; e.currentTarget.style.color = BW.ink2 }}
          onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = BW.ink4 }}>
          <X size={14} />
        </button>
      </div>

      {/* Body */}
      <div style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column' }}>

        {/* Architect insights */}
        {insights.length > 0 && (
          <div style={{ padding: '8px 14px', borderBottom: `1px solid ${BW.line}`, display: 'flex', flexDirection: 'column', gap: 5 }}>
            {insights.map((ins, i) => (
              <div key={i} style={{ display: 'flex', alignItems: 'flex-start', gap: 7, padding: '6px 9px', background: BW.bg, borderRadius: 6, border: `1px solid ${BW.line2}` }}>
                <AlertTriangle size={11} color={ins.level === 'warn' ? BW.ink2 : BW.ink3} style={{ flexShrink: 0, marginTop: 1 }} />
                <span style={{ fontSize: 11, color: ins.level === 'warn' ? BW.ink2 : BW.ink3, lineHeight: 1.4 }}>{ins.msg}</span>
              </div>
            ))}
          </div>
        )}

        {/* Key metadata */}
        {keyMeta.length > 0 && (
          <div style={{ padding: '10px 16px', borderBottom: `1px solid ${BW.line}` }}>
            {keyMeta.map(([k, v]) => {
              const str = v !== null && typeof v === 'object' ? JSON.stringify(v) : String(v ?? '')
              const display = k === 'size' ? formatBytes(Number(v)) : str
              const isPath = k === 'mountpoint' || k === 'image'
              return (
                <div key={k} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 8, marginBottom: 5 }}>
                  <span style={{ fontSize: 10, color: BW.ink4, flexShrink: 0 }}>{k}</span>
                  <span style={{ fontSize: 11, fontFamily: MONO, color: BW.ink3, textAlign: 'right', ...(isPath ? { wordBreak: 'break-all' as const, whiteSpace: 'normal' as const } : { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const, maxWidth: 190 }) }} title={str}>{display}</span>
                </div>
              )
            })}
          </div>
        )}

        {/* LXC/LXD note — actions not yet supported for this runtime */}
        {isLxdContainer && (
          <div style={{ padding: '10px 16px', borderBottom: `1px solid ${BW.line}` }}>
            <p style={{ fontSize: 11.5, color: BW.ink3, lineHeight: 1.5 }}>
              <span style={{ fontFamily: MONO, color: BW.ink2 }}>{ctrRuntime}</span> container, discovery only for now. Terminal, logs, and actions are coming soon.
            </p>
          </div>
        )}

        {/* Tools */}
        {!isLxdContainer && TOOLS_NODES.has(node.type) && (onShowLogs || onShowTerminal) && (
          <div style={{ padding: '10px 16px', borderBottom: `1px solid ${BW.line}` }}>
            <p style={{ fontSize: 10, fontWeight: 600, color: BW.ink4, letterSpacing: '0.08em', textTransform: 'uppercase', marginBottom: 8 }}>Tools</p>
            <div style={{ display: 'flex', gap: 6 }}>
              {onShowLogs && (
                <button onClick={onShowLogs} style={TOOL_BTN}
                  onMouseEnter={(e) => { e.currentTarget.style.borderColor = BW.line2; e.currentTarget.style.color = BW.ink; e.currentTarget.style.background = BW.line }}
                  onMouseLeave={(e) => { e.currentTarget.style.borderColor = BW.line; e.currentTarget.style.color = BW.ink3; e.currentTarget.style.background = BW.bg }}>
                  <FileText size={11} /> Logs
                </button>
              )}
              {onShowTerminal && userCanAct && (
                <button onClick={onShowTerminal} style={TOOL_BTN}
                  onMouseEnter={(e) => { e.currentTarget.style.borderColor = BW.line2; e.currentTarget.style.color = BW.ink; e.currentTarget.style.background = BW.line }}
                  onMouseLeave={(e) => { e.currentTarget.style.borderColor = BW.line; e.currentTarget.style.color = BW.ink3; e.currentTarget.style.background = BW.bg }}>
                  <Terminal size={11} /> Terminal
                </button>
              )}
            </div>
          </div>
        )}

        {/* Image details section */}
        {node.type === 'image' && (
          <div style={{ borderBottom: `1px solid ${BW.line}` }}>
            <SectionHeader title="Image Details" open={openImage} onToggle={() => setOpenImage(v => !v)} />
            {openImage && <ImageDetailsPanel node={node} />}
          </div>
        )}

        {/* Env vars — containers */}
        {node.type === 'container' && envCount > 0 && (
          <div style={{ borderBottom: `1px solid ${BW.line}` }}>
            <SectionHeader title="Environment" count={envCount} open={openEnv} onToggle={() => setOpenEnv(v => !v)} />
            {openEnv && <EnvVarsPanel env={env} />}
          </div>
        )}

        {/* Port mappings — containers */}
        {node.type === 'container' && (
          <div style={{ borderBottom: `1px solid ${BW.line}` }}>
            <SectionHeader title="Ports" count={ports.length} open={openPorts} onToggle={() => setOpenPorts(v => !v)} />
            {openPorts && <PortsPanel ports={ports} />}
          </div>
        )}

        {/* Mounts — containers */}
        {node.type === 'container' && (
          <div style={{ borderBottom: `1px solid ${BW.line}` }}>
            <SectionHeader title="Mounts" count={mounts.length} open={openMounts} onToggle={() => setOpenMounts(v => !v)} />
            {openMounts && <MountsPanel mounts={mounts} />}
          </div>
        )}

        {/* Containers — workloads */}
        {(node.type === 'deployment' || node.type === 'statefulset' || node.type === 'daemonset') && Array.isArray(node.metadata.containers) && node.metadata.containers.length > 0 && (
          <div style={{ borderBottom: `1px solid ${BW.line}` }}>
            <SectionHeader title="Containers" count={node.metadata.containers.length} open={openContainers} onToggle={() => setOpenContainers(v => !v)} />
            {openContainers && <WorkloadContainersPanel containers={node.metadata.containers} />}
          </div>
        )}

        {/* Actions */}
        {actions.length > 0 && userCanAct && (
          <div style={{ padding: '12px 16px', borderBottom: `1px solid ${BW.line}` }}>
            <p style={{ fontSize: 10, fontWeight: 600, color: BW.ink4, letterSpacing: '0.08em', textTransform: 'uppercase', marginBottom: 10 }}>Actions</p>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: activeActionId ? 12 : 0 }}>
              {actions.map((action) => {
                const isOpen = activeActionId === action.id
                const dangerActive = action.danger && isOpen
                return (
                  <button key={action.id} onClick={() => openAction(action)}
                    style={{ display: 'flex', alignItems: 'center', gap: 5, padding: '5px 11px', borderRadius: 6, fontSize: 11, fontWeight: 500, border: `1px solid ${isOpen ? BW.line2 : BW.line}`, background: isOpen ? BW.line : BW.bg, color: isOpen ? BW.ink : BW.ink3, cursor: 'pointer', transition: 'all 0.1s' }}
                    onMouseEnter={(e) => { if (!isOpen) { e.currentTarget.style.borderColor = BW.line2; e.currentTarget.style.color = BW.ink; e.currentTarget.style.background = BW.line } }}
                    onMouseLeave={(e) => { if (!isOpen) { e.currentTarget.style.borderColor = BW.line; e.currentTarget.style.color = BW.ink3; e.currentTarget.style.background = BW.bg } }}
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
                <div style={{ background: BW.bg, border: `1px solid ${BW.line}`, borderRadius: 8, padding: 12 }}>
                  {action.form && actionStatus === 'idle' && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 12 }}>
                      {action.form.map((field) => {
                        const baseStyle: React.CSSProperties = { width: '100%', boxSizing: 'border-box' as const, background: BW.surface, border: `1px solid ${BW.line}`, borderRadius: 6, padding: '6px 10px', fontSize: 12, fontFamily: MONO, color: BW.ink, outline: 'none' }
                        const opts = field.type === 'select' ? (field.options?.(node) ?? []) : []
                        const useSelect = field.type === 'select' && opts.length > 0
                        return (
                          <div key={field.key}>
                            <label style={{ fontSize: 10, color: BW.ink4, display: 'block', marginBottom: 4 }}>{field.label}</label>
                            {field.hint && <p style={{ fontSize: 10, color: BW.ink4, margin: '0 0 5px', lineHeight: 1.4 }}>{field.hint(node)}</p>}
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
                            ) : field.type === 'textarea' ? (
                              <textarea
                                value={formValues[field.key] ?? ''}
                                onChange={(e) => setFormValues((prev) => ({ ...prev, [field.key]: e.target.value }))}
                                placeholder={field.placeholder}
                                rows={4}
                                style={{ ...baseStyle, resize: 'vertical', lineHeight: 1.5 }}
                                onFocus={(e) => { e.currentTarget.style.borderColor = BW.line2 }}
                                onBlur={(e) => { e.currentTarget.style.borderColor = BW.line }}
                              />
                            ) : (
                              <input
                                type={field.type === 'number' ? 'number' : 'text'}
                                value={formValues[field.key] ?? ''}
                                onChange={(e) => setFormValues((prev) => ({ ...prev, [field.key]: e.target.value }))}
                                placeholder={field.placeholder}
                                style={baseStyle}
                                onFocus={(e) => { e.currentTarget.style.borderColor = BW.line2 }}
                                onBlur={(e) => { e.currentTarget.style.borderColor = BW.line }}
                              />
                            )}
                          </div>
                        )
                      })}
                    </div>
                  )}
                  {action.confirm && actionStatus === 'confirming' && (
                    <div style={{ marginBottom: 10 }}>
                      {action.danger ? (
                        <div style={{ padding: '10px 12px', borderRadius: 7, background: 'rgba(239,68,68,0.06)', border: '1px solid rgba(239,68,68,0.2)', marginBottom: 8 }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
                            <AlertTriangle size={11} color="#ef4444" />
                            <span style={{ fontSize: 11, color: '#ef4444', fontWeight: 600 }}>Destructive action, cannot be undone</span>
                          </div>
                          <div style={{ fontSize: 10.5, color: BW.ink3, marginBottom: 8, lineHeight: 1.5 }}>
                            Type <span style={{ fontFamily: MONO, color: BW.ink, background: BW.line, padding: '1px 5px', borderRadius: 3 }}>{node.label}</span> to confirm
                          </div>
                          <input
                            value={confirmInput}
                            onChange={e => setConfirmInput(e.target.value)}
                            placeholder={node.label}
                            autoFocus
                            style={{ width: '100%', boxSizing: 'border-box', padding: '6px 9px', borderRadius: 6, fontSize: 11.5, background: BW.bg, border: `1px solid ${confirmInput === node.label ? 'rgba(239,68,68,0.5)' : BW.line2}`, color: BW.ink, outline: 'none', fontFamily: MONO }}
                          />
                        </div>
                      ) : (
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
                          <span style={{ fontSize: 11, color: BW.ink3 }}>Run &ldquo;{action.label}&rdquo; on {node.label}?</span>
                        </div>
                      )}
                    </div>
                  )}
                  {actionStatus === 'running' && <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}><Loader2 size={13} color={BW.ink2} style={{ animation: 'spin 1s linear infinite' }} /><span style={{ fontSize: 11, color: BW.ink3 }}>{actionMsg}</span></div>}
                  {actionStatus === 'success' && <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: actionOutput ? 8 : 10 }}><CheckCircle2 size={13} color={BW.ink} /><span style={{ fontSize: 11, color: BW.ink2 }}>{actionMsg}</span></div>}
                  {actionStatus === 'error' && <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8, marginBottom: 10, padding: '8px 10px', borderRadius: 6, background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.25)' }}><XCircle size={13} color="#ef4444" style={{ flexShrink: 0, marginTop: 1 }} /><span style={{ fontSize: 11, color: '#ef4444', wordBreak: 'break-word' }}>{actionMsg}</span></div>}
                  {(actionStatus === 'success' || actionStatus === 'error') && (actionOutput || (activeActionId === 'get_yaml' && editedYaml)) && (() => {
                    // Try to parse as JSON for structured display (events, inspect, yaml)
                    let parsed: any = null
                    try { parsed = JSON.parse(actionOutput) } catch {}
                    const activeAction = actions.find(a => a.id === activeActionId)
                    const isEvents = activeAction?.id === 'get_events' && Array.isArray(parsed)
                    const isYaml = activeAction?.id === 'get_yaml'
                    return (
                      <div style={{ marginBottom: 10 }}>
                        {isYaml ? (
                          <div>
                            <textarea
                              value={editedYaml ?? actionOutput}
                              onChange={(e) => setEditedYaml(e.target.value)}
                              spellCheck={false}
                              style={{ width: '100%', boxSizing: 'border-box', minHeight: 240, maxHeight: 360, resize: 'vertical', fontSize: 10.5, fontFamily: MONO, color: BW.ink2, background: BW.surface, border: `1px solid ${BW.line}`, borderRadius: 6, padding: '8px 10px', outline: 'none', lineHeight: 1.5 }}
                            />
                            <p style={{ fontSize: 10, color: BW.ink4, margin: '6px 0 8px' }}>Edit and apply to update the live resource. Changes take effect immediately and may trigger a rollout.</p>
                            <div style={{ display: 'flex', gap: 6 }}>
                              <button
                                onClick={applyYaml}
                                disabled={applying || !editedYaml || editedYaml === actionOutput}
                                style={{ flex: 1, padding: '7px', borderRadius: 6, border: `1px solid ${BW.ink}`, background: BW.ink, color: BW.bg, fontSize: 11.5, fontWeight: 500, cursor: (applying || !editedYaml || editedYaml === actionOutput) ? 'not-allowed' : 'pointer', opacity: (applying || !editedYaml || editedYaml === actionOutput) ? 0.45 : 1 }}>
                                {applying ? 'Applying…' : 'Apply Changes'}
                              </button>
                              <button
                                onClick={() => setEditedYaml(actionOutput)}
                                disabled={!editedYaml || editedYaml === actionOutput}
                                style={{ padding: '7px 12px', borderRadius: 6, border: `1px solid ${BW.line}`, background: 'transparent', color: BW.ink3, fontSize: 11.5, cursor: (!editedYaml || editedYaml === actionOutput) ? 'not-allowed' : 'pointer' }}>
                                Reset
                              </button>
                            </div>
                          </div>
                        ) : isEvents ? (
                          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, maxHeight: 220, overflowY: 'auto' }}>
                            {parsed.length === 0
                              ? <p style={{ fontSize: 11, color: BW.ink4 }}>No events found.</p>
                              : parsed.map((ev: any, i: number) => (
                                <div key={i} style={{ background: BW.surface, border: `1px solid ${ev.type === 'Warning' ? 'rgba(239,68,68,0.25)' : BW.line}`, borderRadius: 6, padding: '6px 10px' }}>
                                  <div style={{ display: 'flex', gap: 6, alignItems: 'center', marginBottom: 3 }}>
                                    <span style={{ fontSize: 9.5, fontFamily: MONO, padding: '1px 5px', borderRadius: 3, background: ev.type === 'Warning' ? 'rgba(239,68,68,0.12)' : 'rgba(34,197,94,0.1)', color: ev.type === 'Warning' ? '#ef4444' : '#22c55e' }}>{ev.type}</span>
                                    <span style={{ fontSize: 10, fontFamily: MONO, color: BW.ink2, fontWeight: 600 }}>{ev.reason}</span>
                                    <span style={{ fontSize: 10, fontFamily: MONO, color: BW.ink4, marginLeft: 'auto' }}>{ev.age}{ev.count > 1 ? ` ×${ev.count}` : ''}</span>
                                  </div>
                                  <p style={{ fontSize: 10.5, color: BW.ink3, margin: 0, lineHeight: 1.4, wordBreak: 'break-word' }}>{ev.message}</p>
                                </div>
                              ))
                            }
                          </div>
                        ) : (() => {
                          const isJson = parsed !== null && typeof parsed === 'object'
                          const content = isJson ? JSON.stringify(parsed, null, 2) : actionOutput
                          return (
                            <div style={{ position: 'relative' }}>
                              <button
                                onClick={() => navigator.clipboard?.writeText(content)}
                                title="Copy"
                                style={{ position: 'absolute', top: 6, right: 6, zIndex: 1, display: 'flex', alignItems: 'center', gap: 4, padding: '3px 7px', borderRadius: 5, border: `1px solid ${BW.line}`, background: BW.surface, color: BW.ink3, fontSize: 10, cursor: 'pointer' }}>
                                <Copy size={11} /> Copy
                              </button>
                              {/* Tabular output (ps, df, ls) keeps column alignment: no wrap,
                                  scroll horizontally. JSON/YAML wraps gently so it stays readable. */}
                              <pre style={{ margin: 0, fontSize: 10.5, fontFamily: MONO, color: BW.ink2, background: BW.bg, border: `1px solid ${BW.line}`, borderRadius: 6, padding: '8px 10px', paddingTop: 28, overflowX: 'auto', overflowY: 'auto', maxHeight: 320, whiteSpace: isJson ? 'pre-wrap' : 'pre', wordBreak: isJson ? 'break-word' : 'normal', lineHeight: 1.5 }}>
                                {content}
                              </pre>
                            </div>
                          )
                        })()}
                      </div>
                    )
                  })()}
                  {(actionStatus === 'idle' || actionStatus === 'confirming') && (() => {
                    const dangerBlocked = action.danger && actionStatus === 'confirming' && confirmInput !== node.label
                    return (
                      <div style={{ display: 'flex', gap: 6 }}>
                        <button
                          onClick={() => !dangerBlocked && handleSubmit(action)}
                          disabled={dangerBlocked}
                          style={{ flex: 1, padding: '6px 10px', borderRadius: 6, fontSize: 11, fontWeight: 600, cursor: dangerBlocked ? 'not-allowed' : 'pointer', border: 'none',
                            background: dangerBlocked ? BW.line : action.danger ? '#ef4444' : BW.ink,
                            color: dangerBlocked ? BW.ink4 : BW.bg,
                            opacity: dangerBlocked ? 0.5 : 1,
                          } as React.CSSProperties}>
                          {action.confirm ? `Confirm ${action.label}` : `Apply ${action.label}`}
                        </button>
                        <button onClick={() => { setActiveActionId(null); setActionStatus('idle'); setConfirmInput('') }} style={{ padding: '6px 10px', borderRadius: 6, border: `1px solid ${BW.line}`, background: 'transparent', color: BW.ink4, fontSize: 11, cursor: 'pointer' }}>Cancel</button>
                      </div>
                    )
                  })()}
                  {(actionStatus === 'success' || actionStatus === 'error') && (
                    <button onClick={() => { setActiveActionId(null); setActionStatus('idle'); setActionMsg('') }} style={{ width: '100%', padding: '6px', borderRadius: 6, border: `1px solid ${BW.line}`, background: 'transparent', color: BW.ink4, fontSize: 11, cursor: 'pointer', marginTop: 4 }}>Dismiss</button>
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
                <p style={{ fontSize: 10, color: BW.ink4, marginBottom: 2 }}>id</p>
                <p style={{ fontSize: 10, fontFamily: MONO, color: BW.ink3, wordBreak: 'break-all' }}>{node.id}</p>
              </div>
              {allMetaEntries.length === 0
                ? <p style={{ fontSize: 12, color: BW.ink4 }}>No metadata</p>
                : allMetaEntries.map(([k, v]) => {
                  const str = v !== null && typeof v === 'object' ? JSON.stringify(v, null, 2) : String(v ?? '')
                  const isLong = str.length > 38 || str.includes('\n')
                  return (
                    <div key={k} style={{ marginBottom: 8 }}>
                      <p style={{ fontSize: 10, color: BW.ink4, marginBottom: 2 }}>{k}</p>
                      {isLong
                        ? <pre style={{ fontSize: 10, background: BW.bg, color: BW.ink3, border: `1px solid ${BW.line}`, borderRadius: 5, padding: '5px 7px', fontFamily: MONO, whiteSpace: 'pre-wrap', wordBreak: 'break-all', margin: 0 }}>{str}</pre>
                        : <p style={{ fontSize: 11, fontFamily: MONO, color: BW.ink3 }}>{str}</p>
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

const ICON_BTN: React.CSSProperties = { width: 28, height: 28, borderRadius: 7, border: 'none', background: 'transparent', color: BW.ink4, cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }
const TOOL_BTN: React.CSSProperties = { display: 'flex', alignItems: 'center', gap: 5, padding: '5px 11px', borderRadius: 6, fontSize: 11, fontWeight: 500, border: `1px solid ${BW.line}`, background: BW.bg, color: BW.ink3, cursor: 'pointer', transition: 'all 0.1s', fontFamily: 'inherit' }
