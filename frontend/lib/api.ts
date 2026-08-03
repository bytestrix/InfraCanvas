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
