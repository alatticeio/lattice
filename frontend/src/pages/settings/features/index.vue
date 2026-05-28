<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import {
  Loader2, Bot, MessageSquare, Container, Bell, Radio, Monitor, Globe, Network, ShieldCheck,
  ToggleLeft, Zap, ZapOff,
} from 'lucide-vue-next'
import Switch from '@/components/ui/switch/Switch.vue'
import { useUserStore } from '@/stores/user'
import { useFeatureStore } from '@/stores/feature'
import { updateFeature } from '@/api/feature'
import type { FeatureFlag } from '@/api/feature'

definePage({
  meta: { titleKey: 'settings.features.title', descKey: 'settings.features.desc' },
})

const { t } = useI18n()
const router = useRouter()
const userStore = useUserStore()
const featureStore = useFeatureStore()

const featureMeta: Record<string, { icon: typeof Bot; desc: string }> = {
  'feature.ai_assistant':    { icon: Bot,          desc: t('settings.features.descAi') },
  'feature.mcp_servers':     { icon: MessageSquare, desc: t('settings.features.descMcp') },
  'feature.agent_sandbox':   { icon: Container,    desc: t('settings.features.descSandbox') },
  'feature.alerts':          { icon: Bell,         desc: t('settings.features.descAlerts') },
  'feature.relays':          { icon: Radio,        desc: t('settings.features.descRelays') },
  'feature.monitor':         { icon: Monitor,      desc: t('settings.features.descMonitor') },
  'feature.network_peering': { icon: Globe,        desc: t('settings.features.descNetPeer') },
  'feature.cluster_peering': { icon: Network,      desc: t('settings.features.descClusterPeer') },
  'feature.approvals':       { icon: ShieldCheck,  desc: t('settings.features.descApprovals') },
}

const groupLabels: Record<string, string> = {
  ai: t('settings.features.groupAi'),
  sandbox: t('settings.features.groupSandbox'),
  settings: t('settings.features.groupSettings'),
  workspace: t('settings.features.groupWorkspace'),
  platform: t('settings.features.groupPlatform'),
}

const groupIcons: Record<string, typeof Bot> = {
  ai: Bot,
  sandbox: Container,
  settings: Bell,
  workspace: Monitor,
  platform: Globe,
}

function groupedFlags() {
  const groups: Record<string, FeatureFlag[]> = {}
  for (const f of featureStore.allFlags) {
    if (!groups[f.group]) groups[f.group] = []
    groups[f.group].push(f)
  }
  return groups
}

const totalEnabled = computed(() => featureStore.allFlags.filter(f => f.enabled).length)
const totalDisabled = computed(() => featureStore.allFlags.filter(f => !f.enabled).length)

async function toggleFlag(key: string, enabled: boolean) {
  try {
    await updateFeature(key, enabled)
    featureStore.setFlag(key, enabled)
    toast.success(enabled ? t('settings.features.enabled') : t('settings.features.disabled'))
  } catch {
    toast.error(t('settings.features.updateFailed'))
  }
}

onMounted(async () => {
  if (!userStore.isPlatformAdmin) {
    router.replace('/dashboard')
    return
  }
  if (!featureStore.loaded) {
    await featureStore.fetchFeatures()
  }
})
</script>

<template>
  <div class="flex flex-col">
    <div class="mx-auto w-full max-w-3xl space-y-5 p-6">

      <!-- No permission -->
      <div v-if="!userStore.isPlatformAdmin" class="flex flex-col items-center justify-center py-16 gap-3">
        <ShieldCheck class="size-10 text-muted-foreground" />
        <p class="text-sm text-muted-foreground">{{ t('settings.features.noPermission') }}</p>
      </div>

      <!-- Loading -->
      <div v-else-if="!featureStore.loaded" class="flex items-center justify-center py-16">
        <Loader2 class="size-6 animate-spin text-muted-foreground" />
      </div>

      <template v-else>
        <!-- Summary stats -->
        <div class="grid grid-cols-3 gap-3">
          <div class="flex flex-col gap-1 p-4 rounded-xl border border-border bg-card">
            <div class="flex items-center gap-2">
              <div class="size-7 rounded-lg bg-primary/10 flex items-center justify-center">
                <ToggleLeft class="size-3.5 text-primary" />
              </div>
              <span class="text-xs text-muted-foreground">{{ t('settings.features.statTotal') }}</span>
            </div>
            <span class="text-2xl font-semibold tabular-nums">{{ featureStore.allFlags.length }}</span>
          </div>
          <div class="flex flex-col gap-1 p-4 rounded-xl border border-border bg-card">
            <div class="flex items-center gap-2">
              <div class="size-7 rounded-lg bg-emerald-500/10 flex items-center justify-center">
                <Zap class="size-3.5 text-emerald-500" />
              </div>
              <span class="text-xs text-muted-foreground">{{ t('settings.features.statEnabled') }}</span>
            </div>
            <span class="text-2xl font-semibold tabular-nums text-emerald-500">{{ totalEnabled }}</span>
          </div>
          <div class="flex flex-col gap-1 p-4 rounded-xl border border-border bg-card">
            <div class="flex items-center gap-2">
              <div class="size-7 rounded-lg bg-muted flex items-center justify-center">
                <ZapOff class="size-3.5 text-muted-foreground" />
              </div>
              <span class="text-xs text-muted-foreground">{{ t('settings.features.statDisabled') }}</span>
            </div>
            <span class="text-2xl font-semibold tabular-nums text-muted-foreground">{{ totalDisabled }}</span>
          </div>
        </div>

        <!-- Feature groups -->
        <div v-for="(items, group) in groupedFlags()" :key="group" class="bg-card border border-border rounded-xl overflow-hidden">
          <!-- Group header -->
          <div class="px-6 py-4 border-b border-border flex items-center gap-3">
            <div class="size-8 rounded-lg bg-muted flex items-center justify-center">
              <component :is="groupIcons[group] || ToggleLeft" class="size-4 text-muted-foreground" />
            </div>
            <div>
              <h2 class="text-sm font-semibold">{{ groupLabels[group] || group }}</h2>
              <p class="text-xs text-muted-foreground mt-0.5">
                {{ items.filter(f => f.enabled).length }} / {{ items.length }} {{ t('settings.features.statEnabled').toLowerCase() }}
              </p>
            </div>
          </div>

          <!-- Feature rows -->
          <div class="divide-y divide-border">
            <div
              v-for="item in items"
              :key="item.key"
              class="flex items-center justify-between px-6 py-3.5 hover:bg-muted/20 transition-colors"
            >
              <div class="flex items-center gap-3 min-w-0">
                <div class="size-8 rounded-lg flex items-center justify-center shrink-0 transition-colors"
                  :class="item.enabled ? 'bg-primary/10' : 'bg-muted'">
                  <component
                    :is="featureMeta[item.key]?.icon || ToggleLeft"
                    class="size-3.5 transition-colors"
                    :class="item.enabled ? 'text-primary' : 'text-muted-foreground'"
                  />
                </div>
                <div class="min-w-0">
                  <p class="text-sm font-medium leading-none">{{ item.label }}</p>
                  <p class="text-xs text-muted-foreground mt-1 truncate">{{ featureMeta[item.key]?.desc }}</p>
                </div>
              </div>
              <Switch
                :model-value="item.enabled"
                @update:model-value="(val: boolean) => toggleFlag(item.key, val)"
              />
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
