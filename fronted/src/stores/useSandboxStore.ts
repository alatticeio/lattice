import { defineStore } from 'pinia'
import {
  listSandboxes, revokeSandbox,
  listTokens, createToken, revokeToken,
  listTrafficAudit,
  type SandboxAgent, type EnrollmentToken, type CreateTokenInput,
  type TrafficAuditEvent, type TrafficAuditParams,
} from '@/api/sandbox'

export const useSandboxStore = defineStore('sandbox', {
  state: () => ({
    sandboxes: [] as SandboxAgent[],
    sandboxesLoading: false,
    sandboxesError: null as string | null,

    tokens: [] as EnrollmentToken[],
    tokensLoading: false,
    tokensError: null as string | null,

    auditEvents: [] as TrafficAuditEvent[],
    auditLoading: false,
    auditError: null as string | null,
  }),

  getters: {
    onlineCount: (state) => state.sandboxes.filter(s => s.status === 'online').length,
    totalSandboxes: (state) => state.sandboxes.length,
  },

  actions: {
    async fetchSandboxes() {
      this.sandboxesLoading = true
      this.sandboxesError = null
      try {
        this.sandboxes = await listSandboxes()
      } catch (e: any) {
        this.sandboxesError = e?.message || 'Failed to load sandboxes'
      } finally {
        this.sandboxesLoading = false
      }
    },

    async revokeSandbox(name: string) {
      await revokeSandbox(name)
      this.sandboxes = this.sandboxes.filter(s => s.name !== name)
    },

    async fetchTokens() {
      this.tokensLoading = true
      this.tokensError = null
      try {
        this.tokens = await listTokens()
      } catch (e: any) {
        this.tokensError = e?.message || 'Failed to load tokens'
      } finally {
        this.tokensLoading = false
      }
    },

    async generateToken(input: CreateTokenInput): Promise<EnrollmentToken> {
      const token = await createToken(input)
      this.tokens.unshift(token)
      return token
    },

    async revokeToken(token: string) {
      await revokeToken(token)
      this.tokens = this.tokens.filter(t => t.maskedToken !== token && t.token !== token)
    },

    async fetchAudit(params: TrafficAuditParams = {}) {
      this.auditLoading = true
      this.auditError = null
      try {
        this.auditEvents = await listTrafficAudit(params)
      } catch (e: any) {
        this.auditError = e?.message || 'Failed to load audit events'
      } finally {
        this.auditLoading = false
      }
    },
  },
})
