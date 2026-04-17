// ─── Graph Types ────────────────────────────────────────────────────────────

export type NodeHealth = 'healthy' | 'degraded' | 'unhealthy' | 'unknown'

export type NodeType =
  | 'pod'
  | 'deployment'
  | 'k8s_service'
  | 'namespace'
  | 'container'
  | 'host'
  | 'cluster'
  | 'node'
  | 'ingress'
  | 'statefulset'
  | 'daemonset'
  | 'cronjob'
  | 'job'
  | 'pvc'
  | 'pv'
  | 'storageclass'
  | 'event'
  | 'image'
  | 'volume'
  | 'network'
  | 'container_runtime'
  | string

export type EdgeType =
  | 'CONTAINS'
  | 'OWNS'
  | 'RUNS_ON'
  | 'TARGETS'
  | 'ROUTES_TO'
  | 'REFERENCES'
  | 'MOUNTS'
  | 'USES'
  | 'CONNECTS_TO'
  | 'BINDS_TO'
  | 'PROVISIONS'
  | 'RELATES_TO'
  | string

export interface GraphNode {
  id: string
  type: NodeType
  label: string
  health: NodeHealth
  metadata: Record<string, any>
}

export interface GraphEdge {
  id: string
  source: string
  target: string
  type: EdgeType
  properties?: Record<string, string>
}

export interface GraphSnapshot {
  hostId: string
  timestamp: string
  collectionDuration: number
}

export interface GraphStats {
  totalNodes: number
  totalEdges: number
  nodesByType: Record<string, number>
  filteredOut: {
    processes: number
    systemdServices: number
    orphanedNodes: number
    danglingImages: number
  }
  collectionScope: string[]
}

export interface GraphOutput {
  snapshot: GraphSnapshot
  nodes: GraphNode[]
  edges: GraphEdge[]
  stats: GraphStats
}

// ─── WebSocket Message Types ─────────────────────────────────────────────────

export interface WsMessagePair {
  type: 'PAIR'
  data: { code: string }
}

export interface WsMessageCommand {
  type: 'COMMAND'
  data: { action: string }
}

export type WsOutbound = WsMessagePair | WsMessageCommand

export interface WsGraphSnapshot {
  type: 'GRAPH_SNAPSHOT'
  data: GraphOutput
}

export interface GraphDiff {
  timestamp: string
  addedNodes: GraphNode[]
  modifiedNodes: GraphNode[]
  removedNodeIds: string[]
  addedEdges: GraphEdge[]
  removedEdgeIds: string[]
}

export interface WsGraphDiff {
  type: 'GRAPH_DIFF'
  data: GraphDiff
}

export interface WsAgentConnected {
  type: 'AGENT_CONNECTED'
  data: { hostname: string; scope: string[] }
}

export interface WsAgentDisconnected {
  type: 'AGENT_DISCONNECTED'
  data: Record<string, never>
}

export interface WsError {
  type: 'ERROR'
  data: { message: string }
}

export interface WsLogData {
  type: 'LOG_DATA'
  data: {
    request_id: string
    container_id: string
    lines: string[]
    done: boolean
    error?: string
  }
}

export interface WsExecData {
  type: 'EXEC_DATA'
  data: {
    session_id: string
    data: string // base64
    error?: string
  }
}

export interface WsExecEnd {
  type: 'EXEC_END'
  data: { session_id: string }
}

export type WsInbound =
  | WsGraphSnapshot
  | WsGraphDiff
  | WsAgentConnected
  | WsAgentDisconnected
  | WsError
  | WsLogData
  | WsExecData
  | WsExecEnd

// ─── App State Types ──────────────────────────────────────────────────────────

export type VMStatus = 'connecting' | 'paired' | 'connected' | 'disconnected' | 'error'

export interface VMState {
  code: string
  status: VMStatus
  hostname: string | null
  scope: string[]
  graph: GraphOutput | null
  error: string | null
  lastUpdated: number | null
}

// ─── Session API Types ────────────────────────────────────────────────────────

export interface SessionInfo {
  code: string
  hostname: string
  scope: string[]
  browserCount: number
  paired: boolean
}

// ─── Node Color Map ───────────────────────────────────────────────────────────

export const NODE_COLORS: Record<string, string> = {
  cluster: '#6366f1',
  node: '#3b82f6',
  namespace: '#8b5cf6',
  deployment: '#06b6d4',
  statefulset: '#0891b2',
  daemonset: '#0284c7',
  cronjob: '#7c3aed',
  job: '#9333ea',
  pod: '#10b981', // overridden by health
  k8s_service: '#f97316',
  ingress: '#f59e0b',
  container: '#10b981',
  host: '#64748b',
  container_runtime: '#475569',
  pvc: '#a78bfa',
  pv: '#7c3aed',
  storageclass: '#c026d3',
  event: '#ef4444',
  image: '#34d399',
  volume: '#84cc16',
  network: '#14b8a6',
}

export const HEALTH_COLORS: Record<NodeHealth, string> = {
  healthy: '#10b981',
  degraded: '#f59e0b',
  unhealthy: '#ef4444',
  unknown: '#64748b',
}

export const NODE_ICONS: Record<string, string> = {
  cluster: '⎈',
  node: '◻',
  namespace: '⬡',
  deployment: '⟳',
  statefulset: '▣',
  daemonset: '⊞',
  cronjob: '⏱',
  job: '▶',
  pod: '◉',
  k8s_service: '⊕',
  ingress: '⊸',
  container: '⬜',
  host: '⬛',
  container_runtime: '⬟',
  pvc: '💾',
  pv: '🗄',
  storageclass: '📦',
  event: '⚡',
  image: '🖼',
  volume: '🗂',
  network: '🌐',
}

export function getNodeColor(type: string, health?: NodeHealth): string {
  if (type === 'pod' && health && health !== 'unknown') {
    return HEALTH_COLORS[health]
  }
  return NODE_COLORS[type] ?? '#64748b'
}

export function getHealthColor(health: NodeHealth): string {
  return HEALTH_COLORS[health]
}

export function getNodeIcon(type: string): string {
  return NODE_ICONS[type] ?? '◆'
}
