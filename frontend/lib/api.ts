import { SessionInfo } from '@/types'

// Fetches the machines connected to this hub. Same-origin: the UI-token
// cookie set on first page load authenticates the request.
export async function fetchSessions(): Promise<SessionInfo[]> {
  const res = await fetch('/api/sessions', { cache: 'no-store' })
  if (!res.ok) throw new Error(`sessions: HTTP ${res.status}`)
  return res.json()
}
