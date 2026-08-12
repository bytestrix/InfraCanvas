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
}

// Uploads a kubeconfig. Omit `context` on the first call to get back the
// list of contexts found in the file for a picker; call again with a chosen
// `context` to actually create the cluster connection.
export async function addCluster(kubeconfig: string, name?: string, context?: string):
  Promise<{ contexts: ClusterContextOption[] } | ClusterEntry> {
  const res = await fetch('/api/clusters', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ kubeconfig, name, context }),
  })
  if (!res.ok) throw new Error((await res.text()) || `clusters: HTTP ${res.status}`)
  return res.json()
}

export async function removeCluster(id: string): Promise<void> {
  const res = await fetch(`/api/clusters/${id}`, { method: 'DELETE' })
  if (!res.ok && res.status !== 204) throw new Error(`clusters: HTTP ${res.status}`)
}
