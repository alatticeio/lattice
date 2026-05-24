<script setup lang="ts">
import { AVATAR_PRESETS, toPresetUrl } from '@/lib/avatarPresets'
import AvatarPreset from '@/components/AvatarPreset.vue'
import { Check } from 'lucide-vue-next'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'

const props = defineProps<{
  open: boolean
  current?: string
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'select', url: string): void
}>()

function select(id: string) {
  emit('select', toPresetUrl(id))
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="$emit('update:open', $event)">
    <DialogContent class="max-w-md">
      <DialogHeader>
        <DialogTitle>选择头像</DialogTitle>
      </DialogHeader>
      <div class="grid grid-cols-4 gap-3 py-2">
        <button
          v-for="preset in AVATAR_PRESETS"
          :key="preset.id"
          @click="select(preset.id)"
          class="relative rounded-xl overflow-hidden focus:outline-none focus-visible:ring-2 focus-visible:ring-ring hover:scale-105 transition-transform"
          :title="preset.label"
        >
          <AvatarPreset :avatar-url="toPresetUrl(preset.id)" :size="72" class="w-full" />
          <div
            v-if="current === toPresetUrl(preset.id)"
            class="absolute inset-0 flex items-center justify-center bg-black/30 rounded-xl"
          >
            <Check class="size-5 text-white" stroke-width="2.5" />
          </div>
        </button>
      </div>
    </DialogContent>
  </Dialog>
</template>
