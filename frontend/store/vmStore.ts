'use client'

import { create } from 'zustand'
import { VMState, GraphOutput, GraphDiff, VMStatus } from '@/types'

interface VMStore {
  vms: Record<string, VMState>

  // Actions
  addVM: (code: string) => void
  removeVM: (code: string) => void
  setVMStatus: (code: string, status: VMStatus) => void
  setVMConnected: (code: string, hostname: string, scope: string[]) => void
  setVMGraph: (code: string, graph: GraphOutput) => void
  applyVMDiff: (code: string, diff: GraphDiff) => void
  setVMError: (code: string, error: string) => void
  clearVMError: (code: string) => void
  setVMDisconnected: (code: string) => void
}

export const useVMStore = create<VMStore>((set) => ({
  vms: {},

  addVM: (code) =>
    set((state) => ({
      vms: {
        ...state.vms,
        [code]: {
          code,
          status: 'connecting',
          hostname: null,
          scope: [],
          graph: null,
          error: null,
          lastUpdated: null,
        },
      },
    })),

  removeVM: (code) =>
    set((state) => {
      const { [code]: _, ...rest } = state.vms
      return { vms: rest }
    }),

  setVMStatus: (code, status) =>
    set((state) => ({
      vms: {
        ...state.vms,
        [code]: { ...state.vms[code], status },
      },
    })),

  setVMConnected: (code, hostname, scope) =>
    set((state) => ({
      vms: {
        ...state.vms,
        [code]: {
          ...state.vms[code],
          status: 'connected',
          hostname,
          scope,
          error: null,
        },
      },
    })),

  setVMGraph: (code, graph) =>
    set((state) => ({
      vms: {
        ...state.vms,
        [code]: {
          ...state.vms[code],
          graph,
          lastUpdated: Date.now(),
        },
      },
    })),

  applyVMDiff: (code, diff) =>
    set((state) => {
      const vm = state.vms[code]
      if (!vm?.graph) return state // no snapshot yet — diff can't be applied

      const nodeMap = new Map(vm.graph.nodes.map((n) => [n.id, n]))
      const edgeMap = new Map(vm.graph.edges.map((e) => [e.id, e]))

      // Apply node changes
      for (const n of diff.addedNodes) nodeMap.set(n.id, n)
      for (const n of diff.modifiedNodes) nodeMap.set(n.id, n)
      for (const id of diff.removedNodeIds) nodeMap.delete(id)

      // Apply edge changes
      for (const e of diff.addedEdges) edgeMap.set(e.id, e)
      for (const id of diff.removedEdgeIds) edgeMap.delete(id)

      return {
        vms: {
          ...state.vms,
          [code]: {
            ...vm,
            graph: {
              ...vm.graph,
              nodes: Array.from(nodeMap.values()),
              edges: Array.from(edgeMap.values()),
            },
            lastUpdated: Date.now(),
          },
        },
      }
    }),

  setVMError: (code, error) =>
    set((state) => ({
      vms: {
        ...state.vms,
        [code]: {
          ...state.vms[code],
          status: 'error',
          error,
        },
      },
    })),

  clearVMError: (code) =>
    set((state) => ({
      vms: {
        ...state.vms,
        [code]: {
          ...state.vms[code],
          error: null,
        },
      },
    })),

  setVMDisconnected: (code) =>
    set((state) => ({
      vms: {
        ...state.vms,
        [code]: {
          ...state.vms[code],
          status: 'disconnected',
        },
      },
    })),
}))
