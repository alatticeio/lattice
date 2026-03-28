<script setup>
import { computed } from 'vue'

const props = defineProps({
  page: { type: Number, default: 1 },
  pageSize: { type: Number, default: 10 },
  total: { type: Number, default: 0 },
  itemName: { type: String, default: '记录' }
})

const emit = defineEmits(['update:page', 'change'])

const totalPages = computed(() => Math.ceil(props.total / props.pageSize) || 1)
const startRange = computed(() => (props.page - 1) * props.pageSize + 1)
const endRange = computed(() => Math.min(props.page * props.pageSize, props.total))

const handlePageChange = (newPage) => {
  if (newPage >= 1 && newPage <= totalPages.value) {
    emit('update:page', newPage)
    emit('change', newPage)
  }
}
</script>

<template>
  <div v-if="total > 0" class="p-4 px-6 bg-base-200/10 flex flex-col sm:flex-row justify-between items-center gap-4 border-t border-base-300">
    <div class="text-xs font-medium opacity-60">
      显示 {{ startRange }} - {{ endRange }} / 共 {{ total }} 条{{ itemName }}
    </div>

    <div class="join shadow-sm">
      <button
          class="join-item btn btn-xs btn-outline border-base-300 hover:bg-base-300 disabled:bg-transparent"
          :disabled="page === 1"
          @click="handlePageChange(page - 1)"
      >
        «
      </button>

      <button class="join-item btn btn-xs btn-outline border-base-300 no-animation font-mono">
        Page {{ page }} / {{ totalPages }}
      </button>

      <button
          class="join-item btn btn-xs btn-outline border-base-300 hover:bg-base-300 disabled:bg-transparent"
          :disabled="page >= totalPages"
          @click="handlePageChange(page + 1)"
      >
        »
      </button>
    </div>
  </div>
</template>