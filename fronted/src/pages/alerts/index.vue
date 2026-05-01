<template>
  <div class="p-6 space-y-6">
    <div class="flex justify-between items-center">
      <h1 class="text-2xl font-bold">{{ $t('manage.alerts') || 'Alert History' }}</h1>
      <div class="flex gap-2">
        <select v-model="statusFilter" class="rounded border px-3 py-1 text-sm">
          <option value="all">All</option>
          <option value="firing">Firing</option>
          <option value="resolved">Resolved</option>
        </select>
      </div>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 5" :key="i" class="animate-pulse h-12 bg-gray-200 rounded" />
    </div>

    <div v-else-if="filteredAlerts.length === 0" class="text-center py-12 text-gray-500">
      {{ $t('manage.no_alerts') || 'No alerts found' }}
    </div>

    <div v-else class="space-y-3">
      <div v-for="alert in filteredAlerts" :key="alert.id"
        class="p-4 rounded-lg border"
        :class="alert.status === 'firing' ? 'border-red-200 bg-red-50' : 'border-green-200 bg-green-50'">
        <div class="flex justify-between items-start">
          <div>
            <span class="font-semibold">{{ alert.message || alert.rule_id }}</span>
            <span class="ml-2 text-sm text-gray-500">{{ formatTime(alert.started_at) }}</span>
          </div>
          <span class="px-2 py-1 text-xs rounded-full font-medium"
            :class="alert.status === 'firing' ? 'bg-red-100 text-red-800' : 'bg-green-100 text-green-800'">
            {{ alert.status }}
          </span>
        </div>
        <div class="mt-1 text-sm text-gray-600">
          <span class="font-mono">Value: {{ alert.value.toFixed(2) }}</span>
          <span class="ml-2">Severity: {{ alert.severity }}</span>
        </div>
      </div>
    </div>

    <!-- Pagination -->
    <div v-if="total > pageSize" class="flex justify-center gap-2">
      <button @click="page--" :disabled="page <= 1" class="px-3 py-1 rounded border disabled:opacity-50">
        Prev
      </button>
      <span class="px-3 py-1">Page {{ page }} / {{ Math.ceil(total / pageSize) }}</span>
      <button @click="page++" :disabled="page * pageSize >= total" class="px-3 py-1 rounded border disabled:opacity-50">
        Next
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { listAlertHistory } from '@/api/alert'
import type { AlertHistory } from '@/types/alert'

const alerts = ref<AlertHistory[]>([])
const loading = ref(true)
const statusFilter = ref('all')
const page = ref(1)
const pageSize = 20
const total = ref(0)

const filteredAlerts = computed(() => {
  if (statusFilter.value === 'all') return alerts.value
  return alerts.value.filter(a => a.status === statusFilter.value)
})

const formatTime = (t: string) => {
  return new Date(t).toLocaleString()
}

const fetchAlerts = async () => {
  loading.value = true
  try {
    const res = await listAlertHistory('', page.value, pageSize)
    alerts.value = res.items
    total.value = res.total
  } catch (e) {
    console.error('Failed to fetch alerts:', e)
  } finally {
    loading.value = false
  }
}

onMounted(fetchAlerts)
</script>
