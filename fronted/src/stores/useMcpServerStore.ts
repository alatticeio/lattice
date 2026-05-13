import { defineStore } from 'pinia'
import {
  listUserMCPTools, registerMCPTool, deleteMCPTool,
  type UserMCPTool, type RegisterMCPToolInput,
} from '@/api/mcp-server'

export const useMcpServerStore = defineStore('mcp-server', {
  state: () => ({
    servers: [] as UserMCPTool[],
    loading: false,
    error: null as string | null,
    registering: false,
  }),

  getters: {
    workspaceServers: (state) =>
      state.servers.filter(s => s.visibility !== 'private'),
  },

  actions: {
    async fetch() {
      this.loading = true
      this.error = null
      try {
        this.servers = await listUserMCPTools()
      } catch (e: any) {
        this.error = e?.message || 'Failed to load MCP servers'
      } finally {
        this.loading = false
      }
    },

    async register(input: RegisterMCPToolInput): Promise<UserMCPTool> {
      this.registering = true
      try {
        const server = await registerMCPTool(input)
        this.servers.unshift(server)
        return server
      } finally {
        this.registering = false
      }
    },

    async remove(name: string) {
      await deleteMCPTool(name)
      this.servers = this.servers.filter(s => s.name !== name)
    },
  },
})
