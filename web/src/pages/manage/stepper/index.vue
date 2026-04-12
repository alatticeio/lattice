<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import {
  Check, Loader2, Copy, ShieldCheck, ArrowRight, ArrowLeft,
  Network, LayoutGrid, KeyRound, Terminal, Tag, Zap,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

definePage({
  meta: { title: '快速接入', description: '全域边缘节点接入引导程序。' },
})

const currentStep = ref(1)
const isJoining = ref(false)
const copied = ref(false)

const form = reactive({
  workspaceName: '',
  token: 'wf_live_8s2k...92nz',
  nodeLabel: 'edge-node-01',
})

const steps = [
  { id: 1, icon: LayoutGrid, title: '工作空间',  desc: '定义逻辑网络边界' },
  { id: 2, icon: KeyRound,   title: '接入凭证',  desc: '生成节点认证 Token' },
  { id: 3, icon: Terminal,   title: '执行加入',  desc: '运行 Wireflow Join' },
  { id: 4, icon: Tag,        title: '节点配置',  desc: '配置节点元数据标签' },
  { id: 5, icon: Zap,        title: '完成接入',  desc: '拓扑同步并注册节点' },
]

const canProceed = computed(() => currentStep.value === 1 ? !!form.workspaceName.trim() : true)
const isLastStep = computed(() => currentStep.value === steps.length)
const isDone = computed(() => currentStep.value > steps.length)

function prevStep() { if (currentStep.value > 1) currentStep.value-- }
function nextStep() {
  if (currentStep.value < steps.length) currentStep.value++
  else currentStep.value = steps.length + 1
}
function handleJoin() {
  isJoining.value = true
  setTimeout(() => { isJoining.value = false; nextStep() }, 2000)
}
function copyToken() {
  navigator.clipboard.writeText(form.token)
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}
const joinCommand = computed(
  () => `curl -sSL https://get.wireflow.io | sudo bash -s -- join --token ${form.token}`
)
function copyCommand() {
  navigator.clipboard.writeText(joinCommand.value)
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}
</script>

<template>
  <div class="flex items-start justify-center p-6 xl:p-10 2xl:p-16 min-h-full animate-in fade-in duration-300">

    <!-- ── Success ─────────────────────────────────────────────────── -->
    <div v-if="isDone" class="w-full max-w-2xl xl:max-w-3xl 2xl:max-w-4xl mt-8 animate-in zoom-in-95 duration-500">
      <div class="bg-card border border-border rounded-2xl p-12 xl:p-16 2xl:p-20 text-center shadow-sm space-y-6">
        <div class="relative mx-auto size-24 xl:size-32 flex items-center justify-center">
          <div class="absolute inset-0 rounded-full bg-emerald-500/10 animate-ping opacity-40" />
          <div class="absolute inset-2 rounded-full bg-emerald-500/10 animate-ping opacity-20 animation-delay-150" />
          <div class="relative size-24 xl:size-32 rounded-full bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center">
            <Check class="size-10 xl:size-14 text-emerald-500 stroke-[2.5]" />
          </div>
        </div>
        <div class="space-y-2">
          <h2 class="text-3xl xl:text-4xl 2xl:text-5xl font-black tracking-tighter">接入成功</h2>
          <p class="text-muted-foreground text-sm max-w-sm mx-auto leading-relaxed">
            加密隧道建立完毕，节点
            <code class="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">{{ form.nodeLabel }}</code>
            已注册至工作空间
            <code class="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">{{ form.workspaceName }}</code>。
          </p>
        </div>
        <!-- Summary stats -->
        <div class="grid grid-cols-3 gap-3 max-w-xs mx-auto">
          <div class="bg-muted/30 rounded-xl p-3">
            <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">状态</p>
            <p class="text-xs font-black text-emerald-500">在线</p>
          </div>
          <div class="bg-muted/30 rounded-xl p-3">
            <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">延迟</p>
            <p class="text-xs font-black">12 ms</p>
          </div>
          <div class="bg-muted/30 rounded-xl p-3">
            <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">加密</p>
            <p class="text-xs font-black">WG</p>
          </div>
        </div>
        <div class="flex flex-col sm:flex-row gap-3 justify-center pt-2">
          <Button size="lg" class="gap-2">
            <Network class="size-4" /> 进入控制台
          </Button>
          <Button variant="outline" size="lg">查看拓扑图</Button>
        </div>
      </div>
    </div>

    <!-- ── Wizard ──────────────────────────────────────────────────── -->
    <div v-else class="w-full max-w-4xl xl:max-w-5xl 2xl:max-w-6xl">
      <div class="bg-card border border-border rounded-2xl shadow-sm overflow-hidden flex min-h-[520px] xl:min-h-[640px] 2xl:min-h-[760px]">

        <!-- ── Left: Step rail ──────────────────────────────────────── -->
        <div class="w-64 xl:w-72 2xl:w-80 shrink-0 border-r border-border bg-muted/20 flex flex-col">
          <!-- Rail header -->
          <div class="px-6 py-6 xl:px-8 xl:py-7 border-b border-border">
            <div class="flex items-center gap-2.5">
              <div class="size-8 xl:size-10 rounded-lg bg-primary flex items-center justify-center shrink-0">
                <Network class="size-4 xl:size-5 text-primary-foreground" />
              </div>
              <div>
                <p class="text-xs xl:text-sm font-black uppercase tracking-wider">快速接入</p>
                <p class="text-[10px] xl:text-xs text-muted-foreground/60">Node Onboarding</p>
              </div>
            </div>
          </div>

          <!-- Step list -->
          <nav class="flex-1 px-3 xl:px-4 py-4 xl:py-5 space-y-0.5 xl:space-y-1">
            <div v-for="(step, i) in steps" :key="step.id" class="relative">
              <!-- Connecting line -->
              <div
                v-if="i < steps.length - 1"
                class="absolute left-[22px] top-9 w-px h-3.5 transition-colors duration-500"
                :class="currentStep > step.id ? 'bg-primary/50' : 'bg-border'"
              />

              <button
                class="w-full flex items-center gap-3 px-3 xl:px-4 py-2.5 xl:py-3 rounded-lg transition-all duration-200 text-left"
                :class="currentStep === step.id
                  ? 'bg-primary/10 text-primary'
                  : currentStep > step.id
                    ? 'text-muted-foreground hover:bg-muted/50'
                    : 'text-muted-foreground/40 cursor-default'"
                :disabled="currentStep < step.id"
              >
                <!-- Icon / Done indicator -->
                <div
                  class="size-5 xl:size-6 rounded-full flex items-center justify-center shrink-0 transition-all duration-300"
                  :class="currentStep > step.id
                    ? 'bg-primary text-primary-foreground'
                    : currentStep === step.id
                      ? 'bg-primary/15 text-primary ring-2 ring-primary/20'
                      : 'bg-muted text-muted-foreground/30'"
                >
                  <Check v-if="currentStep > step.id" class="size-3 xl:size-3.5 stroke-[3]" />
                  <component v-else :is="step.icon" class="size-3 xl:size-3.5" />
                </div>

                <div class="min-w-0">
                  <p class="text-xs xl:text-sm font-semibold leading-none truncate">{{ step.title }}</p>
                  <p class="text-[10px] xl:text-xs mt-0.5 truncate opacity-70">{{ step.desc }}</p>
                </div>
              </button>
            </div>
          </nav>

          <!-- Progress indicator -->
          <div class="px-6 xl:px-8 py-5 xl:py-6 border-t border-border">
            <div class="flex justify-between text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-2">
              <span>进度</span>
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
          <div class="px-8 xl:px-12 2xl:px-16 pt-8 xl:pt-10 pb-6 xl:pb-8 border-b border-border/60">
            <div class="flex items-center gap-2 mb-3">
              <span class="text-[10px] xl:text-xs font-black tracking-[0.2em] uppercase text-muted-foreground/40 tabular-nums">
                STEP {{ String(currentStep).padStart(2, '0') }} / {{ String(steps.length).padStart(2, '0') }}
              </span>
            </div>
            <h2 class="text-2xl xl:text-3xl 2xl:text-4xl font-black tracking-tight">
              <span v-if="currentStep === 1">命名您的工作空间</span>
              <span v-else-if="currentStep === 2">生成接入凭证</span>
              <span v-else-if="currentStep === 3">执行 Wireflow Join</span>
              <span v-else-if="currentStep === 4">配置节点元数据</span>
              <span v-else>验证节点状态</span>
            </h2>
            <p class="mt-1.5 text-sm xl:text-base text-muted-foreground leading-relaxed">
              <span v-if="currentStep === 1">工作空间是节点和策略的逻辑边界，名称将用作路由标识符。</span>
              <span v-else-if="currentStep === 2">Token 是节点握手的唯一凭证，请妥善保管，不要泄露。</span>
              <span v-else-if="currentStep === 3">在目标机器终端执行以下命令，完成节点注册。</span>
              <span v-else-if="currentStep === 4">为节点设置标识符，便于在拓扑视图中识别和管理。</span>
              <span v-else>正在检测节点连接状态并同步拓扑信息。</span>
            </p>
          </div>

          <!-- Content body -->
          <div class="flex-1 px-8 xl:px-12 2xl:px-16 py-7 xl:py-10">

            <!-- Step 1: Workspace -->
            <div v-if="currentStep === 1" class="space-y-5 animate-in slide-in-from-right-3 duration-300">
              <div class="space-y-1.5">
                <label class="text-xs font-semibold text-foreground/60 uppercase tracking-wider">工作空间名称</label>
                <Input
                  v-model="form.workspaceName"
                  placeholder="production-cluster-us"
                  class="h-11 font-mono text-sm"
                  autofocus
                />
                <p class="text-xs text-muted-foreground/50">建议使用小写字母、数字和连字符，例如 <code class="font-mono text-xs">us-west-prod</code></p>
              </div>
              <!-- Preview -->
              <div v-if="form.workspaceName" class="flex items-center gap-3 p-4 rounded-xl border border-primary/20 bg-primary/5 animate-in fade-in duration-200">
                <div class="size-9 rounded-lg bg-primary/15 flex items-center justify-center shrink-0">
                  <LayoutGrid class="size-4 text-primary" />
                </div>
                <div>
                  <p class="text-sm font-bold text-primary">{{ form.workspaceName }}</p>
                  <p class="text-[11px] text-muted-foreground/60 font-mono mt-0.5">namespace · 0 nodes</p>
                </div>
                <div class="ml-auto">
                  <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20">就绪</span>
                </div>
              </div>
            </div>

            <!-- Step 2: Token -->
            <div v-else-if="currentStep === 2" class="space-y-4 animate-in slide-in-from-right-3 duration-300">
              <div class="space-y-1.5">
                <label class="text-xs font-semibold text-foreground/60 uppercase tracking-wider">接入 Token</label>
                <div class="flex items-center gap-2 h-11 px-3 rounded-lg border border-border bg-muted/20 font-mono text-sm">
                  <ShieldCheck class="size-4 text-emerald-500 shrink-0" />
                  <span class="flex-1 truncate text-foreground/60 text-xs">{{ form.token }}</span>
                  <button class="shrink-0 p-1 rounded hover:bg-muted transition-colors text-muted-foreground hover:text-foreground" @click="copyToken">
                    <Check v-if="copied" class="size-3.5 text-emerald-500" />
                    <Copy v-else class="size-3.5" />
                  </button>
                </div>
              </div>
              <div class="flex items-start gap-2.5 rounded-xl bg-amber-500/5 border border-amber-500/20 p-4">
                <ShieldCheck class="size-4 text-amber-500 mt-0.5 shrink-0" />
                <p class="text-xs text-amber-600/80 dark:text-amber-400/80 leading-relaxed">
                  此 Token 仅显示一次，请立即复制并保存至安全位置。Token 与工作空间 <strong>{{ form.workspaceName }}</strong> 绑定。
                </p>
              </div>
            </div>

            <!-- Step 3: Join command -->
            <div v-else-if="currentStep === 3" class="space-y-4 animate-in slide-in-from-right-3 duration-300">
              <div class="rounded-xl bg-zinc-950 border border-zinc-800 overflow-hidden shadow-lg">
                <div class="flex items-center gap-1.5 px-4 py-2.5 border-b border-zinc-800/80 bg-zinc-900/50">
                  <div class="size-2.5 rounded-full bg-rose-500/70" />
                  <div class="size-2.5 rounded-full bg-amber-500/70" />
                  <div class="size-2.5 rounded-full bg-emerald-500/70" />
                  <span class="ml-2 text-[11px] text-zinc-500 font-mono flex-1">bash — terminal</span>
                  <button
                    class="flex items-center gap-1.5 text-[11px] text-zinc-500 hover:text-zinc-200 transition-colors px-2 py-0.5 rounded hover:bg-zinc-800"
                    @click="copyCommand"
                  >
                    <Check v-if="copied" class="size-3 text-emerald-400" />
                    <Copy v-else class="size-3" />
                    {{ copied ? 'Copied!' : 'Copy' }}
                  </button>
                </div>
                <div class="p-5">
                  <div class="flex gap-3 font-mono text-sm leading-relaxed">
                    <span class="text-zinc-500 select-none mt-0.5">$</span>
                    <code class="text-emerald-400/90 break-all text-xs leading-6">{{ joinCommand }}</code>
                  </div>
                </div>
              </div>
              <p class="text-xs text-muted-foreground/60 leading-relaxed">确保目标机器已开放出站连接，执行完毕后点击「检测并继续」。</p>
            </div>

            <!-- Step 4: Node label -->
            <div v-else-if="currentStep === 4" class="space-y-5 animate-in slide-in-from-right-3 duration-300">
              <div class="space-y-1.5">
                <label class="text-xs font-semibold text-foreground/60 uppercase tracking-wider">节点标识符</label>
                <Input v-model="form.nodeLabel" class="h-11 font-mono text-sm" placeholder="edge-node-01" />
                <p class="text-xs text-muted-foreground/50">此标识符将在拓扑视图和策略规则中引用。</p>
              </div>
              <!-- Node preview -->
              <div v-if="form.nodeLabel" class="flex items-center gap-3 p-4 rounded-xl border border-border bg-muted/20 animate-in fade-in duration-200">
                <div class="size-9 rounded-lg bg-emerald-500/10 flex items-center justify-center shrink-0">
                  <div class="size-2 rounded-full bg-emerald-500 animate-pulse" />
                </div>
                <div>
                  <p class="text-sm font-bold">{{ form.nodeLabel }}</p>
                  <p class="text-[11px] text-muted-foreground/60 font-mono mt-0.5">{{ form.workspaceName }} · pending</p>
                </div>
              </div>
            </div>

            <!-- Step 5: Verify -->
            <div v-else class="animate-in slide-in-from-right-3 duration-300 space-y-4">
              <div
                class="flex items-center gap-4 p-5 rounded-xl border transition-colors duration-300"
                :class="isJoining ? 'border-primary/20 bg-primary/5' : 'border-emerald-500/20 bg-emerald-500/5'"
              >
                <div
                  class="size-12 rounded-full flex items-center justify-center shrink-0 transition-colors duration-300"
                  :class="isJoining ? 'bg-primary/10' : 'bg-emerald-500/10'"
                >
                  <Loader2 v-if="isJoining" class="size-5 text-primary animate-spin" />
                  <Check v-else class="size-5 text-emerald-500 stroke-[2.5]" />
                </div>
                <div>
                  <p class="text-sm font-bold">
                    {{ isJoining ? '正在检测节点连接...' : '准备就绪，点击完成接入' }}
                  </p>
                  <p class="text-xs text-muted-foreground mt-0.5">
                    {{ isJoining ? '建立 WireGuard 隧道，通常需要 5–10 秒' : '系统将同步拓扑并完成节点注册' }}
                  </p>
                </div>
              </div>
              <!-- Config summary -->
              <div class="grid grid-cols-2 gap-3">
                <div class="p-3 rounded-lg border border-border bg-muted/20">
                  <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">工作空间</p>
                  <p class="text-xs font-bold font-mono truncate">{{ form.workspaceName }}</p>
                </div>
                <div class="p-3 rounded-lg border border-border bg-muted/20">
                  <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">节点标识符</p>
                  <p class="text-xs font-bold font-mono truncate">{{ form.nodeLabel }}</p>
                </div>
              </div>
            </div>

          </div>

          <!-- Content footer / Navigation -->
          <div class="border-t border-border/60 px-8 xl:px-12 2xl:px-16 py-4 xl:py-5 flex items-center justify-between bg-muted/10">
            <Button
              variant="ghost"
              :disabled="currentStep === 1"
              class="gap-2 text-muted-foreground"
              @click="prevStep"
            >
              <ArrowLeft class="size-4" /> 上一步
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

            <Button v-if="currentStep === 3" :disabled="isJoining" class="gap-2" @click="handleJoin">
              <Loader2 v-if="isJoining" class="size-4 animate-spin" />
              {{ isJoining ? '检测中...' : '检测并继续' }}
              <ArrowRight v-if="!isJoining" class="size-4" />
            </Button>

            <Button v-else-if="isLastStep" :disabled="!canProceed" class="gap-2" @click="nextStep">
              完成接入 <Check class="size-4" />
            </Button>

            <Button v-else :disabled="!canProceed" class="gap-2" @click="nextStep">
              继续 <ArrowRight class="size-4" />
            </Button>
          </div>

        </div>
      </div>
    </div>

  </div>
</template>
