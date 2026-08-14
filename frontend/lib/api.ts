import { SessionInfo } from '@/types'

// Fetches the machines connected to this hub. Same-origin: the UI-token
// cookie set on first page load authenticates the request.
export async function fetchSessions(): Promise<SessionInfo[]> {
  const res = await fetch('/api/sessions', { cache: 'no-store' })
  if (!res.ok) throw new Error(`sessions: HTTP ${res.status}`)
  return res.json()
}

export interface JoinInfo {
  joinUrl: string
  token: string
  caveat: string
}

// Fetches the address + join token other VMs use to join this hub.
// Empty joinUrl means the hub isn't reachable from other machines
// (e.g. --private) and the Add-machine flow should stay hidden.
export async function fetchJoinInfo(): Promise<JoinInfo> {
  const res = await fetch('/api/join-info', { cache: 'no-store' })
  if (!res.ok) throw new Error(`join-info: HTTP ${res.status}`)
  return res.json()
}

export interface ClusterContextOption {
  name: string
  serverUrl: string
  current: boolean
}

export interface ClusterEntry {
  id: string
  name: string
  context_name: string
  server_url?: string
  added_at: string
  online: boolean
  read_only?: boolean
}

// Uploads a kubeconfig. Omit `context` on the first call to get back the
// list of contexts found in the file for a picker; call again with a chosen
// `context` to actually create the cluster connection. readOnly, if true,
// blocks every write action and terminal session against this cluster
// specifically, independent of the server's global --read-only flag.
export async function addCluster(kubeconfig: string, name?: string, context?: string, readOnly?: boolean):
  Promise<{ contexts: ClusterContextOption[] } | ClusterEntry> {
  const res = await fetch('/api/clusters', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ kubeconfig, name, context, readOnly }),
  })
  if (!res.ok) throw new Error((await res.text()) || `clusters: HTTP ${res.status}`)
  return res.json()
}

export async function removeCluster(id: string): Promise<void> {
  const res = await fetch(`/api/clusters/${id}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 204) throw new Error(`clusters: HTTP ${res.status}`)
}

// Toggles a connected cluster's own read-only flag after the fact.
export async function setClusterReadOnly(id: string, readOnly: boolean): Promise<ClusterEntry> {
  const res = await fetch(`/api/clusters/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ readOnly }),
  })
  if (!res.ok) throw new Error((await res.text()) || `clusters: HTTP ${res.status}`)
  return res.json()
}

export interface PermissionPreview {
  canView: boolean
  canViewSecrets: boolean
  canExec: boolean
  canRestartOrKill: boolean
  canScaleOrEdit: boolean
  warnings?: string[]
}

// Checks what a kubeconfig context can actually do before connecting it —
// nothing is persisted, no cluster is added. Safe to call as many times as
// you like while picking a context.
export async function previewClusterPermissions(kubeconfig: string, context: string): Promise<PermissionPreview> {
  const res = await fetch('/api/clusters/preview', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ kubeconfig, context }),
  })
  if (!res.ok) throw new Error((await res.text()) || `clusters/preview: HTTP ${res.status}`)
  return res.json()
}

export interface AuditEntry {
  timestamp: string
  event: 'action_requested' | 'action_completed' | 'exec_requested'
  action_id?: string
  type?: string
  machine_id?: string
  hostname?: string
  entity_id?: string
  namespace?: string
  success?: boolean
  message?: string
}

export async function fetchAuditLog(limit = 200): Promise<AuditEntry[]> {
  const res = await fetch(`/api/audit?limit=${limit}`, { cache: 'no-store' })
  if (!res.ok) throw new Error(`audit: HTTP ${res.status}`)
  return res.json()
}
