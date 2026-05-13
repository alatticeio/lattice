<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Search, ShieldCheck } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { useSandboxStore } from '@/stores/useSandboxStore'

definePage({
  meta: { titleKey: 'common.sandbox.audit.title', descKey: 'common.sandbox.audit.desc' },
})

const { t } = useI18n()
const store = useSandboxStore()

const searchQuery = ref('')
const verdictFilter = ref<'allow' | 'drop' | ''>('')

const filteredEvents = computed(() => {
  let events = store.auditEvents
  const q = searchQuery.value.toLowerCase().trim()
  if (q) {
    events = events.filter(e =>
      e.sandboxName.toLowerCase().includes(q) ||
      e.dstIP.toLowerCase().includes(q) ||
      e.srcIP.toLowerCase().includes(q)
    )
  }
  if (verdictFilter.value) {
    events = events.filter(e => e.verdict === verdictFilter.value)
  }
  return events
})

const verdictClass = (v: string) =>
  v === 'allow'
    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
    : 'bg-rose-500/10 text-rose-600 dark:text-rose-400'

async function fetchData() {
  await store.fetchAudit({
    keyword: searchQuery.value || undefined,
    verdict: verdictFilter.value || undefined,
  })
}

let debounce: ReturnType<typeof setTimeout>
watch([searchQuery, verdictFilter], () => {
  clearTimeout(debounce)
  debounce = setTimeout(fetchData, 300)
})

onMounted(fetchData)
</script>

<template>
  <div class="p-6 space-y-4">
    <!-- Filters -->
    <div class="flex flex-wrap items-center gap-3">
      <div class="relative max-w-xs">
        <Search class="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          v-model="searchQuery"
          :placeholder="t('common.sandbox.audit.searchPlaceholder')"
          class="h-9 pl-8 text-sm"
        />
      </div>
      <div class="flex gap-1.5">
        <Button
          v-for="opt in [
            { value: '', label: t('common.sandbox.audit.filterAll') },
            { value: 'allow', label: t('common.sandbox.audit.filterAllow') },
            { value: 'drop', label: t('common.sandbox.audit.filterDrop') },
          ]"
          :key="opt.value"
          :variant="verdictFilter === opt.value ? 'default' : 'outline'"
          size="sm"
          @click="verdictFilter = opt.value as '' | 'allow' | 'drop'"
        >
          {{ opt.label }}
        </Button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="store.auditLoading" class="flex gap-4">
      <div v-for="i in 4" :key="i" class="h-10 flex-1 animate-pulse rounded-lg bg-muted" />
    </div>

    <!-- Empty -->
    <div v-else-if="filteredEvents.length === 0" class="flex flex-col items-center gap-2 py-16 text-sm text-muted-foreground">
      <ShieldCheck class="size-10 opacity-40" />
      <p>{{ searchQuery || verdictFilter ? t('common.sandbox.audit.noMatch') : t('common.sandbox.audit.empty') }}</p>
    </div>

    <!-- Table -->
    <div v-else class="rounded-xl border border-border bg-card overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('common.sandbox.audit.colTime') }}</TableHead>
            <TableHead>{{ t('common.sandbox.audit.colSandbox') }}</TableHead>
            <TableHead>{{ t('common.sandbox.audit.colSrcIP') }}</TableHead>
            <TableHead>{{ t('common.sandbox.audit.colDst') }}</TableHead>
            <TableHead>{{ t('common.sandbox.audit.colProtocol') }}</TableHead>
            <TableHead>{{ t('common.sandbox.audit.colVerdict') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="event in filteredEvents" :key="event.id">
            <TableCell class="text-xs text-muted-foreground">
              {{ new Date(event.timestamp).toLocaleString() }}
            </TableCell>
            <TableCell class="font-medium text-sm">{{ event.sandboxName }}</TableCell>
            <TableCell class="font-mono text-xs text-muted-foreground">{{ event.srcIP }}</TableCell>
            <TableCell class="font-mono text-xs text-muted-foreground">
              {{ event.dstIP }}:{{ event.dstPort }}
            </TableCell>
            <TableCell class="text-xs uppercase text-muted-foreground">{{ event.protocol }}</TableCell>
            <TableCell>
              <Badge :class="verdictClass(event.verdict)" variant="secondary">
                {{ t(`common.sandbox.audit.${event.verdict}`) }}
              </Badge>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>
  </div>
</template>
