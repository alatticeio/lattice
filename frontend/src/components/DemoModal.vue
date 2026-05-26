<script setup lang="ts">
import { ref, onUnmounted, watch } from 'vue'
import { Copy, Check, RefreshCw, ExternalLink } from 'lucide-vue-next'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

interface DemoSession {
  workspace_id: string
  expires_at: string
  device1_cmd: string
  device2_cmd: string
  console_url: string
}

const STORAGE_KEY = 'lattice_demo'

const openModel = defineModel<boolean>('open')

type State = 'loading' | 'ready' | 'expired' | 'error'

const state = ref<State>('loading')
const session = ref<DemoSession | null>(null)
const errorMsg = ref('')
const timeLeft = ref('')
const copied1 = ref(false)
const copied2 = ref(false)

let timer: ReturnType<typeof setInterval> | null = null

function formatTime(ms: number): string {
  if (ms <= 0) return '0:00'
  const totalSec = Math.floor(ms / 1000)
  const m = Math.floor(totalSec / 60)
  const s = totalSec % 60
  return `${m}:${s.toString().padStart(2, '0')}`
}

function startCountdown(expiresAt: string) {
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    const ms = new Date(expiresAt).getTime() - Date.now()
    if (ms <= 0) {
      timeLeft.value = '0:00'
      state.value = 'expired'
      clearInterval(timer!)
    } else {
      timeLeft.value = formatTime(ms)
    }
  }, 1000)
}

async function launch() {
  state.value = 'loading'
  errorMsg.value = ''

  // Check localStorage first
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const cached: DemoSession = JSON.parse(raw)
      if (new Date(cached.expires_at).getTime() > Date.now()) {
        session.value = cached
        state.value = 'ready'
        startCountdown(cached.expires_at)
        return
      }
    }
  } catch {
    // ignore parse errors
  }

  // Call API
  try {
    const res = await fetch('/api/v1/demo/launch', { method: 'POST' })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      const fallback = res.status === 429
        ? 'Too many demo sessions from your network. Please try again later.'
        : 'Failed to launch demo. Please try again.'
      errorMsg.value = (body as { message?: string }).message ?? fallback
      state.value = 'error'
      return
    }
    const body = await res.json()
    const data: DemoSession = body.data
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
    session.value = data
    state.value = 'ready'
    startCountdown(data.expires_at)
  } catch {
    errorMsg.value = 'Network error. Please check your connection and try again.'
    state.value = 'error'
  }
}

function reset() {
  localStorage.removeItem(STORAGE_KEY)
  session.value = null
  launch()
}

async function copy(text: string, which: 1 | 2) {
  await navigator.clipboard.writeText(text)
  if (which === 1) {
    copied1.value = true
    setTimeout(() => { copied1.value = false }, 2000)
  } else {
    copied2.value = true
    setTimeout(() => { copied2.value = false }, 2000)
  }
}

function openConsole(url: string) {
  window.open(url, '_blank')
}

watch(openModel, (v) => {
  if (v) launch()
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <Dialog v-model:open="openModel">
    <DialogContent class="max-w-lg">
      <DialogHeader>
        <DialogTitle>Zero-Friction Demo</DialogTitle>
        <p v-if="state === 'ready' && session" class="text-sm text-muted-foreground mt-1">
          Expires in: <span class="font-mono font-semibold">{{ timeLeft }}</span>
        </p>
      </DialogHeader>

      <!-- Loading -->
      <div v-if="state === 'loading'" class="flex items-center justify-center py-10 text-muted-foreground text-sm">
        Setting up demo workspace...
      </div>

      <!-- Error -->
      <div v-else-if="state === 'error'" class="space-y-4 py-2">
        <p class="text-sm text-destructive">{{ errorMsg }}</p>
        <Button variant="outline" size="sm" @click="launch">Try Again</Button>
      </div>

      <!-- Expired -->
      <div v-else-if="state === 'expired'" class="space-y-4 py-2">
        <p class="text-sm text-muted-foreground">This demo session has expired.</p>
        <Button variant="outline" size="sm" @click="reset">
          <RefreshCw class="mr-2 h-4 w-4" /> Start New Demo
        </Button>
      </div>

      <!-- Ready -->
      <div v-else-if="state === 'ready' && session" class="space-y-5 py-2">
        <div class="space-y-2">
          <p class="text-sm font-medium">Step 1 — Run on Device 1</p>
          <div class="relative rounded-md bg-muted p-3 font-mono text-xs break-all pr-10">
            {{ session.device1_cmd }}
            <Button
              variant="ghost" size="icon"
              class="absolute top-1.5 right-1.5 h-6 w-6"
              @click="copy(session!.device1_cmd, 1)"
            >
              <Check v-if="copied1" class="h-3 w-3 text-green-500" />
              <Copy v-else class="h-3 w-3" />
            </Button>
          </div>
        </div>

        <div class="space-y-2">
          <p class="text-sm font-medium">Step 2 — Run on Device 2</p>
          <div class="relative rounded-md bg-muted p-3 font-mono text-xs break-all pr-10">
            {{ session.device2_cmd }}
            <Button
              variant="ghost" size="icon"
              class="absolute top-1.5 right-1.5 h-6 w-6"
              @click="copy(session!.device2_cmd, 2)"
            >
              <Check v-if="copied2" class="h-3 w-3 text-green-500" />
              <Copy v-else class="h-3 w-3" />
            </Button>
          </div>
        </div>

        <div class="space-y-1">
          <p class="text-sm font-medium">Step 3 — Verify (on either device)</p>
          <div class="rounded-md bg-muted p-3 font-mono text-xs space-y-1">
            <div>lattice status</div>
            <div>ping &lt;peer-ip&gt;</div>
          </div>
        </div>

        <div class="flex items-center justify-between pt-2">
          <Button variant="ghost" size="sm" @click="reset">
            <RefreshCw class="mr-2 h-3 w-3" /> Start New Demo
          </Button>
          <Button
            v-if="session.console_url"
            size="sm"
            class="gap-1.5"
            @click="openConsole(session!.console_url)"
          >
            <ExternalLink class="h-3.5 w-3.5" /> Open Console
          </Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
