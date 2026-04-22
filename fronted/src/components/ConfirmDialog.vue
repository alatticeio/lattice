<script setup lang="ts">
import { confirmState } from '@/composables/useConfirm'
import { AlertTriangle, Info } from 'lucide-vue-next'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

function handleConfirm() {
  confirmState.open = false
  confirmState.resolve?.(true)
}

function handleCancel() {
  confirmState.open = false
  confirmState.resolve?.(false)
}
</script>

<template>
  <Dialog :open="confirmState.open" @update:open="v => { if (!v) handleCancel() }">
    <DialogContent class="sm:max-w-sm gap-0 p-0 overflow-hidden">
      <DialogHeader class="px-6 pt-6 pb-4">
        <div class="flex items-center gap-3 mb-1">
          <div
            class="size-9 rounded-xl flex items-center justify-center shrink-0"
            :class="confirmState.type === 'danger' ? 'bg-destructive/10' : 'bg-amber-500/10'"
          >
            <AlertTriangle
              v-if="confirmState.type === 'danger' || confirmState.type === 'warning'"
              class="size-4"
              :class="confirmState.type === 'danger' ? 'text-destructive' : 'text-amber-500'"
            />
            <Info v-else class="size-4 text-blue-500" />
          </div>
          <DialogTitle class="text-base">{{ confirmState.title }}</DialogTitle>
        </div>
        <DialogDescription class="text-sm leading-relaxed ml-12">
          {{ confirmState.message }}
        </DialogDescription>
      </DialogHeader>

      <DialogFooter class="px-6 py-4 border-t border-border">
        <Button variant="outline" @click="handleCancel">{{ confirmState.cancelText }}</Button>
        <Button
          :variant="confirmState.type === 'danger' ? 'destructive' : 'default'"
          @click="handleConfirm"
        >
          {{ confirmState.confirmText }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
