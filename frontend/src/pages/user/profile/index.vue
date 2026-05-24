<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Camera, MapPin, Save, Loader2 } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { toast } from 'vue-sonner'
import UserSettingsNav from '@/components/UserSettingsNav.vue'
import AvatarPreset from '@/components/AvatarPreset.vue'
import AvatarPicker from '@/components/AvatarPicker.vue'
import { useUserStore } from '@/stores/user'
import request from '@/api/request'

definePage({
  meta: { titleKey: 'settings.profile.title', descKey: 'settings.profile.desc' },
})

const userStore = useUserStore()

const loading = ref(false)
const saving = ref(false)
const pickerOpen = ref(false)

const form = ref({
  name: userStore.userInfo?.username ?? '',
  email: userStore.userInfo?.email ?? '',
  avatarUrl: userStore.userInfo?.avatarUrl ?? '',
  title: '',
  company: '',
  bio: '',
  timezone: '',
  language: '',
})

const bioMax = 200
const bioLeft = computed(() => bioMax - (form.value.bio?.length ?? 0))

async function fetchProfile() {
  loading.value = true
  try {
    const { data, code } = await request.post('/profile/getProfile', {}) as any
    if (code === 200 && data) {
      form.value = {
        name: data.name || userStore.userInfo?.username || '',
        email: data.email || userStore.userInfo?.email || '',
        avatarUrl: data.avatarUrl || userStore.userInfo?.avatarUrl || '',
        title: data.title ?? '',
        company: data.company ?? '',
        bio: data.bio ?? '',
        timezone: data.timezone ?? '',
        language: data.language ?? '',
      }
    }
  } catch {
    // fetchProfile 失败时保持 userStore 的基础数据，不影响保存
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const { code } = await request.put('/profile/updateProfile', form.value) as any
    if (code === 200) {
      if (userStore.userInfo) {
        userStore.userInfo.avatarUrl = form.value.avatarUrl
        userStore.userInfo.username = form.value.name
      }
      toast.success('保存成功')
    }
  } catch {
    toast.error('保存失败')
  } finally {
    saving.value = false
  }
}

function onAvatarSelect(url: string) {
  form.value.avatarUrl = url
}

onMounted(fetchProfile)
</script>

<template>
  <div class="flex flex-col">
    <UserSettingsNav />

    <div v-if="loading" class="flex items-center justify-center py-16">
      <Loader2 class="size-6 animate-spin text-muted-foreground" />
    </div>

    <div v-else class="mx-auto w-full max-w-3xl space-y-5 p-6">

      <!-- Avatar -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="h-20 bg-gradient-to-br from-primary/20 via-primary/5 to-transparent" />
        <div class="px-6 pb-5">
          <div class="flex items-end justify-between -mt-8 mb-3">
            <div class="relative">
              <AvatarPreset
                :avatar-url="form.avatarUrl"
                :size="64"
                :fallback="form.name"
                class="ring-4 ring-card"
              />
              <button
                @click="pickerOpen = true"
                class="absolute bottom-0 right-0 size-6 rounded-full bg-card border border-border shadow-sm flex items-center justify-center hover:bg-muted transition-colors"
              >
                <Camera class="size-3" />
              </button>
            </div>
            <div class="flex gap-2">
              <Button variant="outline" size="sm" @click="pickerOpen = true">选择头像</Button>
            </div>
          </div>
          <p class="text-xs text-muted-foreground/60">从预设图标中选择你的头像</p>
        </div>
      </div>

      <!-- Basic info -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 border-b border-border">
          <h2 class="text-sm font-semibold">Basic Information</h2>
          <p class="text-xs text-muted-foreground mt-0.5">Your public name, username, and short bio.</p>
        </div>
        <div class="p-6 space-y-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-foreground/80">Display Name</label>
              <Input v-model="form.name" placeholder="Your name" />
            </div>
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-foreground/80">Title</label>
              <Input v-model="form.title" placeholder="e.g. Software Engineer" />
            </div>
          </div>
          <div class="space-y-1.5">
            <div class="flex items-center justify-between">
              <label class="text-xs font-medium text-foreground/80">Bio</label>
              <span class="text-[11px] text-muted-foreground/50" :class="bioLeft < 20 ? 'text-amber-500' : ''">
                {{ bioLeft }} / {{ bioMax }}
              </span>
            </div>
            <textarea
              v-model="form.bio"
              :maxlength="bioMax"
              rows="3"
              placeholder="Write a short bio..."
              class="w-full rounded-md border border-input bg-background px-3 py-2 text-sm placeholder:text-muted-foreground shadow-xs resize-none focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:border-ring transition-[color,box-shadow]"
            />
          </div>
        </div>
      </div>

      <!-- Contact -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 border-b border-border">
          <h2 class="text-sm font-semibold">Contact & Location</h2>
          <p class="text-xs text-muted-foreground mt-0.5">Where people can find you online.</p>
        </div>
        <div class="p-6 space-y-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-foreground/80 flex items-center gap-1.5">
                <MapPin class="size-3 text-muted-foreground" /> Company
              </label>
              <Input v-model="form.company" placeholder="Company name" />
            </div>
          </div>
        </div>
      </div>

      <!-- Save -->
      <div class="flex justify-end gap-2">
        <Button variant="outline" @click="fetchProfile">Cancel</Button>
        <Button class="gap-1.5" :disabled="saving" @click="save">
          <Loader2 v-if="saving" class="size-3.5 animate-spin" />
          <Save v-else class="size-3.5" />
          Save changes
        </Button>
      </div>

    </div>
  </div>

  <AvatarPicker v-model:open="pickerOpen" :current="form.avatarUrl" @select="onAvatarSelect" />
</template>
