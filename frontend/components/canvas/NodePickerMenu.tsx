'use client'

import { useEffect, useRef, useState } from 'react'
import { Check } from 'lucide-react'

export interface PickerItem {
  key: string
  label: string
  /** Omit for a leaf item (e.g. one specific pod) — shown for categories/types instead. */
  count?: number
  checked: boolean
  health?: 'healthy' | 'degraded' | 'unhealthy' | 'unknown'
}

const HEALTH_DOT: Record<string, string> = {
  healthy: '#22c55e', degraded: '#f59e0b', unhealthy: '#ef4444', unknown: 'var(--line3)',
}

interface NodePickerMenuProps {
  title: string
  items: PickerItem[]
  x: number
  y: number
  onConfirm: (selectedKeys: string[]) => void
  onClose: () => void
}

/** Windows-desktop-style popup: check items, hit "Show" to reveal them on the canvas. */
export default function NodePickerMenu({ title, items, x, y, onConfirm, onClose }: NodePickerMenuProps) {
  const [checked, setChecked] = useState<Set<string>>(
    () => new Set(items.filter((i) => i.checked).map((i) => i.key))
  )
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    function onEsc(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('mousedown', onDocClick)
    document.addEventListener('keydown', onEsc)
    return () => {
      document.removeEventListener('mousedown', onDocClick)
      document.removeEventListener('keydown', onEsc)
    }
  }, [onClose])

  function toggle(key: string) {
    setChecked((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  // Clamp so the menu never renders off-screen
  const width = 240
  const maxHeight = Math.min(360, items.length * 34 + 96)
  const left = Math.min(x, (typeof window !== 'undefined' ? window.innerWidth : 1200) - width - 16)
  const top = Math.min(y, (typeof window !== 'undefined' ? window.innerHeight : 800) - maxHeight - 16)

  return (
    <div
      ref={ref}
      style={{
        position: 'fixed',
        left, top,
        width,
        background: 'var(--surface)',
        border: '1px solid var(--line2)',
        borderRadius: 8,
        boxShadow: '0 8px 32px rgba(0,0,0,0.6)',
        zIndex: 60,
        overflow: 'hidden',
        fontFamily: 'var(--font-geist,"Geist",ui-sans-serif,system-ui,sans-serif)',
      }}
      onClick={(e) => e.stopPropagation()}
    >
      <div style={{
        padding: '9px 12px',
        borderBottom: '1px solid var(--line)',
        fontSize: 11, fontWeight: 500, color: 'var(--ink3)',
        textTransform: 'uppercase', letterSpacing: '0.06em',
      }}>
        {title}
      </div>

      <div style={{ maxHeight: maxHeight - 88, overflowY: 'auto', padding: '4px 0' }}>
        {items.length === 0 && (
          <div style={{ padding: '14px 12px', fontSize: 12, color: 'var(--ink4)' }}>Nothing here</div>
        )}
        {items.map((item) => {
          const isChecked = checked.has(item.key)
          return (
            <div
              key={item.key}
              onClick={() => toggle(item.key)}
              style={{
                display: 'flex', alignItems: 'center', gap: 8,
                padding: '7px 12px', cursor: 'pointer',
                background: isChecked ? 'var(--surface2)' : 'transparent',
              }}
              onMouseEnter={(e) => { if (!isChecked) e.currentTarget.style.background = 'var(--surface2)' }}
              onMouseLeave={(e) => { if (!isChecked) e.currentTarget.style.background = 'transparent' }}
            >
              <div style={{
                width: 14, height: 14, borderRadius: 4, flexShrink: 0,
                border: `1px solid ${isChecked ? 'var(--ink)' : 'var(--line2)'}`,
                background: isChecked ? 'var(--ink)' : 'transparent',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}>
                {isChecked && <Check size={10} strokeWidth={3} color="var(--bg)" />}
              </div>
              {item.health && (
                <span style={{ width: 5, height: 5, borderRadius: '50%', background: HEALTH_DOT[item.health], flexShrink: 0 }} />
              )}
              <span style={{
                fontSize: 12, color: 'var(--ink)', flex: 1,
                overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }} title={item.label}>
                {item.label}
              </span>
              {item.count !== undefined && (
                <span style={{
                  fontSize: 10.5, fontWeight: 600, color: 'var(--ink3)',
                  fontFamily: 'var(--font-geist-mono,"Geist Mono",ui-monospace,monospace)',
                  background: 'var(--line)', border: '1px solid var(--line2)',
                  borderRadius: 20, padding: '1px 7px', flexShrink: 0,
                }}>
                  {item.count}
                </span>
              )}
            </div>
          )
        })}
      </div>

      <div style={{ display: 'flex', gap: 6, padding: '8px 10px', borderTop: '1px solid var(--line)' }}>
        <button
          onClick={onClose}
          style={{
            flex: 1, height: 28, borderRadius: 6, fontSize: 12,
            background: 'var(--surface)', border: '1px solid var(--line2)', color: 'var(--ink2)',
            cursor: 'pointer',
          }}
        >
          Cancel
        </button>
        <button
          onClick={() => onConfirm([...checked])}
          style={{
            flex: 1, height: 28, borderRadius: 6, fontSize: 12, fontWeight: 500,
            background: 'var(--ink)', border: '1px solid var(--ink)', color: 'var(--bg)',
            cursor: 'pointer',
          }}
        >
          Show{checked.size > 0 ? ` (${checked.size})` : ''}
        </button>
      </div>
    </div>
  )
}
