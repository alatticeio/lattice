<script setup lang="ts">
import { ref, computed, onUnmounted, watch } from 'vue'
import { Copy, Check, RefreshCw, ExternalLink, Container, Loader2 } from 'lucide-vue-next'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

interface SandboxDemoSession {
  workspace_id: string
  expires_at: string
  server_url: string
  token: string
  install_cmd: string
  console_url: string
}

const STORAGE_KEY = 'lattice_demo_sandbox'
const openModel = defineModel<boolean>('open')

type State = 'loading' | 'ready' | 'expired' | 'error'
type Preset = 'claude' | 'python3' | 'curl'

const state = ref<State>('loading')
const session = ref<SandboxDemoSession | null>(null)
const errorMsg = ref('')
const timeLeft = ref('')
const remainingMs = ref(0)
const copiedRun = ref(false)
const preset = ref<Preset>('claude')

let timer: ReturnType<typeof setInterval> | null = null

const timerClass = computed(() => {
  if (remainingMs.value <= 60_000) return 'text-red-500'
  if (remainingMs.value <= 300_000) return 'text-amber-500'
  return 'text-emerald-500'
})

function formatTime(ms: number): string {
  if (ms <= 0) return '0:00'
  const totalSec = Math.floor(ms / 1000)
  const m = Math.floor(totalSec / 60)
  const s = totalSec % 60
  return `${m}:${s.toString().padStart(2, '0')}`
}

function startCountdown(expiresAt: string) {
  if (timer) clearInterval(timer)
  remainingMs.value = Math.max(0, new Date(expiresAt).getTime() - Date.now())
  timeLeft.value = formatTime(remainingMs.value)
  timer = setInterval(() => {
    const ms = new Date(expiresAt).getTime() - Date.now()
    remainingMs.value = Math.max(0, ms)
    if (ms <= 0) {
      timeLeft.value = '0:00'
      state.value = 'expired'
      clearInterval(timer!)
    } else {
      timeLeft.value = formatTime(ms)
    }
  }, 1000)
}

const presets: { value: Preset; label: string; suffix: string }[] = [
  {
    value: 'claude',
    label: 'claude',
    suffix: '-- claude --model claude-opus-4-6',
  },
  {
    value: 'python3',
    label: 'python3',
    suffix: "-- python3 -c \"import urllib.request; print(urllib.request.urlopen('https://httpbin.org/get').read().decode())\"",
  },
  {
    value: 'curl',
    label: 'curl',
    suffix: '-- curl -s https://httpbin.org/get',
  },
]

const runCmd = computed(() => {
  if (!session.value) return ''
  const p = presets.find(x => x.value === preset.value) ?? presets[0]
  return `docker run --rm --cap-add NET_ADMIN ghcr.io/alatticeio/lattice sandbox run demo-agent --server-url ${session.value.server_url} --token ${session.value.token} ${p.suffix}`
})

async function launch() {
  state.value = 'loading'
  errorMsg.value = ''
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const cached: SandboxDemoSession = JSON.parse(raw)
      if (new Date(cached.expires_at).getTime() > Date.now()) {
        // Re-issue a fresh magic token for the existing workspace so the
        // console_url works even after a server restart (magic tokens are
        // in-memory and lost on restart).
        const res = await fetch('/api/v1/demo/sandbox/launch', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ workspace_id: cached.workspace_id }),
        })
        if (res.ok) {
          const body = await res.json()
          if (body.code === 200 && body.data?.console_url) {
            const fresh: SandboxDemoSession = { ...cached, console_url: body.data.console_url }
            localStorage.setItem(STORAGE_KEY, JSON.stringify(fresh))
            session.value = fresh
            state.value = 'ready'
            startCountdown(cached.expires_at)
            return
          }
        }
        // Re-issue failed (workspace deleted/expired) — fall through to create a new session.
        localStorage.removeItem(STORAGE_KEY)
      }
    }
  } catch { /* ignore */ }

  try {
    const res = await fetch('/api/v1/demo/sandbox/launch', { method: 'POST' })
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      errorMsg.value = (body as { message?: string }).message
        ?? (res.status === 429 ? 'Too many sandbox demo sessions. Please try again later.' : 'Failed to launch sandbox demo.')
      state.value = 'error'
      return
    }
    const data: SandboxDemoSession = (await res.json()).data
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
    session.value = data
    state.value = 'ready'
    startCountdown(data.expires_at)
  } catch {
    errorMsg.value = 'Network error. Please check your connection.'
    state.value = 'error'
  }
}

function reset() {
  localStorage.removeItem(STORAGE_KEY)
  session.value = null
  launch()
}

function execCopy(text: string) {
  const el = document.createElement('textarea')
  el.value = text
  el.setAttribute('readonly', '')
  el.style.cssText = 'position:fixed;top:0;left:0;width:2em;height:2em;opacity:0;pointer-events:none'
  // Append inside the dialog so Radix focus trap doesn't steal focus back
  const container = document.querySelector('[role="dialog"]') ?? document.body
  container.appendChild(el)
  el.focus()
  el.select()
  try { document.execCommand('copy') } catch { /* ignore */ }
  container.removeChild(el)
}

async function copy(text: string) {
  if (navigator.clipboard) {
    try { await navigator.clipboard.writeText(text) } catch { execCopy(text) }
  } else {
    execCopy(text)
  }
  copiedRun.value = true
  setTimeout(() => { copiedRun.value = false }, 2000)
}

function openConsole() {
  if (session.value?.console_url) window.open(session.value.console_url, '_blank')
}

watch(openModel, (v) => {
  if (v) {
    launch()
  } else {
    if (timer) { clearInterval(timer); timer = null }
  }
})
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>

<template>
  <Dialog v-model:open="openModel">
    <DialogContent class="max-w-xl p-0 overflow-hidden gap-0">

      <!-- Header -->
      <div class="px-6 pt-6 pb-5 border-b border-border">
        <div class="flex items-center gap-2.5 pr-8">
          <div class="flex items-center justify-center w-8 h-8 rounded-lg bg-primary/10 shrink-0">
            <Container class="size-4 text-primary" />
          </div>
          <div>
            <h2 class="text-base font-semibold leading-none">Try Sandbox</h2>
            <p class="text-xs text-muted-foreground mt-1">Run an AI agent in an isolated network sandbox</p>
          </div>
        </div>
        <!-- Timer badge -->
        <div v-if="state === 'ready'" class="mt-3 inline-flex items-center gap-1.5 rounded-full border border-border px-2.5 py-1 text-xs font-mono font-medium" :class="timerClass">
          <span class="size-1.5 rounded-full bg-current animate-pulse" />
          {{ timeLeft }}
        </div>
      </div>

      <!-- Body -->
      <div class="px-6 py-5">

        <!-- Loading -->
        <div v-if="state === 'loading'" class="flex flex-col items-center gap-3 py-10 text-muted-foreground">
          <Loader2 class="size-5 animate-spin" />
          <span class="text-sm">Setting up your sandbox workspace…</span>
        </div>

        <!-- Error -->
        <div v-else-if="state === 'error'" class="space-y-4 py-2">
          <div class="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3">
            <p class="text-sm text-destructive">{{ errorMsg }}</p>
          </div>
          <Button variant="outline" size="sm" @click="launch" class="gap-1.5">
            <RefreshCw class="size-3.5" /> Try Again
          </Button>
        </div>

        <!-- Expired -->
        <div v-else-if="state === 'expired'" class="space-y-4 py-2">
          <div class="rounded-lg border border-border bg-muted/40 px-4 py-3">
            <p class="text-sm text-muted-foreground">This sandbox session has expired.</p>
          </div>
          <Button variant="outline" size="sm" @click="reset" class="gap-1.5">
            <RefreshCw class="size-3.5" /> Start New Session
          </Button>
        </div>

        <!-- Ready -->
        <div v-else-if="state === 'ready' && session" class="space-y-4">

          <!-- Prerequisite notice -->
          <div class="rounded-lg border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
            Lattice creates a WireGuard interface in the container. Requires <code class="text-[11px] bg-muted px-1 rounded">--cap-add NET_ADMIN</code>. No other dependencies needed.
          </div>

          <!-- Step 1: Run agent -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <span class="flex items-center justify-center size-5 rounded-full bg-primary text-primary-foreground text-[11px] font-bold shrink-0">1</span>
                <span class="text-sm font-medium">Run your agent</span>
              </div>
              <!-- Preset toggles -->
              <div class="flex items-center gap-1.5">
                <div class="flex rounded-md border border-input overflow-hidden text-xs font-mono">
                  <button
                    v-for="p in presets"
                    :key="p.value"
                    type="button"
                    :class="preset === p.value ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground hover:bg-muted/80'"
                    class="px-2 py-0.5 border-l border-input first:border-l-0 transition-colors"
                    @click="preset = p.value"
                  >{{ p.label }}</button>
                </div>
              </div>
            </div>
            <div class="group relative rounded-lg bg-zinc-950 dark:bg-zinc-900 border border-zinc-800 px-4 py-3 pr-12">
              <code class="text-xs text-zinc-100 font-mono break-all whitespace-pre-wrap leading-relaxed">{{ runCmd }}</code>
              <button
                class="absolute top-2.5 right-2.5 flex items-center justify-center size-7 rounded-md transition-colors"
                :class="copiedRun ? 'bg-emerald-500/20 text-emerald-400' : 'bg-zinc-800 text-zinc-400 hover:bg-zinc-700 hover:text-zinc-100'"
                @click="copy(runCmd)"
              >
                <Check v-if="copiedRun" class="size-3.5" />
                <Copy v-else class="size-3.5" />
              </button>
            </div>
          </div>

          <!-- Step 2: Open Console -->
          <div class="space-y-2">
            <div class="flex items-center gap-2">
              <span class="flex items-center justify-center size-5 rounded-full bg-muted text-muted-foreground text-[11px] font-bold shrink-0">2</span>
              <span class="text-sm font-medium">View agent in console</span>
            </div>
            <div class="rounded-lg bg-zinc-950 dark:bg-zinc-900 border border-zinc-800 px-4 py-3">
              <div class="flex items-center gap-2 font-mono text-xs">
                <span class="text-zinc-500 select-none">#</span>
                <span class="text-zinc-400">Agent appears in /sandbox once the container starts</span>
              </div>
            </div>
          </div>

          <!-- Footer actions -->
          <div class="flex items-center justify-between pt-1">
            <button class="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors" @click="reset">
              <RefreshCw class="size-3" /> New Session
            </button>
            <Button
              v-if="session.console_url"
              size="sm"
              class="gap-1.5 h-8 text-xs"
              @click="openConsole"
            >
              <ExternalLink class="size-3.5" /> Open Console
            </Button>
          </div>
        </div>

      </div>
    </DialogContent>
  </Dialog>
</template>
