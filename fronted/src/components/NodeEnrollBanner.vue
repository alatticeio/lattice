<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Terminal, Copy, Check, ChevronDown, ChevronUp, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { create as createToken } from '@/api/token'
import { useWorkspaceStore } from '@/stores/workspace'
import { toast } from 'vue-sonner'

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()

const nodeName    = ref('my-laptop')
const loading     = ref(false)
const DISMISS_KEY = 'lattice:playground:enroll-banner-dismissed'
const dismissed   = ref(localStorage.getItem(DISMISS_KEY) === 'true')
const expanded    = ref(true)
const command     = ref('')
const copied      = ref(false)

const serverURL = computed(() => window.location.origin)

async function generate() {
  const ws = workspaceStore.currentWorkspace
  if (!ws) {
    toast.error(t('manage.nodes.enroll.errorNoWorkspace'))
    return
  }
  const name = nodeName.value.trim()
  if (!name) return

  loading.value = true
  command.value = ''
  try {
    const res = await createToken({}) as any
    const token = res?.data?.token
    if (!token) {
      toast.error(t('manage.nodes.enroll.errorGenerate'))
      return
    }
    command.value = [
      `curl -sSL ${serverURL.value}/install.sh |`,
      `  sh -s -- \\`,
      `  --server ${serverURL.value} \\`,
      `  --token  ${token} \\`,
      `  --name   ${name}`,
    ].join('\n')
  } catch {
    toast.error(t('manage.nodes.enroll.errorGenerate'))
  } finally {
    loading.value = false
  }
}

function dismiss() {
  dismissed.value = true
  localStorage.setItem(DISMISS_KEY, 'true')
}

async function copyCommand() {
  if (!command.value) return
  try {
    await navigator.clipboard.writeText(command.value)
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  } catch {
    toast.error(t('manage.nodes.enroll.copyBtn'))
  }
}
</script>

<template>
  <div
    v-if="!dismissed"
    class="rounded-xl border border-blue-500/20 bg-blue-500/5 px-5 py-4 transition-all"
  >
    <!-- Header row -->
    <div class="flex items-center justify-between gap-3">
      <div class="flex items-center gap-2.5">
        <div class="size-8 rounded-lg bg-blue-500/10 flex items-center justify-center shrink-0">
          <Terminal class="size-4 text-blue-500" />
        </div>
        <div>
          <p class="text-sm font-semibold leading-none">{{ t('manage.nodes.enroll.bannerTitle') }}</p>
          <p class="text-xs text-muted-foreground mt-0.5">{{ t('manage.nodes.enroll.bannerDesc') }}</p>
        </div>
      </div>
      <div class="flex items-center gap-1 shrink-0">
        <button
          class="p-1 rounded hover:bg-muted/60 text-muted-foreground transition-colors"
          @click="expanded = !expanded"
        >
          <ChevronUp v-if="expanded" class="size-4" />
          <ChevronDown v-else class="size-4" />
        </button>
        <button
          class="p-1 rounded hover:bg-muted/60 text-muted-foreground transition-colors"
          @click="dismiss()"
        >
          <X class="size-4" />
        </button>
      </div>
    </div>

    <!-- Body: input + generate + command -->
    <div v-if="expanded" class="mt-4 space-y-3">
      <div class="flex gap-2 items-center max-w-md">
        <Input
          v-model="nodeName"
          :placeholder="t('manage.nodes.enroll.namePlaceholder')"
          class="h-8 text-xs"
          @keydown.enter="generate"
        />
        <Button
          size="sm"
          class="shrink-0"
          :disabled="loading || !nodeName.trim()"
          @click="generate"
        >
          {{ loading ? t('manage.nodes.enroll.generating') : t('manage.nodes.enroll.generateBtn') }}
        </Button>
      </div>

      <!-- Generated command block -->
      <div v-if="command" class="rounded-lg bg-neutral-950 dark:bg-black border border-neutral-800 p-3">
        <div class="flex items-center justify-between mb-2">
          <span class="text-[11px] text-neutral-400 font-medium">{{ t('manage.nodes.enroll.commandTitle') }}</span>
          <div class="flex items-center gap-2">
            <span class="text-[11px] text-neutral-500">{{ t('manage.nodes.enroll.tokenExpiry') }}</span>
            <Button
              size="sm"
              variant="outline"
              class="h-6 px-2 text-[11px] border-neutral-700 text-neutral-300 hover:bg-neutral-800 hover:text-white bg-transparent"
              @click="copyCommand"
            >
              <Check v-if="copied" class="size-3 mr-1 text-emerald-400" />
              <Copy v-else class="size-3 mr-1" />
              {{ copied ? t('manage.nodes.enroll.copied') : t('manage.nodes.enroll.copyBtn') }}
            </Button>
          </div>
        </div>
        <pre class="font-mono text-xs text-neutral-200 whitespace-pre-wrap break-all leading-relaxed">{{ command }}</pre>
      </div>
    </div>
  </div>
</template>
