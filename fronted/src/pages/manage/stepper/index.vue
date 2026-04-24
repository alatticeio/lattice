<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  Check, Loader2, Copy, ShieldCheck, ArrowRight, ArrowLeft,
  Network, LayoutGrid, KeyRound, Terminal, Tag, Shield, Plus, X,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'vue-sonner'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore, getWsInitials } from '@/stores/workspace'
import { add as addWorkspace } from '@/api/workspace'
import { create as createToken } from '@/api/token'
import { createPolicy } from '@/api/policy'

definePage({
  meta: { titleKey: 'manage.stepper.title', descKey: 'manage.stepper.desc' },
})

const { t } = useI18n()
const router = useRouter()
const workspaceStore = useWorkspaceStore()
const { currentWorkspace } = storeToRefs(workspaceStore)

const hasWorkspace = computed(() => !!currentWorkspace.value)

const currentStep = ref(1)
const isDone = ref(false)
const copied = ref<string | null>(null)

const steps = computed(() => [
  { id: 1, icon: LayoutGrid, title: t('manage.stepper.steps.workspace'),  desc: t('manage.stepper.steps.workspaceDesc') },
  { id: 2, icon: KeyRound,   title: t('manage.stepper.steps.token'),      desc: t('manage.stepper.steps.tokenDesc') },
  { id: 3, icon: Terminal,   title: t('manage.stepper.steps.join'),       desc: t('manage.stepper.steps.joinDesc') },
  { id: 4, icon: Tag,        title: t('manage.stepper.steps.labels'),     desc: t('manage.stepper.steps.labelsDesc') },
  { id: 5, icon: Shield,     title: t('manage.stepper.steps.policy'),     desc: t('manage.stepper.steps.policyDesc') },
])

// ── Step 1: Workspace ─────────────────────────────────────────────
const wsForm = reactive({ displayName: '' })
const wsLoading = ref(false)

async function handleStep1Next() {
  if (hasWorkspace.value) {
    currentStep.value = 2
    await fetchToken()
    return
  }
  if (!wsForm.displayName.trim()) { toast.error(t('manage.stepper.step1.nameRequired')); return }
  wsLoading.value = true
  try {
    const { data, code } = await addWorkspace({
      displayName: wsForm.displayName.trim(),
      slug: wsForm.displayName.trim().toLowerCase().replace(/[\s_]+/g, '-'),
    }) as any
    if (code === 200 && data) {
      workspaceStore.switchWorkspace(data)
      workspaceStore.fetchAll()
      currentStep.value = 2
      await fetchToken()
    } else {
      toast.error(t('manage.stepper.step1.createFailed'))
    }
  } catch {
    toast.error(t('manage.stepper.step1.createFailed'))
  } finally {
    wsLoading.value = false
  }
}

// ── Step 2: Token ─────────────────────────────────────────────────
const generatedToken = ref('')
const tokenLoading = ref(false)

async function fetchToken() {
  if (generatedToken.value) return
  tokenLoading.value = true
  try {
    const { data, code } = await createToken({}) as any
    if (code === 200) {
      generatedToken.value = data?.token ?? (typeof data === 'string' ? data : '')
    } else {
      toast.error(t('manage.stepper.step2.tokenFailed'))
    }
  } catch {
    toast.error(t('manage.stepper.step2.tokenFailed'))
  } finally {
    tokenLoading.value = false
  }
}

const joinCommand = computed(() =>
  `wireflow join --token ${generatedToken.value || '<token>'}`)

// ── Copy helper ───────────────────────────────────────────────────
async function copyText(text: string, key: string) {
  await navigator.clipboard.writeText(text)
  copied.value = key
  setTimeout(() => { copied.value = null }, 2000)
}

// ── Step 4: Node labels ───────────────────────────────────────────
const nodeLabels = ref<{ key: string; value: string }[]>([{ key: '', value: '' }])
function addLabel() { nodeLabels.value.push({ key: '', value: '' }) }
function removeLabel(i: number) { nodeLabels.value.splice(i, 1) }
const labelMap = computed(() => {
  const m: Record<string, string> = {}
  nodeLabels.value.forEach(l => { if (l.key.trim()) m[l.key.trim()] = l.value.trim() })
  return m
})

// ── Step 5: Policy ────────────────────────────────────────────────
const policyForm = reactive({ name: '', action: 'Allow' as 'Allow' | 'Deny' })
const policyLoading = ref(false)

async function handleCreatePolicy() {
  if (!policyForm.name.trim()) { toast.error(t('manage.stepper.step5.nameRequired')); return }
  policyLoading.value = true
  try {
    const payload: Record<string, any> = {
      name: policyForm.name.trim(),
      action: policyForm.action,
      policyTypes: ['Ingress', 'Egress'],
    }
    if (Object.keys(labelMap.value).length) {
      payload.peerSelector = { matchLabels: labelMap.value }
    }
    const { code } = await createPolicy(payload) as any
    if (code === 200) {
      isDone.value = true
    } else {
      toast.error(t('manage.stepper.step5.createFailed'))
    }
  } catch {
    toast.error(t('manage.stepper.step5.createFailed'))
  } finally {
    policyLoading.value = false
  }
}

// ── Navigation ────────────────────────────────────────────────────
function prevStep() { if (currentStep.value > 1) currentStep.value-- }
function nextStep() { if (currentStep.value < steps.value.length) currentStep.value++ }

const canProceed = computed(() => {
  if (currentStep.value === 1) return hasWorkspace.value || !!wsForm.displayName.trim()
  if (currentStep.value === 2) return !!generatedToken.value && !tokenLoading.value
  return true
})
</script>

<template>
  <div class="flex items-start justify-center p-4 xl:p-6 2xl:p-10 min-h-full animate-in fade-in duration-300">

    <!-- ── Success ─────────────────────────────────────────────────── -->
    <div v-if="isDone" class="w-full max-w-xl xl:max-w-2xl mt-6 animate-in zoom-in-95 duration-500">
      <div class="bg-card border border-border rounded-2xl p-8 xl:p-10 text-center shadow-sm space-y-5">
        <div class="relative mx-auto size-20 xl:size-24 flex items-center justify-center">
          <div class="absolute inset-0 rounded-full bg-emerald-500/10 animate-ping opacity-40" />
          <div class="absolute inset-2 rounded-full bg-emerald-500/10 animate-ping opacity-20 animation-delay-150" />
          <div class="relative size-20 xl:size-24 rounded-full bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center">
            <Check class="size-9 xl:size-11 text-emerald-500 stroke-[2.5]" />
          </div>
        </div>
        <div class="space-y-2">
          <h2 class="text-2xl xl:text-3xl font-black tracking-tighter">{{ t('manage.stepper.done.title') }}</h2>
          <p class="text-muted-foreground text-sm max-w-sm mx-auto leading-relaxed">
            {{ t('manage.stepper.done.desc', { ws: currentWorkspace?.displayName }) }}
          </p>
        </div>
        <div class="grid grid-cols-2 gap-3 max-w-xs mx-auto">
          <div class="bg-muted/30 rounded-xl p-3">
            <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">{{ t('manage.stepper.done.workspaceLabel') }}</p>
            <p class="text-xs font-black truncate">{{ currentWorkspace?.displayName }}</p>
          </div>
          <div class="bg-muted/30 rounded-xl p-3">
            <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">{{ t('manage.stepper.done.policyLabel') }}</p>
            <p class="text-xs font-black text-emerald-500">{{ policyForm.action }}</p>
          </div>
        </div>
        <div class="flex flex-col sm:flex-row gap-3 justify-center pt-2">
          <Button size="lg" class="gap-2" @click="router.push('/manage/nodes')">
            <Network class="size-4" /> {{ t('manage.stepper.done.viewNodes') }}
          </Button>
          <Button variant="outline" size="lg" @click="router.push('/manage/topology')">
            {{ t('manage.stepper.done.viewTopology') }}
          </Button>
        </div>
      </div>
    </div>

    <!-- ── Wizard ──────────────────────────────────────────────────── -->
    <div v-else class="w-full max-w-3xl xl:max-w-4xl">
      <div class="bg-card border border-border rounded-2xl shadow-sm overflow-hidden flex min-h-[460px] xl:min-h-[500px]">

        <!-- ── Left: Step rail ──────────────────────────────────────── -->
        <div class="w-52 xl:w-60 shrink-0 border-r border-border bg-muted/20 flex flex-col">
          <!-- Rail header -->
          <div class="px-5 py-4 xl:px-6 xl:py-5 border-b border-border">
            <div class="flex items-center gap-2.5">
              <img src="@/assets/logo.svg" class="size-7 xl:size-8 shrink-0" alt="Wireflow" />
              <div>
                <p class="text-xs font-black uppercase tracking-wider">{{ t('manage.stepper.title') }}</p>
                <p class="text-[10px] text-muted-foreground/60">Node Onboarding</p>
              </div>
            </div>
          </div>

          <!-- Step list -->
          <nav class="flex-1 px-3 py-3 space-y-0.5">
            <div v-for="(step, i) in steps" :key="step.id" class="relative">
              <div
                v-if="i < steps.length - 1"
                class="absolute left-[19px] top-8 w-px h-3 transition-colors duration-500"
                :class="currentStep > step.id ? 'bg-primary/50' : 'bg-border'"
              />
              <button
                class="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg transition-all duration-200 text-left"
                :class="currentStep === step.id
                  ? 'bg-primary/10 text-primary'
                  : currentStep > step.id
                    ? 'text-muted-foreground hover:bg-muted/50'
                    : 'text-muted-foreground/40 cursor-default'"
                :disabled="currentStep < step.id"
              >
                <div
                  class="size-5 rounded-full flex items-center justify-center shrink-0 transition-all duration-300"
                  :class="currentStep > step.id
                    ? 'bg-primary text-primary-foreground'
                    : currentStep === step.id
                      ? 'bg-primary/15 text-primary ring-2 ring-primary/20'
                      : 'bg-muted text-muted-foreground/30'"
                >
                  <Check v-if="currentStep > step.id" class="size-3 stroke-[3]" />
                  <component v-else :is="step.icon" class="size-3" />
                </div>
                <div class="min-w-0">
                  <p class="text-xs font-semibold leading-none truncate">{{ step.title }}</p>
                  <p class="text-[10px] mt-0.5 truncate opacity-70">{{ step.desc }}</p>
                </div>
              </button>
            </div>
          </nav>

          <!-- Progress indicator -->
          <div class="px-5 xl:px-6 py-4 border-t border-border">
            <div class="flex justify-between text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-2">
              <span>{{ t('manage.stepper.progress') }}</span>
              <span>{{ Math.round(((currentStep - 1) / steps.length) * 100) }}%</span>
            </div>
            <div class="h-1 bg-muted rounded-full overflow-hidden">
              <div
                class="h-full bg-primary rounded-full transition-all duration-500"
                :style="{ width: ((currentStep - 1) / steps.length * 100) + '%' }"
              />
            </div>
          </div>
        </div>

        <!-- ── Right: Content ───────────────────────────────────────── -->
        <div class="flex-1 flex flex-col min-w-0">

          <!-- Content header -->
          <div class="px-6 xl:px-8 pt-6 xl:pt-7 pb-4 xl:pb-5 border-b border-border/60">
            <div class="flex items-center gap-2 mb-2">
              <span class="text-[10px] font-black tracking-[0.2em] uppercase text-muted-foreground/40 tabular-nums">
                STEP {{ String(currentStep).padStart(2, '0') }} / {{ String(steps.length).padStart(2, '0') }}
              </span>
            </div>
            <h2 class="text-xl xl:text-2xl font-black tracking-tight">
              <span v-if="currentStep === 1">{{ hasWorkspace ? t('manage.stepper.step1.titleHas') : t('manage.stepper.step1.titleCreate') }}</span>
              <span v-else-if="currentStep === 2">{{ t('manage.stepper.step2.title') }}</span>
              <span v-else-if="currentStep === 3">{{ t('manage.stepper.step3.title') }}</span>
              <span v-else-if="currentStep === 4">{{ t('manage.stepper.step4.title') }}</span>
              <span v-else>{{ t('manage.stepper.step5.title') }}</span>
            </h2>
            <p class="mt-1 text-sm text-muted-foreground leading-relaxed">
              <span v-if="currentStep === 1 && hasWorkspace">{{ t('manage.stepper.step1.descHas') }}</span>
              <span v-else-if="currentStep === 1">{{ t('manage.stepper.step1.descCreate') }}</span>
              <span v-else-if="currentStep === 2">{{ t('manage.stepper.step2.desc') }}</span>
              <span v-else-if="currentStep === 3">{{ t('manage.stepper.step3.desc') }}</span>
              <span v-else-if="currentStep === 4">{{ t('manage.stepper.step4.desc') }}</span>
              <span v-else>{{ t('manage.stepper.step5.desc') }}</span>
            </p>
          </div>

          <!-- Content body -->
          <div class="flex-1 px-6 xl:px-8 py-5 xl:py-6">

            <!-- ── Step 1: Workspace ─────────────────────────────────── -->
            <div v-if="currentStep === 1" class="space-y-4 animate-in slide-in-from-right-3 duration-300">

              <div v-if="hasWorkspace" class="flex items-center gap-3 p-4 rounded-xl border border-primary/20 bg-primary/5">
                <div class="size-10 rounded-lg bg-primary/15 flex items-center justify-center shrink-0 text-primary font-black text-sm">
                  {{ getWsInitials(currentWorkspace!.displayName) }}
                </div>
                <div class="min-w-0 flex-1">
                  <p class="font-bold text-sm text-primary truncate">{{ currentWorkspace!.displayName }}</p>
                  <p class="text-[11px] text-muted-foreground/60 font-mono mt-0.5 truncate">
                    {{ currentWorkspace!.namespace ?? currentWorkspace!.slug }}
                  </p>
                </div>
                <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20 shrink-0">
                  {{ t('manage.stepper.step1.currentTag') }}
                </span>
              </div>

              <template v-else>
                <div class="flex items-start gap-2.5 rounded-xl bg-amber-500/5 border border-amber-500/20 p-3.5">
                  <ShieldCheck class="size-4 text-amber-500 mt-0.5 shrink-0" />
                  <p class="text-xs text-amber-600/80 dark:text-amber-400/80 leading-relaxed">
                    {{ t('manage.stepper.step1.noWsWarn') }}
                  </p>
                </div>
                <div class="space-y-1.5">
                  <label class="text-xs font-semibold text-foreground/60 uppercase tracking-wider">{{ t('manage.stepper.step1.nameLabel') }}</label>
                  <Input
                    v-model="wsForm.displayName"
                    placeholder="production-cluster"
                    class="h-11 font-mono text-sm"
                    autofocus
                  />
                  <p class="text-xs text-muted-foreground/50">
                    {{ t('manage.stepper.step1.nameHint') }}
                  </p>
                </div>
                <div v-if="wsForm.displayName" class="flex items-center gap-3 p-4 rounded-xl border border-border bg-muted/20 animate-in fade-in duration-200">
                  <div class="size-9 rounded-lg bg-primary/15 flex items-center justify-center shrink-0 text-primary font-black text-xs">
                    {{ getWsInitials(wsForm.displayName) }}
                  </div>
                  <div>
                    <p class="text-sm font-bold">{{ wsForm.displayName }}</p>
                    <p class="text-[11px] text-muted-foreground/60 font-mono mt-0.5">
                      slug: {{ wsForm.displayName.toLowerCase().replace(/[\s_]+/g, '-') }}
                    </p>
                  </div>
                  <span class="ml-auto text-[10px] font-semibold px-2 py-0.5 rounded-full bg-muted text-muted-foreground ring-1 ring-border">
                    {{ t('manage.stepper.step1.pendingTag') }}
                  </span>
                </div>
              </template>
            </div>

            <!-- ── Step 2: Token ─────────────────────────────────────── -->
            <div v-else-if="currentStep === 2" class="space-y-4 animate-in slide-in-from-right-3 duration-300">
              <div class="space-y-1.5">
                <label class="text-xs font-semibold text-foreground/60 uppercase tracking-wider">{{ t('manage.stepper.step2.tokenLabel') }}</label>
                <div class="flex items-center gap-2 h-11 px-3 rounded-lg border border-border bg-muted/20 font-mono">
                  <ShieldCheck class="size-4 text-emerald-500 shrink-0" />
                  <span v-if="tokenLoading" class="flex-1 flex items-center gap-2 text-xs text-muted-foreground">
                    <Loader2 class="size-3.5 animate-spin" /> {{ t('manage.stepper.step2.generating') }}
                  </span>
                  <span v-else class="flex-1 truncate text-foreground/70 text-xs">{{ generatedToken || '—' }}</span>
                  <button
                    v-if="!tokenLoading && generatedToken"
                    class="shrink-0 p-1 rounded hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
                    @click="copyText(generatedToken, 'token')"
                  >
                    <Check v-if="copied === 'token'" class="size-3.5 text-emerald-500" />
                    <Copy v-else class="size-3.5" />
                  </button>
                </div>
              </div>
              <div class="flex items-start gap-2.5 rounded-xl bg-amber-500/5 border border-amber-500/20 p-4">
                <ShieldCheck class="size-4 text-amber-500 mt-0.5 shrink-0" />
                <p class="text-xs text-amber-600/80 dark:text-amber-400/80 leading-relaxed">
                  {{ t('manage.stepper.step2.warning', { ws: currentWorkspace?.displayName }) }}
                </p>
              </div>
            </div>

            <!-- ── Step 3: Join command ───────────────────────────────── -->
            <div v-else-if="currentStep === 3" class="space-y-4 animate-in slide-in-from-right-3 duration-300">
              <div class="rounded-xl bg-zinc-950 border border-zinc-800 overflow-hidden shadow-lg">
                <div class="flex items-center gap-1.5 px-4 py-2.5 border-b border-zinc-800/80 bg-zinc-900/50">
                  <div class="size-2.5 rounded-full bg-rose-500/70" />
                  <div class="size-2.5 rounded-full bg-amber-500/70" />
                  <div class="size-2.5 rounded-full bg-emerald-500/70" />
                  <span class="ml-2 text-[11px] text-zinc-500 font-mono flex-1">bash — terminal</span>
                  <button
                    class="flex items-center gap-1.5 text-[11px] text-zinc-500 hover:text-zinc-200 transition-colors px-2 py-0.5 rounded hover:bg-zinc-800"
                    @click="copyText(joinCommand, 'cmd')"
                  >
                    <Check v-if="copied === 'cmd'" class="size-3 text-emerald-400" />
                    <Copy v-else class="size-3" />
                    {{ copied === 'cmd' ? 'Copied!' : 'Copy' }}
                  </button>
                </div>
                <div class="p-5">
                  <div class="flex gap-3 font-mono text-sm leading-relaxed">
                    <span class="text-zinc-500 select-none mt-0.5">$</span>
                    <code class="text-emerald-400/90 break-all text-xs leading-6">{{ joinCommand }}</code>
                  </div>
                </div>
              </div>
              <p class="text-xs text-muted-foreground/60 leading-relaxed">
                {{ t('manage.stepper.step3.hint') }}
              </p>
            </div>

            <!-- ── Step 4: Node labels ────────────────────────────────── -->
            <div v-else-if="currentStep === 4" class="space-y-4 animate-in slide-in-from-right-3 duration-300">
              <div class="space-y-2">
                <label class="text-xs font-semibold text-foreground/60 uppercase tracking-wider">{{ t('manage.stepper.step4.labelsLabel') }}</label>
                <div
                  v-for="(label, i) in nodeLabels" :key="i"
                  class="flex items-center gap-2"
                >
                  <Input v-model="label.key"   placeholder="key"   class="h-9 font-mono text-xs flex-1" />
                  <span class="text-muted-foreground text-sm font-mono shrink-0">=</span>
                  <Input v-model="label.value" placeholder="value" class="h-9 font-mono text-xs flex-1" />
                  <button
                    class="shrink-0 size-8 flex items-center justify-center rounded-lg hover:bg-destructive/10 text-muted-foreground/40 hover:text-destructive transition-colors"
                    @click="removeLabel(i)"
                  >
                    <X class="size-3.5" />
                  </button>
                </div>
                <button
                  class="flex items-center gap-1.5 text-xs text-primary font-semibold hover:underline mt-1"
                  @click="addLabel"
                >
                  <Plus class="size-3.5" /> {{ t('manage.stepper.step4.addLabel') }}
                </button>
              </div>

              <div v-if="Object.keys(labelMap).length" class="p-3.5 rounded-xl border border-border bg-muted/20">
                <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-2">
                  {{ t('manage.stepper.step4.previewTitle') }}
                </p>
                <div class="flex flex-wrap gap-1.5">
                  <span
                    v-for="(v, k) in labelMap" :key="k"
                    class="font-mono text-[11px] px-2 py-0.5 rounded bg-primary/8 text-primary ring-1 ring-primary/20"
                  >{{ k }}={{ v }}</span>
                </div>
              </div>

              <p class="text-xs text-muted-foreground/50 leading-relaxed">
                {{ t('manage.stepper.step4.hint') }}
              </p>
            </div>

            <!-- ── Step 5: Policy ─────────────────────────────────────── -->
            <div v-else class="space-y-4 animate-in slide-in-from-right-3 duration-300">
              <div class="grid grid-cols-2 gap-3">
                <div class="space-y-1.5 col-span-2">
                  <label class="text-xs font-semibold text-foreground/60 uppercase tracking-wider">{{ t('manage.stepper.step5.nameLabel') }}</label>
                  <Input
                    v-model="policyForm.name"
                    :placeholder="`${currentWorkspace?.displayName ?? 'workspace'}-policy`"
                    class="h-9 font-mono text-xs"
                  />
                </div>
                <div class="space-y-1.5">
                  <label class="text-xs font-semibold text-foreground/60 uppercase tracking-wider">{{ t('manage.stepper.step5.actionLabel') }}</label>
                  <select
                    v-model="policyForm.action"
                    class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 transition-[color,box-shadow]"
                  >
                    <option value="Allow">{{ t('manage.stepper.step5.allowOption') }}</option>
                    <option value="Deny">{{ t('manage.stepper.step5.denyOption') }}</option>
                  </select>
                </div>
                <div class="space-y-1.5">
                  <label class="text-xs font-semibold text-foreground/60 uppercase tracking-wider">{{ t('manage.stepper.step5.directionLabel') }}</label>
                  <div class="h-9 flex items-center px-3 rounded-md border border-border bg-muted/20 text-xs text-muted-foreground">
                    Ingress + Egress
                  </div>
                </div>
              </div>

              <div class="p-3.5 rounded-xl border border-border bg-muted/20">
                <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-2">
                  {{ t('manage.stepper.step5.selectorTitle') }}
                </p>
                <div v-if="Object.keys(labelMap).length" class="flex flex-wrap gap-1.5">
                  <span
                    v-for="(v, k) in labelMap" :key="k"
                    class="font-mono text-[11px] px-2 py-0.5 rounded bg-primary/8 text-primary ring-1 ring-primary/20"
                  >{{ k }}={{ v }}</span>
                </div>
                <p v-else class="text-xs text-muted-foreground/40 italic">
                  {{ t('manage.stepper.step5.noLabels') }}
                </p>
              </div>
            </div>

          </div>

          <!-- Content footer / Navigation -->
          <div class="border-t border-border/60 px-6 xl:px-8 py-3 xl:py-4 flex items-center justify-between bg-muted/10">
            <Button
              variant="ghost"
              :disabled="currentStep === 1"
              class="gap-2 text-muted-foreground"
              @click="prevStep"
            >
              <ArrowLeft class="size-4" /> {{ t('manage.stepper.nav.prev') }}
            </Button>

            <!-- Dot progress -->
            <div class="flex items-center gap-1.5">
              <div
                v-for="step in steps" :key="step.id"
                class="rounded-full transition-all duration-300"
                :class="currentStep === step.id
                  ? 'w-5 h-1.5 bg-primary'
                  : currentStep > step.id
                    ? 'size-1.5 bg-primary/40'
                    : 'size-1.5 bg-border'"
              />
            </div>

            <!-- Step 1 -->
            <Button
              v-if="currentStep === 1"
              :disabled="!canProceed || wsLoading"
              class="gap-2"
              @click="handleStep1Next"
            >
              <Loader2 v-if="wsLoading" class="size-4 animate-spin" />
              <template v-else>
                {{ hasWorkspace ? t('manage.stepper.nav.confirmContinue') : t('manage.stepper.nav.createContinue') }}
                <ArrowRight class="size-4" />
              </template>
            </Button>

            <!-- Step 5 -->
            <div v-else-if="currentStep === 5" class="flex items-center gap-2">
              <Button variant="ghost" class="text-muted-foreground text-sm" @click="isDone = true">
                {{ t('manage.stepper.nav.skip') }}
              </Button>
              <Button :disabled="policyLoading || !policyForm.name.trim()" class="gap-2" @click="handleCreatePolicy">
                <Loader2 v-if="policyLoading" class="size-4 animate-spin" />
                <template v-else>
                  {{ t('manage.stepper.nav.createFinish') }} <Check class="size-4" />
                </template>
              </Button>
            </div>

            <!-- Steps 2–4 -->
            <Button v-else :disabled="!canProceed" class="gap-2" @click="nextStep">
              {{ t('manage.stepper.nav.next') }} <ArrowRight class="size-4" />
            </Button>
          </div>

        </div>
      </div>
    </div>

  </div>
</template>
