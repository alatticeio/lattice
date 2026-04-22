<script setup lang="ts">
import { ref, computed } from 'vue'
import { ShieldCheck, Smartphone, Monitor, Chrome, AlertTriangle, Eye, EyeOff, LogOut } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import Switch from '@/components/ui/switch/Switch.vue'
import UserSettingsNav from '@/components/UserSettingsNav.vue'

definePage({
  meta: { title: 'Account', description: 'Manage your account security and settings.' },
})

const email = ref('admin@example.com')
const newEmail = ref('')
const showCurrent = ref(false)
const showNew = ref(false)
const showConfirm = ref(false)
const passwords = ref({ current: '', next: '', confirm: '' })
const twoFactor = ref(false)

const sessions = [
  { device: 'Chrome on macOS',    icon: Chrome,     location: 'San Francisco, CA', time: 'Active now',  current: true },
  { device: 'Safari on iPhone',   icon: Smartphone, location: 'San Francisco, CA', time: '2 hours ago', current: false },
  { device: 'Firefox on Windows', icon: Monitor,    location: 'New York, NY',      time: '3 days ago',  current: false },
]

const pwStrength = computed(() => {
  const len = passwords.value.next.length
  if (len === 0)  return { level: 0, label: '',          bar: '',              text: '' }
  if (len < 6)    return { level: 1, label: 'Too short', bar: 'bg-rose-500',   text: 'text-rose-500' }
  if (len < 10)   return { level: 2, label: 'Fair',      bar: 'bg-amber-400',  text: 'text-amber-500' }
  if (len < 14)   return { level: 3, label: 'Good',      bar: 'bg-blue-500',   text: 'text-blue-500' }
  return               { level: 4, label: 'Strong',   bar: 'bg-emerald-500', text: 'text-emerald-500' }
})
</script>

<template>
  <div class="flex flex-col">
    <UserSettingsNav />

    <div class="mx-auto w-full max-w-3xl space-y-5 p-6">

      <!-- Email -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 border-b border-border">
          <h2 class="text-sm font-semibold">Email Address</h2>
          <p class="text-xs text-muted-foreground mt-0.5">
            Current: <span class="font-mono text-foreground">{{ email }}</span>
          </p>
        </div>
        <div class="p-6 space-y-3">
          <div class="space-y-1.5">
            <label class="text-xs font-medium text-foreground/80">New email address</label>
            <Input v-model="newEmail" type="email" placeholder="new@example.com" />
          </div>
          <div class="space-y-1.5">
            <label class="text-xs font-medium text-foreground/80">Confirm with password</label>
            <Input type="password" placeholder="Your current password" />
          </div>
          <Button size="sm">Update email</Button>
        </div>
      </div>

      <!-- Password -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 border-b border-border">
          <h2 class="text-sm font-semibold">Change Password</h2>
          <p class="text-xs text-muted-foreground mt-0.5">Use a strong password you don't use elsewhere.</p>
        </div>
        <div class="p-6 space-y-4">
          <div class="space-y-1.5">
            <label class="text-xs font-medium text-foreground/80">Current password</label>
            <div class="relative">
              <Input v-model="passwords.current" :type="showCurrent ? 'text' : 'password'" placeholder="••••••••" class="pr-9" />
              <button class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors" @click="showCurrent = !showCurrent">
                <EyeOff v-if="showCurrent" class="size-4" /><Eye v-else class="size-4" />
              </button>
            </div>
          </div>
          <div class="space-y-1.5">
            <label class="text-xs font-medium text-foreground/80">New password</label>
            <div class="relative">
              <Input v-model="passwords.next" :type="showNew ? 'text' : 'password'" placeholder="••••••••" class="pr-9" />
              <button class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors" @click="showNew = !showNew">
                <EyeOff v-if="showNew" class="size-4" /><Eye v-else class="size-4" />
              </button>
            </div>
          </div>
          <div class="space-y-1.5">
            <label class="text-xs font-medium text-foreground/80">Confirm new password</label>
            <div class="relative">
              <Input v-model="passwords.confirm" :type="showConfirm ? 'text' : 'password'" placeholder="••••••••" class="pr-9" />
              <button class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors" @click="showConfirm = !showConfirm">
                <EyeOff v-if="showConfirm" class="size-4" /><Eye v-else class="size-4" />
              </button>
            </div>
          </div>
          <div v-if="passwords.next.length > 0" class="space-y-1.5">
            <div class="flex gap-1">
              <div v-for="i in 4" :key="i" class="h-1 flex-1 rounded-full transition-all duration-300"
                :class="i <= pwStrength.level ? pwStrength.bar : 'bg-muted'" />
            </div>
            <p class="text-xs" :class="pwStrength.text">{{ pwStrength.label }}</p>
          </div>
          <Button size="sm">Update password</Button>
        </div>
      </div>

      <!-- 2FA -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 flex items-center justify-between gap-4" :class="twoFactor ? 'border-b border-border' : ''">
          <div class="flex items-center gap-3">
            <div class="size-8 rounded-lg flex items-center justify-center shrink-0"
              :class="twoFactor ? 'bg-emerald-500/10' : 'bg-muted'">
              <ShieldCheck class="size-4" :class="twoFactor ? 'text-emerald-500' : 'text-muted-foreground'" />
            </div>
            <div>
              <h2 class="text-sm font-semibold">Two-Factor Authentication</h2>
              <p class="text-xs text-muted-foreground">{{ twoFactor ? 'Enabled — authenticator app configured.' : 'Add an extra layer of security.' }}</p>
            </div>
          </div>
          <Switch v-model="twoFactor" />
        </div>
        <div v-if="twoFactor" class="p-6">
          <div class="flex items-center gap-5 p-4 rounded-xl bg-muted/30 border border-border">
            <div class="size-20 shrink-0 bg-white rounded-lg p-1.5 border border-border">
              <div class="grid size-full grid-cols-5 gap-px">
                <div v-for="i in 25" :key="i" class="rounded-[1px]"
                  :class="[1,2,3,4,5,6,10,11,15,16,20,21,22,23,24,25,7,13,19].includes(i) ? 'bg-black' : 'bg-white'" />
              </div>
            </div>
            <div>
              <p class="text-sm font-medium">Scan with your authenticator app</p>
              <p class="text-xs text-muted-foreground mt-1">Or enter the setup key manually:</p>
              <code class="text-primary text-sm tracking-widest mt-1.5 block font-mono">JBSW Y3DP EBLW 64TM</code>
            </div>
          </div>
        </div>
      </div>

      <!-- Sessions -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 border-b border-border flex items-center justify-between">
          <div>
            <h2 class="text-sm font-semibold">Active Sessions</h2>
            <p class="text-xs text-muted-foreground mt-0.5">Manage all signed-in devices.</p>
          </div>
          <Button variant="ghost" size="sm" class="gap-1.5 text-xs text-muted-foreground hover:text-destructive">
            <LogOut class="size-3.5" /> Revoke all
          </Button>
        </div>
        <div class="divide-y divide-border">
          <div v-for="session in sessions" :key="session.device" class="flex items-center gap-4 px-6 py-4">
            <div class="size-9 rounded-xl bg-muted flex items-center justify-center shrink-0">
              <component :is="session.icon" class="size-4 text-muted-foreground" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <p class="text-sm font-medium truncate">{{ session.device }}</p>
                <span v-if="session.current"
                  class="shrink-0 text-[10px] font-semibold px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20">
                  Current
                </span>
              </div>
              <p class="text-xs text-muted-foreground">{{ session.location }} · {{ session.time }}</p>
            </div>
            <Button v-if="!session.current" variant="ghost" size="sm" class="shrink-0 text-xs text-muted-foreground hover:text-destructive">
              Revoke
            </Button>
          </div>
        </div>
      </div>

      <!-- Danger zone -->
      <div class="border border-destructive/30 rounded-xl overflow-hidden">
        <div class="px-6 py-3.5 border-b border-destructive/20 bg-destructive/5 flex items-center gap-2">
          <AlertTriangle class="size-3.5 text-destructive" />
          <h2 class="text-sm font-semibold text-destructive">Danger Zone</h2>
        </div>
        <div class="px-6 py-5 flex items-center justify-between gap-6">
          <div>
            <p class="text-sm font-medium">Delete this account</p>
            <p class="text-xs text-muted-foreground mt-0.5">Permanently remove your account and all data. This cannot be undone.</p>
          </div>
          <Button variant="destructive" size="sm" class="shrink-0">Delete account</Button>
        </div>
      </div>

    </div>
  </div>
</template>
