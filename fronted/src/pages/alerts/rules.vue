<template>
  <div class="p-6 space-y-6">
    <div class="flex justify-between items-center">
      <h1 class="text-2xl font-bold">{{ $t('manage.alert_rules') || 'Alert Rules' }}</h1>
      <button @click="showCreateDialog = true" class="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700">
        + Create Rule
      </button>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 3" :key="i" class="animate-pulse h-16 bg-gray-200 rounded" />
    </div>

    <div v-else-if="rules.length === 0" class="text-center py-12 text-gray-500">
      {{ $t('manage.no_alert_rules') || 'No alert rules configured' }}
    </div>

    <div v-else class="space-y-3">
      <div v-for="rule in rules" :key="rule.id"
        class="p-4 rounded-lg border bg-white">
        <div class="flex justify-between items-start">
          <div class="flex-1">
            <div class="flex items-center gap-2">
              <span class="font-semibold">{{ rule.name }}</span>
              <span class="px-2 py-0.5 text-xs rounded-full"
                :class="rule.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'">
                {{ rule.enabled ? 'Enabled' : 'Disabled' }}
              </span>
              <span class="px-2 py-0.5 text-xs rounded-full"
                :class="severityColor(rule.severity)">
                {{ rule.severity }}
              </span>
            </div>
            <div class="mt-1 text-sm text-gray-600">
              <span class="font-mono">{{ rule.metric_type }}</span>
              <span class="mx-1">{{ rule.operator }}</span>
              <span class="font-mono">{{ rule.threshold }}</span>
            </div>
            <div v-if="rule.message" class="mt-1 text-sm text-gray-500">
              {{ rule.message }}
            </div>
          </div>
          <div class="flex gap-2">
            <button @click="deleteRule(rule.id)" class="text-red-600 hover:text-red-800 text-sm">
              Delete
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Dialog (simplified) -->
    <div v-if="showCreateDialog" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div class="bg-white rounded-lg p-6 w-full max-w-lg">
        <h2 class="text-lg font-bold mb-4">Create Alert Rule</h2>
        <div class="space-y-3">
          <input v-model="form.name" placeholder="Rule name" class="w-full border rounded px-3 py-2" />
          <select v-model="form.metric_type" class="w-full border rounded px-3 py-2">
            <option value="lattice_online_count">Online Count</option>
            <option value="lattice_avg_latency">Avg Latency</option>
            <option value="lattice_packet_loss">Packet Loss</option>
          </select>
          <div class="flex gap-2">
            <select v-model="form.operator" class="w-1/3 border rounded px-3 py-2">
              <option value="gt">&gt;</option>
              <option value="gte">&gt;=</option>
              <option value="lt">&lt;</option>
              <option value="lte">&lt;=</option>
              <option value="eq">=</option>
            </select>
            <input v-model.number="form.threshold" type="number" placeholder="Threshold" class="w-2/3 border rounded px-3 py-2" />
          </div>
          <select v-model="form.severity" class="w-full border rounded px-3 py-2">
            <option value="critical">Critical</option>
            <option value="warning">Warning</option>
            <option value="info">Info</option>
          </select>
          <textarea v-model="form.message" placeholder="Alert message" class="w-full border rounded px-3 py-2" rows="2" />
        </div>
        <div class="flex justify-end gap-2 mt-4">
          <button @click="showCreateDialog = false" class="px-4 py-2 border rounded">Cancel</button>
          <button @click="submitRule" class="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700">Create</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listAlertRules, createAlertRule, deleteAlertRule as delRule } from '@/api/alert'
import type { AlertRule, CreateAlertRuleRequest } from '@/types/alert'

const rules = ref<AlertRule[]>([])
const loading = ref(true)
const showCreateDialog = ref(false)

const form = ref<CreateAlertRuleRequest>({
  name: '',
  metric_type: '',
  operator: 'gt',
  threshold: 0,
  duration: '',
  lookback: '5m',
  severity: 'warning',
  message: '',
})

const severityColor = (s: string) => {
  switch (s) {
    case 'critical': return 'bg-red-100 text-red-800'
    case 'warning': return 'bg-yellow-100 text-yellow-800'
    case 'info': return 'bg-blue-100 text-blue-800'
    default: return 'bg-gray-100 text-gray-600'
  }
}

const fetchRules = async () => {
  loading.value = true
  try {
    const res = await listAlertRules('')
    rules.value = res
  } catch (e) {
    console.error('Failed to fetch rules:', e)
  } finally {
    loading.value = false
  }
}

const deleteRule = async (id: string) => {
  if (!confirm('Delete this rule?')) return
  try {
    await delRule(id)
    await fetchRules()
  } catch (e) {
    console.error('Failed to delete rule:', e)
  }
}

const submitRule = async () => {
  try {
    await createAlertRule('', form.value)
    showCreateDialog.value = false
    form.value = { name: '', metric_type: '', operator: 'gt', threshold: 0, duration: '', lookback: '5m', severity: 'warning', message: '' }
    await fetchRules()
  } catch (e) {
    console.error('Failed to create rule:', e)
  }
}

onMounted(fetchRules)
</script>
