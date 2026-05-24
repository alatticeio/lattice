<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'vue-sonner'
import { Save, Loader2, Server } from 'lucide-vue-next'
import {
  getPlatformSettings,
  updatePlatformSettings,
} from '@/api/platform'

definePage({
  meta: { titleKey: 'settings.platform.title', descKey: 'settings.platform.desc' },
})

const { t } = useI18n()

const natsUrl = ref('')
const loading = ref(false)
const saving = ref(false)

async function fetchSettings() {
  loading.value = true
  try {
    const { data } = await getPlatformSettings() as any
    natsUrl.value = data?.nats_url ?? ''
  } catch {
    toast.error(t('settings.platform.loadFailed'))
  } finally {
    loading.value = false
  }
}

function validate(): string | null {
  const v = natsUrl.value.trim()
  if (!v) return t('settings.platform.validationRequired')
  if (!v.startsWith('nats://') && !v.startsWith('nats+tls://')) {
    return t('settings.platform.validationPrefix')
  }
  return null
}

async function handleSave() {
  const err = validate()
  if (err) {
    toast.error(err)
    return
  }
  saving.value = true
  try {
    await updatePlatformSettings({ nats_url: natsUrl.value.trim() })
    toast.success(t('settings.platform.saved'))
  } catch {
    toast.error(t('settings.platform.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(fetchSettings)
</script>

<template>
  <div class="flex flex-col gap-6 p-6 animate-in fade-in duration-300">
    <!-- Header -->
    <div>
      <h1 class="text-xl font-semibold tracking-tight">{{ t('settings.platform.title') }}</h1>
      <p class="text-sm text-muted-foreground mt-1">{{ t('settings.platform.desc') }}</p>
    </div>

    <!-- Loading state -->
    <div v-if="loading" class="flex items-center justify-center py-16">
      <Loader2 class="size-6 animate-spin text-muted-foreground" />
    </div>

    <!-- Settings form -->
    <div v-else class="max-w-xl space-y-6">
      <!-- NATS URL -->
      <div class="space-y-2">
        <label class="text-sm font-medium flex items-center gap-1.5">
          <Server class="size-4 text-muted-foreground" />
          {{ t('settings.platform.natsUrlLabel') }}
        </label>
        <Input
          v-model="natsUrl"
          :placeholder="t('settings.platform.natsUrlPlaceholder')"
          class="font-mono text-sm"
        />
        <p class="text-xs text-muted-foreground">{{ t('settings.platform.natsUrlHint') }}</p>
      </div>

      <!-- Save button -->
      <Button :disabled="saving" @click="handleSave" class="gap-1.5">
        <Save v-if="!saving" class="size-4" />
        <Loader2 v-else class="size-4 animate-spin" />
        {{ saving ? t('settings.platform.saving') : t('settings.platform.saveBtn') }}
      </Button>
    </div>
  </div>
</template>
