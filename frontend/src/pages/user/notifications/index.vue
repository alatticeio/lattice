<script setup lang="ts">
import { ref } from 'vue'
import Switch from '@/components/ui/switch/Switch.vue'
import { Button } from '@/components/ui/button'
import UserSettingsNav from '@/components/UserSettingsNav.vue'
import { MessageSquare, AtSign, Heart, UserPlus, ShieldAlert, Megaphone, Mail, Smartphone, Bell } from 'lucide-vue-next'

definePage({
  meta: { title: 'Notifications', description: 'Choose what you want to be notified about.' },
})

interface NotifSetting {
  label: string
  description: string
  icon: typeof Bell
  email: boolean
  push: boolean
  inApp: boolean
}

const settings = ref<NotifSetting[]>([
  { label: 'Comments',          description: 'When someone comments on your posts or projects.', icon: MessageSquare, email: true,  push: true,  inApp: true  },
  { label: 'Mentions',          description: 'When someone mentions you with @username.',        icon: AtSign,        email: true,  push: true,  inApp: true  },
  { label: 'Likes & Reactions', description: 'When someone likes or reacts to your content.',   icon: Heart,         email: false, push: true,  inApp: true  },
  { label: 'New Followers',     description: 'When someone starts following you.',              icon: UserPlus,      email: false, push: false, inApp: true  },
  { label: 'Security Alerts',   description: 'Important alerts about your account security.',  icon: ShieldAlert,   email: true,  push: true,  inApp: true  },
  { label: 'Product Updates',   description: 'New features, releases, and announcements.',     icon: Megaphone,     email: true,  push: false, inApp: false },
])

const channels = ref({ email: true, push: true, inApp: true })
const digestFreq = ref<'realtime' | 'daily' | 'weekly'>('daily')

function toggleAll(ch: 'email' | 'push' | 'inApp') {
  const next = !channels.value[ch]
  channels.value[ch] = next
  settings.value.forEach(s => { s[ch] = next })
}

function save() { /* submit */ }
</script>

<template>
  <div class="flex flex-col">
    <UserSettingsNav />

    <div class="mx-auto w-full max-w-3xl space-y-5 p-6">

      <!-- Channels -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 border-b border-border">
          <h2 class="text-sm font-semibold">Notification Channels</h2>
          <p class="text-xs text-muted-foreground mt-0.5">Master toggles — disabling a channel silences all its notifications.</p>
        </div>
        <div class="p-6 grid gap-3 sm:grid-cols-3">
          <div
            v-for="ch in [
              { key: 'email' as const, label: 'Email',  icon: Mail,       value: channels.email },
              { key: 'push'  as const, label: 'Push',   icon: Smartphone, value: channels.push  },
              { key: 'inApp' as const, label: 'In-App', icon: Bell,       value: channels.inApp },
            ]"
            :key="ch.key"
            class="flex flex-col gap-3 p-4 rounded-xl border border-border bg-background transition-opacity"
            :class="ch.value ? '' : 'opacity-50'"
          >
            <div class="flex items-center justify-between">
              <div class="size-8 rounded-lg flex items-center justify-center transition-colors"
                :class="ch.value ? 'bg-primary/10' : 'bg-muted'">
                <component :is="ch.icon" class="size-4 transition-colors" :class="ch.value ? 'text-primary' : 'text-muted-foreground'" />
              </div>
              <Switch :model-value="ch.value" @update:model-value="toggleAll(ch.key)" />
            </div>
            <div>
              <p class="text-sm font-medium">{{ ch.label }}</p>
              <p class="text-xs text-muted-foreground">{{ ch.value ? 'Enabled' : 'Disabled' }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Per-event matrix -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 border-b border-border">
          <h2 class="text-sm font-semibold">Event Preferences</h2>
          <p class="text-xs text-muted-foreground mt-0.5">Fine-tune which channels receive each event type.</p>
        </div>
        <div class="grid grid-cols-[1fr_56px_56px_56px] items-center gap-2 px-6 py-2.5 border-b border-border bg-muted/20">
          <span class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60">Event</span>
          <span class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60 text-center">Email</span>
          <span class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60 text-center">Push</span>
          <span class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60 text-center">In-App</span>
        </div>
        <div class="divide-y divide-border">
          <div
            v-for="item in settings" :key="item.label"
            class="grid grid-cols-[1fr_56px_56px_56px] items-center gap-2 px-6 py-3.5 hover:bg-muted/20 transition-colors"
          >
            <div class="flex items-center gap-3 min-w-0">
              <div class="size-8 rounded-lg bg-muted flex items-center justify-center shrink-0">
                <component :is="item.icon" class="size-3.5 text-muted-foreground" />
              </div>
              <div class="min-w-0">
                <p class="text-sm font-medium leading-none">{{ item.label }}</p>
                <p class="text-xs text-muted-foreground mt-1 truncate">{{ item.description }}</p>
              </div>
            </div>
            <div class="flex justify-center"><Switch v-model="item.email" :disabled="!channels.email" /></div>
            <div class="flex justify-center"><Switch v-model="item.push"  :disabled="!channels.push"  /></div>
            <div class="flex justify-center"><Switch v-model="item.inApp" :disabled="!channels.inApp" /></div>
          </div>
        </div>
      </div>

      <!-- Digest -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 border-b border-border">
          <h2 class="text-sm font-semibold">Email Digest</h2>
          <p class="text-xs text-muted-foreground mt-0.5">Receive a grouped summary instead of individual emails.</p>
        </div>
        <div class="p-6">
          <div class="grid grid-cols-3 gap-2">
            <button
              v-for="opt in [
                { id: 'realtime' as const, label: 'Real-time', desc: 'Immediately' },
                { id: 'daily'    as const, label: 'Daily',     desc: 'Once a day'  },
                { id: 'weekly'   as const, label: 'Weekly',    desc: 'Once a week' },
              ]"
              :key="opt.id"
              class="flex flex-col items-center gap-0.5 py-3.5 px-2 rounded-xl border text-sm font-medium transition-all"
              :class="digestFreq === opt.id
                ? 'border-primary bg-primary/5 text-primary ring-1 ring-primary/20'
                : 'border-border text-muted-foreground hover:text-foreground hover:border-muted-foreground/30'"
              @click="digestFreq = opt.id"
            >
              {{ opt.label }}
              <span class="text-[10px] font-normal opacity-60">{{ opt.desc }}</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Save -->
      <div class="flex justify-end gap-2">
        <Button variant="outline">Cancel</Button>
        <Button @click="save">Save preferences</Button>
      </div>

    </div>
  </div>
</template>
