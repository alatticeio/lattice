<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { toast } from 'vue-sonner'
import { Save, Loader2 } from 'lucide-vue-next'
import {
  getPlatformSettings,
  updatePlatformSettings,
} from '@/api/platform'

definePage({
  meta: { titleKey: 'settings.platform.title', descKey: 'settings.platform.desc' },
})

const { t } = useI18n()

// NATS fields
const natsScheme = ref<'nats://' | 'nats+tls://'>('nats://')
const natsHost = ref('')
const natsPort = ref('4222')

// STUN fields
const stunHost = ref('')
const stunPort = ref('3478')

const loading = ref(false)
const saving = ref(false)

function parseNatsURL(url: string) {
  if (!url) return
  const scheme = url.startsWith('nats+tls://') ? 'nats+tls://' : 'nats://'
  natsScheme.value = scheme
  const rest = url.slice(scheme.length)
  const colon = rest.lastIndexOf(':')
  if (colon !== -1) {
    natsHost.value = rest.slice(0, colon)
    natsPort.value = rest.slice(colon + 1)
  } else {
    natsHost.value = rest
  }
}

function parseStunURL(url: string) {
  if (!url) return
  const body = url.startsWith('stun:') ? url.slice(5) : url
  const colon = body.lastIndexOf(':')
  if (colon !== -1) {
    stunHost.value = body.slice(0, colon)
    stunPort.value = body.slice(colon + 1)
  } else {
    stunHost.value = body
  }
}

const natsPreview = computed(() =>
  natsHost.value ? `${natsScheme.value}${natsHost.value}:${natsPort.value || '4222'}` : '',
)

const stunPreview = computed(() =>
  stunHost.value ? `stun:${stunHost.value}:${stunPort.value || '3478'}` : '',
)

async function fetchSettings() {
  loading.value = true
  try {
    const { data } = await getPlatformSettings() as any
    parseNatsURL(data?.nats_url ?? '')
    parseStunURL(data?.stun_url ?? '')
  } catch {
    toast.error(t('settings.platform.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    await updatePlatformSettings({ nats_url: natsPreview.value, stun_url: stunPreview.value })
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
    <!-- Loading state -->
    <div v-if="loading" class="flex items-center justify-center py-16">
      <Loader2 class="size-6 animate-spin text-muted-foreground" />
    </div>

    <!-- Settings form -->
    <div v-else class="max-w-lg space-y-8">
      <!-- NATS URL -->
      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <label class="text-sm font-medium">{{ t('settings.platform.natsUrlLabel') }}</label>
          <!-- scheme toggle -->
          <div class="flex rounded-md border border-input overflow-hidden text-xs font-mono">
            <button
              type="button"
              :class="natsScheme === 'nats://' ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground hover:bg-muted/80'"
              class="px-2 py-0.5 transition-colors"
              @click="natsScheme = 'nats://'"
            >nats://</button>
            <button
              type="button"
              :class="natsScheme === 'nats+tls://' ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground hover:bg-muted/80'"
              class="px-2 py-0.5 border-l border-input transition-colors"
              @click="natsScheme = 'nats+tls://'"
            >nats+tls://</button>
          </div>
        </div>
        <div class="flex h-9 overflow-hidden rounded-md border border-input bg-background text-sm font-mono ring-offset-background focus-within:ring-1 focus-within:ring-ring focus-within:ring-offset-2">
          <input
            v-model="natsHost"
            :placeholder="t('settings.platform.hostPlaceholder')"
            class="flex-1 min-w-0 bg-transparent px-3 placeholder:text-muted-foreground focus:outline-none"
          />
          <span class="flex items-center border-x border-input bg-muted px-2 text-muted-foreground select-none">:</span>
          <input
            v-model="natsPort"
            placeholder="4222"
            class="w-16 bg-transparent px-2 placeholder:text-muted-foreground focus:outline-none"
          />
        </div>
        <p v-if="natsPreview" class="text-xs font-mono text-muted-foreground bg-muted/50 rounded px-2 py-1">
          {{ natsPreview }}
        </p>
        <p class="text-xs text-muted-foreground">{{ t('settings.platform.natsUrlHint') }}</p>
      </div>

      <!-- STUN URL -->
      <div class="space-y-2">
        <label class="text-sm font-medium">{{ t('settings.platform.stunUrlLabel') }}</label>
        <div class="flex h-9 overflow-hidden rounded-md border border-input bg-background text-sm font-mono ring-offset-background focus-within:ring-1 focus-within:ring-ring focus-within:ring-offset-2">
          <input
            v-model="stunHost"
            :placeholder="t('settings.platform.hostPlaceholder')"
            class="flex-1 min-w-0 bg-transparent px-3 placeholder:text-muted-foreground focus:outline-none"
          />
          <span class="flex items-center border-x border-input bg-muted px-2 text-muted-foreground select-none">:</span>
          <input
            v-model="stunPort"
            placeholder="3478"
            class="w-16 bg-transparent px-2 placeholder:text-muted-foreground focus:outline-none"
          />
        </div>
        <p v-if="stunPreview" class="text-xs font-mono text-muted-foreground bg-muted/50 rounded px-2 py-1">
          {{ stunPreview }}
        </p>
        <p class="text-xs text-muted-foreground">{{ t('settings.platform.stunUrlHint') }}</p>
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
