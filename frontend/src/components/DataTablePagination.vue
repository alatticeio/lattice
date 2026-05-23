<script setup lang="ts">
import type { Table } from '@tanstack/vue-table'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { computed } from 'vue'

interface DataTablePaginationProps {
  table: Table<any>
}

const props = defineProps<DataTablePaginationProps>()

// 计算需要显示的页码（可选：如果页数太多，可以自行添加省略号逻辑）
const pageIndex = computed(() => props.table.getState().pagination.pageIndex)
const pageCount = computed(() => props.table.getPageCount())
</script>

<template>
  <div class="flex items-center justify-between px-2 py-4">
    <div class="text-sm text-muted-foreground">
      共 {{ table.getFilteredRowModel().rows.length }} 条 ·
      第 {{ pageIndex + 1 }} / {{ pageCount }} 页
    </div>

    <div class="flex items-center gap-1">
      <Button
          variant="outline"
          size="sm"
          class="size-8 p-0"
          :disabled="!table.getCanPreviousPage()"
          @click="table.previousPage()"
      >
        <span class="sr-only">上一页</span>
        <ChevronLeft class="size-4" />
      </Button>

      <Button
          v-for="p in pageCount"
          :key="p"
          variant="outline"
          size="sm"
          class="size-8 p-0 text-xs transition-colors"
          :class="
          p - 1 === pageIndex
            ? 'bg-primary text-primary-foreground border-primary hover:bg-primary/90 hover:text-primary-foreground'
            : ''
        "
          @click="table.setPageIndex(p - 1)"
      >
        {{ p }}
      </Button>

      <Button
          variant="outline"
          size="sm"
          class="size-8 p-0"
          :disabled="!table.getCanNextPage()"
          @click="table.nextPage()"
      >
        <span class="sr-only">下一页</span>
        <ChevronRight class="size-4" />
      </Button>
    </div>
  </div>
</template>