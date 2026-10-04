'use client'

import type { SessionInfo } from '@/types'
import {
  Badge, CardFooter, CardGrid, CardHeader, EmptyState, Facts, FleetCard, FleetPage, MONO, PrimaryButton, type Fact,
} from './common'

interface Props {
  machines: SessionInfo[]
  isActive: (m: SessionInfo) => boolean
  onOpen: (m: SessionInfo) => void
  onAddMachine: () => void
}

export default function MachinesView({ machines, isActive, onOpen, onAddMachine }: Props) {
  const online = machines.filter(m => m.online).length
  const summary = machines.length === 0
    ? 'No machines reported by this server.'
    : `${machines.length} ${machines.length === 1 ? 'machine' : 'machines'}, ${online} online`

  return (
    <FleetPage title="Machines" summary={summary} action={<PrimaryButton onClick={onAddMachine}>+ Add machine</PrimaryButton>}>
      {machines.length === 0 ? (
        <EmptyState>
          The list comes from <code style={{ fontFamily:MONO }}>/api/sessions</code>, which the dashboard
          polls every 10 seconds. It stays empty if that request fails or the server predates it.
        </EmptyState>
      ) : (
        <CardGrid>
          {machines.map(m => {
            const active = isActive(m)
            const open = () => onOpen(m)
            const rows: Fact[] = [{ label:'resources', value:m.nodeCount.toLocaleString() }]
            if (m.scope?.length) rows.push({ label:'scope', value:m.scope.join(', '), mono:true })
            return (
              <FleetCard key={m.id} active={active} onOpen={open}>
                <CardHeader m={m} onOpen={open} badges={<>
                  {m.local && <Badge title="The agent on the host running this dashboard">local</Badge>}
                  {active && <Badge title="Shown on Overview and Canvas">viewing</Badge>}
                  {m.readOnly && <Badge title="Write actions and terminals are blocked">read-only</Badge>}
                </>} />
                <Facts rows={rows} />
                <CardFooter />
              </FleetCard>
            )
          })}
        </CardGrid>
      )}
    </FleetPage>
  )
}
