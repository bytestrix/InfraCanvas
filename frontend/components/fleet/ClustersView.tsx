'use client'

import { useEffect, useRef, useState } from 'react'
import type { SessionInfo } from '@/types'
import { fetchClusters, type ClusterEntry } from '@/lib/api'
import {
  Badge, CardFooter, CardGrid, CardHeader, EmptyState, Facts, FleetCard, FleetPage, MONO, PrimaryButton, SmallButton,
  type Fact,
} from './common'

interface Props {
  clusters: SessionInfo[]
  isActive: (m: SessionInfo) => boolean
  onOpen: (m: SessionInfo) => void
  onAddCluster: () => void
  onToggleReadOnly: (m: SessionInfo) => Promise<void>
  onRemove: (m: SessionInfo) => Promise<void>
}

// Kubeconfig connections run as sessions with machineId "cluster-<id>";
// in-cluster relay pods connect like any remote agent and have no such id.
function kubeconfigClusterId(m: SessionInfo): string | null {
  return m.machineId?.startsWith('cluster-') ? m.machineId.slice('cluster-'.length) : null
}

export default function ClustersView({ clusters, isActive, onOpen, onAddCluster, onToggleReadOnly, onRemove }: Props) {
  // Context, API server and added date live in /api/clusters, not in the
  // session list. They only change when a cluster is added or removed, so
  // refetch when the set of clusters changes instead of polling. Any failure
  // (e.g. a server without the cluster manager answers 503) just means the
  // cards show fewer details.
  const [entries, setEntries] = useState<Record<string, ClusterEntry>>({})
  const clusterSetKey = clusters.map(c => c.machineId ?? c.id).join(',')
  useEffect(() => {
    let stop = false
    fetchClusters()
      .then(list => { if (!stop) setEntries(Object.fromEntries(list.map(e => [e.id, e]))) })
      .catch(() => {})
    return () => { stop = true }
  }, [clusterSetKey])

  const online = clusters.filter(m => m.online).length
  const summary = clusters.length === 0
    ? 'No clusters connected.'
    : `${clusters.length} ${clusters.length === 1 ? 'cluster' : 'clusters'}, ${online} online`

  return (
    <FleetPage title="Clusters" summary={summary} action={<PrimaryButton onClick={onAddCluster}>+ Connect cluster</PrimaryButton>}>
      {clusters.length === 0 ? (
        <EmptyState>
          Connect a cluster with a kubeconfig. The file stays on this machine, and the dashboard
          talks to the cluster&apos;s API server directly.
        </EmptyState>
      ) : (
        <CardGrid>
          {clusters.map(m => {
            const id = kubeconfigClusterId(m)
            return (
              <ClusterCard key={m.id} m={m} clusterId={id} entry={id ? entries[id] : undefined}
                active={isActive(m)} onOpen={() => onOpen(m)}
                onToggleReadOnly={onToggleReadOnly} onRemove={onRemove} />
            )
          })}
        </CardGrid>
      )}
    </FleetPage>
  )
}

function ClusterCard({ m, clusterId, entry, active, onOpen, onToggleReadOnly, onRemove }: {
  m: SessionInfo
  clusterId: string | null
  entry?: ClusterEntry
  active: boolean
  onOpen: () => void
  onToggleReadOnly: (m: SessionInfo) => Promise<void>
  onRemove: (m: SessionInfo) => Promise<void>
}) {
  const [busy, setBusy] = useState<'readonly' | 'remove' | null>(null)
  const [confirming, setConfirming] = useState(false)
  const [error, setError] = useState('')
  const confirmTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => () => { if (confirmTimer.current) clearTimeout(confirmTimer.current) }, [])

  const run = (kind: 'readonly' | 'remove', fn: () => Promise<void>) => {
    setBusy(kind)
    setError('')
    fn().catch(e => setError(e?.message ?? 'Request failed')).finally(() => setBusy(null))
  }

  // Two clicks to disconnect, same as the sidebar: the first arms it for 3s.
  const disconnect = () => {
    if (confirmTimer.current) clearTimeout(confirmTimer.current)
    if (!confirming) {
      setConfirming(true)
      setError('')
      confirmTimer.current = setTimeout(() => setConfirming(false), 3000)
      return
    }
    setConfirming(false)
    run('remove', () => onRemove(m))
  }

  const rows: Fact[] = [{ label:'source', value: clusterId ? 'kubeconfig' : 'in-cluster agent' }]
  if (entry?.context_name) rows.push({ label:'context', value:entry.context_name, mono:true })
  if (entry?.server_url) rows.push({ label:'server', value:entry.server_url, mono:true })
  rows.push({ label:'resources', value:m.nodeCount.toLocaleString() })
  const added = entry?.added_at ? new Date(entry.added_at) : null
  if (added && !Number.isNaN(added.getTime())) rows.push({ label:'added', value:added.toLocaleDateString() })

  return (
    <FleetCard active={active} onOpen={onOpen}>
      <CardHeader m={m} onOpen={onOpen} badges={<>
        {active && <Badge title="Shown on Overview and Canvas">viewing</Badge>}
        {m.readOnly && <Badge title="Write actions and terminals are blocked on this cluster">read-only</Badge>}
      </>} />
      <Facts rows={rows} />
      {error && <div style={{ fontSize:11, color:'var(--unhealthy)', fontFamily:MONO, wordBreak:'break-word' }}>{error}</div>}
      <CardFooter actions={clusterId ? <>
        <SmallButton
          disabled={busy !== null}
          title={m.readOnly ? 'Allow write actions and terminals again' : 'Block write actions and terminals on this cluster'}
          onClick={() => run('readonly', () => onToggleReadOnly(m))}
        >{busy === 'readonly' ? 'Saving…' : m.readOnly ? 'Allow writes' : 'Make read-only'}</SmallButton>
        <SmallButton
          disabled={busy !== null}
          danger={confirming}
          title="Remove this cluster connection and its stored kubeconfig"
          onClick={disconnect}
        >{busy === 'remove' ? 'Disconnecting…' : confirming ? 'Confirm' : 'Disconnect'}</SmallButton>
      </> : undefined} />
    </FleetCard>
  )
}
