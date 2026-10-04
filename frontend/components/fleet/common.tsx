'use client'

// Building blocks shared by the Machines and Clusters pages.

import { Fragment, type ReactNode } from 'react'
import type { SessionInfo } from '@/types'

export const MONO = "var(--font-geist-mono,'Geist Mono','JetBrains Mono',ui-monospace,monospace)"
export const SANS = "var(--font-geist,'Geist',ui-sans-serif,system-ui,sans-serif)"

// Go encodes an unset time.Time as 0001-01-01T00:00:00Z, so anything before
// 2000 counts as unknown instead of rendering as "739000d ago".
export function timeAgo(iso?: string): string | null {
  if (!iso) return null
  const t = Date.parse(iso)
  if (Number.isNaN(t) || new Date(t).getUTCFullYear() < 2000) return null
  const secs = Math.max(0, Math.round((Date.now() - t) / 1000))
  if (secs < 60) return 'just now'
  const mins = Math.floor(secs / 60)
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 48) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

// "Online · joined 2h ago" or "Offline · last seen 5m ago". pairedAt is set
// once, on the session's first pairing; lastSeen moves on every message and
// on disconnect.
export function statusLine(m: SessionInfo): string {
  const label = m.online ? 'Online' : 'Offline'
  const when = m.online ? timeAgo(m.pairedAt) : timeAgo(m.lastSeen)
  return when ? `${label} · ${m.online ? 'joined' : 'last seen'} ${when}` : label
}

export function FleetPage({ title, summary, action, children }: {
  title: string; summary: string; action?: ReactNode; children: ReactNode
}) {
  return (
    <div className="fleet-page" style={{ flex:1, minHeight:0, overflow:'auto' }}>
      <div style={{ display:'flex', alignItems:'flex-start', justifyContent:'space-between', gap:16, flexWrap:'wrap', marginBottom:20 }}>
        <div style={{ minWidth:0 }}>
          <h1 style={{ fontSize:20, fontWeight:600, color:'var(--ink)', margin:0 }}>{title}</h1>
          <p style={{ fontSize:12.5, color:'var(--ink3)', margin:'4px 0 0' }}>{summary}</p>
        </div>
        {action}
      </div>
      {children}
    </div>
  )
}

export function CardGrid({ children }: { children: ReactNode }) {
  return (
    <div style={{ display:'grid', gridTemplateColumns:'repeat(auto-fill, minmax(min(260px, 100%), 1fr))', gap:12 }}>
      {children}
    </div>
  )
}

// The whole card opens the machine on click; the title is a real <button> so
// the same action is reachable by keyboard.
export function FleetCard({ active, onOpen, children }: { active: boolean; onOpen: () => void; children: ReactNode }) {
  return (
    <div className="fleet-card" data-active={active} onClick={onOpen} style={{
      background:'var(--surface)', borderRadius:12, padding:'14px 16px 12px',
      display:'flex', flexDirection:'column', gap:12, cursor:'pointer', minWidth:0,
    }}>
      {children}
    </div>
  )
}

export function CardHeader({ m, onOpen, badges }: { m: SessionInfo; onOpen: () => void; badges?: ReactNode }) {
  const name = m.hostname || m.id
  return (
    <div style={{ minWidth:0 }}>
      <div style={{ display:'flex', alignItems:'center', gap:8, minWidth:0 }}>
        <span aria-hidden style={{ width:8, height:8, borderRadius:'50%', flexShrink:0, background: m.online ? 'var(--healthy)' : 'var(--ink4)' }} />
        <button
          onClick={e => { e.stopPropagation(); onOpen() }}
          title={`Open ${name}`}
          style={{
            flex:1, minWidth:0, padding:0, border:'none', background:'transparent', cursor:'pointer', textAlign:'left',
            fontSize:14, fontWeight:600, color:'var(--ink)', fontFamily:SANS,
            overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap',
          }}
        >{name}</button>
        {badges}
      </div>
      <div style={{ fontSize:11.5, color:'var(--ink3)', marginTop:4, paddingLeft:16 }}>{statusLine(m)}</div>
    </div>
  )
}

export function Badge({ children, title }: { children: ReactNode; title?: string }) {
  return (
    <span title={title} style={{
      fontSize:9.5, fontFamily:MONO, padding:'1px 6px', borderRadius:20, flexShrink:0,
      border:'1px solid var(--line2)', color:'var(--ink3)',
    }}>{children}</span>
  )
}

export interface Fact { label: string; value: string; mono?: boolean }

// Values are truncated to one line; the title shows the full text on hover.
export function Facts({ rows }: { rows: Fact[] }) {
  return (
    <dl style={{ display:'grid', gridTemplateColumns:'auto minmax(0,1fr)', columnGap:14, rowGap:5, margin:0, fontSize:11.5 }}>
      {rows.map(({ label, value, mono }) => (
        <Fragment key={label}>
          <dt style={{ color:'var(--ink4)', fontFamily:MONO }}>{label}</dt>
          <dd title={value} style={{
            margin:0, color:'var(--ink2)', overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap',
            fontFamily: mono ? MONO : undefined,
          }}>{value}</dd>
        </Fragment>
      ))}
    </dl>
  )
}

// Clicks inside `actions` never reach the card, including clicks on a
// disabled button, which some browsers still dispatch to ancestors.
export function CardFooter({ actions }: { actions?: ReactNode }) {
  return (
    <div style={{ display:'flex', alignItems:'center', gap:6, marginTop:'auto', paddingTop:10, borderTop:'1px solid var(--line)', minHeight:35 }}>
      {actions && (
        <div onClick={e => e.stopPropagation()} style={{ display:'flex', alignItems:'center', gap:6, flexWrap:'wrap', minWidth:0 }}>
          {actions}
        </div>
      )}
      <span style={{ marginLeft:'auto', fontSize:11.5, color:'var(--ink3)', flexShrink:0 }}>Open →</span>
    </div>
  )
}

export function SmallButton({ children, onClick, disabled, danger, title }: {
  children: ReactNode; onClick: () => void; disabled?: boolean; danger?: boolean; title?: string
}) {
  return (
    <button title={title} disabled={disabled} onClick={onClick} style={{
      fontSize:11, fontFamily:MONO, padding:'3px 8px', borderRadius:6,
      cursor: disabled ? 'default' : 'pointer', opacity: disabled ? 0.5 : 1,
      border:`1px solid ${danger ? 'var(--unhealthy)' : 'var(--line2)'}`,
      background: danger ? 'color-mix(in srgb, var(--unhealthy) 12%, transparent)' : 'transparent',
      color: danger ? 'var(--unhealthy)' : 'var(--ink2)',
    }}>{children}</button>
  )
}

export function PrimaryButton({ children, onClick }: { children: ReactNode; onClick: () => void }) {
  return (
    <button onClick={onClick} style={{
      fontSize:12.5, padding:'7px 14px', borderRadius:7, cursor:'pointer', flexShrink:0,
      border:'none', background:'var(--ink)', color:'var(--bg)', fontFamily:SANS,
    }}>{children}</button>
  )
}

export function EmptyState({ children }: { children: ReactNode }) {
  return (
    <div style={{
      padding:'28px 20px', borderRadius:12, border:'1px dashed var(--line2)',
      fontSize:12.5, color:'var(--ink3)', textAlign:'center', lineHeight:1.6,
    }}>{children}</div>
  )
}
