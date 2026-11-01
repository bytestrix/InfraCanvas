import { create } from 'zustand'
import type { GraphNode } from '@/types'

export interface TerminalTab {
  id: string
  label: string    // node label (container name, pod name, or "VM shell")
  vmLabel: string  // VM identifier shown as subtitle
  vmCode: string
  node: GraphNode
  layer: 'docker' | 'host' | 'kubernetes'
  namespace?: string  // kubernetes only
}

export const TERMINAL_FONT_SIZE = 13

interface TerminalStore {
  tabs: TerminalTab[]
  activeTabId: string | null
  isOpen: boolean
  openTerminal: (tab: Omit<TerminalTab, 'id'>) => void
  closeTab: (id: string) => void
  setActive: (id: string) => void
  setOpen: (open: boolean) => void
  toggle: () => void
}

function genId(): string {
  return Math.random().toString(36).slice(2) + Date.now().toString(36)
}

export const useTerminalStore = create<TerminalStore>((set, get) => ({
  tabs: [],
  activeTabId: null,
  isOpen: false,

  openTerminal: (tab) => {
    const existing = get().tabs.find(
      t => t.vmCode === tab.vmCode && t.node.id === tab.node.id && t.layer === tab.layer,
    )
    if (existing) {
      set({ activeTabId: existing.id, isOpen: true })
      return
    }
    const id = genId()
    set(s => ({ tabs: [...s.tabs, { ...tab, id }], activeTabId: id, isOpen: true }))
  },

  closeTab: (id) => {
    set(s => {
      const tabs = s.tabs.filter(t => t.id !== id)
      const activeTabId = s.activeTabId === id
        ? (tabs.length > 0 ? tabs[tabs.length - 1].id : null)
        : s.activeTabId
      return { tabs, activeTabId, isOpen: tabs.length > 0 && s.isOpen }
    })
  },

  setActive: (id) => set({ activeTabId: id, isOpen: true }),
  setOpen: (open) => set({ isOpen: open }),
  toggle: () => set(s => ({ isOpen: !s.isOpen })),
}))
