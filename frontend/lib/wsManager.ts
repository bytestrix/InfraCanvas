/**
 * WebSocket Manager — module-level singleton Map.
 * Lives outside React lifecycle to survive navigation.
 * Includes exponential-backoff reconnect and proper GRAPH_DIFF application.
 */

import { WsInbound, WsOutbound } from '@/types'
import { useVMStore } from '@/store/vmStore'

const WS_BASE = process.env.NEXT_PUBLIC_WS_URL ?? 'ws://localhost:8080'
const WS_URL = WS_BASE.replace(/\/+$/, '') + '/ws/canvas'

const RECONNECT_BASE_MS = 1_000
const RECONNECT_MAX_MS  = 30_000
const RECONNECT_JITTER  = 0.2 // ±20% jitter

interface SocketEntry {
  ws: WebSocket
  reconnectTimer: ReturnType<typeof setTimeout> | null
  attempt: number
  destroyed: boolean // true when user explicitly disconnects
  serverRejected: boolean // true when server sent ERROR (don't reconnect)
}

// Module-level sockets map: code → entry
const sockets = new Map<string, SocketEntry>()

function getStore() {
  return useVMStore.getState()
}

export function connectVM(code: string): void {
  const existing = sockets.get(code)
  if (existing) {
    if (existing.ws.readyState === WebSocket.OPEN) return
    existing.destroyed = true
    existing.ws.close()
    if (existing.reconnectTimer) clearTimeout(existing.reconnectTimer)
    sockets.delete(code)
  }

  getStore().addVM(code)
  _openSocket(code, 0)
}

function _openSocket(code: string, attempt: number): void {
  const entry: SocketEntry = {
    ws: new WebSocket(WS_URL),
    reconnectTimer: null,
    attempt,
    destroyed: false,
    serverRejected: false,
  }
  sockets.set(code, entry)

  entry.ws.onopen = () => {
    entry.attempt = 0 // reset backoff on success
    const msg: WsOutbound = { type: 'PAIR', data: { code } }
    entry.ws.send(JSON.stringify(msg))
    getStore().setVMStatus(code, 'paired')
  }

  entry.ws.onmessage = (event) => {
    let parsed: WsInbound
    try {
      parsed = JSON.parse(event.data)
    } catch {
      console.error('[wsManager] Failed to parse message', event.data)
      return
    }
    handleMessage(code, parsed)
  }

  entry.ws.onerror = () => {
    // onerror always fires before onclose — just let onclose handle recovery
    getStore().setVMError(code, 'Connection error — retrying...')
  }

  entry.ws.onclose = (event) => {
    const current = sockets.get(code)
    if (!current || current.destroyed) return // intentional close

    if (current.serverRejected) return // server already sent ERROR — don't reconnect

    if (event.code === 1000) {
      // Clean close from server side (e.g. session expired)
      getStore().setVMDisconnected(code)
      return
    }

    // Abnormal close — schedule reconnect with exponential backoff + jitter
    const delay = _backoffDelay(current.attempt)
    console.warn(`[wsManager] Disconnected (code=${event.code}) — reconnecting in ${delay}ms (attempt ${current.attempt + 1})`)
    getStore().setVMStatus(code, 'connecting')

    current.reconnectTimer = setTimeout(() => {
      const c = sockets.get(code)
      if (!c || c.destroyed) return
      sockets.delete(code)
      _openSocket(code, current.attempt + 1)
    }, delay)
  }
}

function _backoffDelay(attempt: number): number {
  const base = Math.min(RECONNECT_BASE_MS * Math.pow(2, attempt), RECONNECT_MAX_MS)
  const jitter = base * RECONNECT_JITTER * (Math.random() * 2 - 1)
  return Math.round(base + jitter)
}

function handleMessage(code: string, msg: WsInbound): void {
  const store = getStore()

  switch (msg.type) {
    case 'AGENT_CONNECTED':
      store.setVMConnected(code, msg.data.hostname, msg.data.scope)
      break

    case 'AGENT_DISCONNECTED':
      store.setVMDisconnected(code)
      break

    case 'GRAPH_SNAPSHOT':
      store.setVMGraph(code, msg.data)
      if (store.vms[code]?.status !== 'connected') {
        store.setVMStatus(code, 'connected')
      }
      break

    case 'GRAPH_DIFF':
      store.applyVMDiff(code, msg.data)
      break

    case 'ERROR': {
      // Mark as server-rejected so onclose won't loop
      const entry = sockets.get(code)
      if (entry) entry.serverRejected = true
      store.setVMError(code, msg.data.message)
      break
    }

    case 'ACTION_RESULT' as any:
      actionResultListeners.forEach((fn) => fn((msg as any).data))
      break

    case 'ACTION_PROGRESS' as any:
      actionProgressListeners.forEach((fn) => fn((msg as any).data))
      break

    case 'LOG_DATA' as any: {
      const d = (msg as any).data
      const listeners = logDataListeners.get(d.request_id)
      if (listeners) listeners.forEach((fn) => fn(d))
      break
    }

    case 'EXEC_DATA' as any: {
      const d = (msg as any).data
      const listeners = execDataListeners.get(d.session_id)
      if (listeners) listeners.forEach((fn) => fn(d))
      break
    }

    case 'EXEC_END' as any: {
      const d = (msg as any).data
      const listeners = execEndListeners.get(d.session_id)
      if (listeners) listeners.forEach((fn) => fn(d))
      break
    }

    default:
      console.warn('[wsManager] Unknown message type:', (msg as any).type)
  }
}

export function sendCommand(code: string, action: string): void {
  const entry = sockets.get(code)
  if (!entry || entry.ws.readyState !== WebSocket.OPEN) {
    console.warn('[wsManager] Cannot send command — socket not open for', code)
    return
  }
  const msg: WsOutbound = { type: 'COMMAND', data: { action } }
  entry.ws.send(JSON.stringify(msg))
}

export function sendAction(code: string, payload: object): void {
  const entry = sockets.get(code)
  if (!entry || entry.ws.readyState !== WebSocket.OPEN) {
    console.warn('[wsManager] Cannot send action — socket not open for', code)
    return
  }
  entry.ws.send(JSON.stringify({ type: 'BROWSER_ACTION', data: payload }))
}

// ── Action result / progress subscriptions ────────────────────────────────────

type ActionHandler = (data: any) => void

const actionResultListeners = new Set<ActionHandler>()
const actionProgressListeners = new Set<ActionHandler>()

/** Subscribe to ACTION_RESULT messages. Returns an unsubscribe function. */
export function subscribeActionResult(handler: ActionHandler): () => void {
  actionResultListeners.add(handler)
  return () => actionResultListeners.delete(handler)
}

/** Subscribe to ACTION_PROGRESS messages. Returns an unsubscribe function. */
export function subscribeActionProgress(handler: ActionHandler): () => void {
  actionProgressListeners.add(handler)
  return () => actionProgressListeners.delete(handler)
}

// ── Log data subscriptions ────────────────────────────────────────────────────

// requestID → set of handlers
const logDataListeners = new Map<string, Set<ActionHandler>>()

export function subscribeLogData(requestID: string, handler: ActionHandler): () => void {
  if (!logDataListeners.has(requestID)) logDataListeners.set(requestID, new Set())
  logDataListeners.get(requestID)!.add(handler)
  return () => {
    const s = logDataListeners.get(requestID)
    if (s) { s.delete(handler); if (s.size === 0) logDataListeners.delete(requestID) }
  }
}

// ── Exec session subscriptions ────────────────────────────────────────────────

const execDataListeners = new Map<string, Set<ActionHandler>>()
const execEndListeners  = new Map<string, Set<ActionHandler>>()

export function subscribeExecData(sessionID: string, handler: ActionHandler): () => void {
  if (!execDataListeners.has(sessionID)) execDataListeners.set(sessionID, new Set())
  execDataListeners.get(sessionID)!.add(handler)
  return () => {
    const s = execDataListeners.get(sessionID)
    if (s) { s.delete(handler); if (s.size === 0) execDataListeners.delete(sessionID) }
  }
}

export function subscribeExecEnd(sessionID: string, handler: ActionHandler): () => void {
  if (!execEndListeners.has(sessionID)) execEndListeners.set(sessionID, new Set())
  execEndListeners.get(sessionID)!.add(handler)
  return () => {
    const s = execEndListeners.get(sessionID)
    if (s) { s.delete(handler); if (s.size === 0) execEndListeners.delete(sessionID) }
  }
}

export function sendExecStart(
  code: string,
  sessionID: string,
  containerID: string,
  cmd: string[],
  rows: number,
  cols: number,
  layer: 'docker' | 'host' = 'docker',
): void {
  const entry = sockets.get(code)
  if (!entry || entry.ws.readyState !== WebSocket.OPEN) return
  entry.ws.send(JSON.stringify({
    type: 'EXEC_START',
    data: { session_id: sessionID, container_id: containerID, layer, cmd, rows, cols },
  }))
}

export function sendExecInput(code: string, sessionID: string, data: string): void {
  const entry = sockets.get(code)
  if (!entry || entry.ws.readyState !== WebSocket.OPEN) return
  entry.ws.send(JSON.stringify({ type: 'EXEC_INPUT', data: { session_id: sessionID, data } }))
}

export function sendExecResize(code: string, sessionID: string, rows: number, cols: number): void {
  const entry = sockets.get(code)
  if (!entry || entry.ws.readyState !== WebSocket.OPEN) return
  entry.ws.send(JSON.stringify({ type: 'EXEC_RESIZE', data: { session_id: sessionID, rows, cols } }))
}

export function sendExecEnd(code: string, sessionID: string): void {
  const entry = sockets.get(code)
  if (!entry || entry.ws.readyState !== WebSocket.OPEN) return
  entry.ws.send(JSON.stringify({ type: 'EXEC_END', data: { session_id: sessionID } }))
}

export function disconnectVM(code: string): void {
  const entry = sockets.get(code)
  if (entry) {
    entry.destroyed = true
    if (entry.reconnectTimer) clearTimeout(entry.reconnectTimer)
    entry.ws.close(1000, 'User disconnected')
    sockets.delete(code)
  }
  getStore().removeVM(code)
}

export function isConnected(code: string): boolean {
  const entry = sockets.get(code)
  return !!entry && entry.ws.readyState === WebSocket.OPEN
}

export function getSocketState(code: string): number | null {
  const entry = sockets.get(code)
  return entry ? entry.ws.readyState : null
}

// WebSocket manager for operations panel — sends to all open sockets
export function getWSManager() {
  return {
    send: (type: string, data: any) => {
      sockets.forEach((entry) => {
        if (entry.ws.readyState === WebSocket.OPEN) {
          entry.ws.send(JSON.stringify({ type, data }))
        }
      })
    },
    on: (_type: string, _handler: (data: any) => void) => {
      // Placeholder — real event subscription handled via store
    },
  }
}
