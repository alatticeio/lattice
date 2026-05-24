<script setup lang="ts">
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface Props {
  open?: boolean
  title?: string
  description?: string
  cancelText?: string
  confirmText?: string
  variant?: 'destructive' | 'default'
}

const props = withDefaults(defineProps<Props>(), {
  title: '确认执行此操作吗？',
  description: '此操作执行后可能无法撤销，请谨慎操作。',
  cancelText: '取消',
  confirmText: '确认执行',
  variant: 'destructive'
})

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'confirm'): void
  (e: 'cancel'): void
}>()
</script>

<template>
  <AlertDialog
    :open="open"
    @update:open="emit('update:open', $event)"
  >
    <AlertDialogTrigger v-if="$slots.default" as-child>
      <slot />
    </AlertDialogTrigger>

    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ title }}</AlertDialogTitle>
        <AlertDialogDescription>
          {{ description }}
        </AlertDialogDescription>
      </AlertDialogHeader>

      <AlertDialogFooter>
        <AlertDialogCancel @click="emit('cancel')">
          {{ cancelText }}
        </AlertDialogCancel>

        <AlertDialogAction
          :class="cn(variant === 'destructive' && buttonVariants({ variant: 'destructive' }))"
          @click="emit('confirm')"
        >
          {{ confirmText }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>