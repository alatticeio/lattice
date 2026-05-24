<script setup lang="ts">
import { ref, computed } from 'vue'
import { Camera, Trash2, Globe, MapPin, Building2, Github, Twitter, Save } from 'lucide-vue-next'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import UserSettingsNav from '@/components/UserSettingsNav.vue'

definePage({
  meta: { title: 'Profile', description: 'Manage your public profile information.' },
})

const form = ref({
  displayName: 'Admin User',
  username: 'adminuser',
  bio: 'Full-stack developer and product designer. Building things for the web.',
  website: 'https://example.com',
  location: 'San Francisco, CA',
  company: 'Acme Inc.',
  twitter: 'adminuser',
  github: 'adminuser',
})

const bioMax = 200
const bioLeft = computed(() => bioMax - form.value.bio.length)

function save() { /* submit */ }
</script>

<template>
  <div class="flex flex-col">
    <UserSettingsNav />

    <div class="mx-auto w-full max-w-3xl space-y-5 p-6">

      <!-- Avatar -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="h-20 bg-gradient-to-br from-primary/20 via-primary/5 to-transparent" />
        <div class="px-6 pb-5">
          <div class="flex items-end justify-between -mt-8 mb-3">
            <div class="relative">
              <Avatar class="size-16 ring-4 ring-card">
                <AvatarFallback class="bg-primary text-primary-foreground text-xl font-bold">AU</AvatarFallback>
              </Avatar>
              <button class="absolute bottom-0 right-0 size-6 rounded-full bg-card border border-border shadow-sm flex items-center justify-center hover:bg-muted transition-colors">
                <Camera class="size-3" />
              </button>
            </div>
            <div class="flex gap-2">
              <Button variant="outline" size="sm">Upload photo</Button>
              <Button variant="ghost" size="sm" class="text-muted-foreground hover:text-destructive gap-1.5">
                <Trash2 class="size-3.5" /> Remove
              </Button>
            </div>
          </div>
          <p class="text-xs text-muted-foreground/60">JPG, PNG or GIF · Max 5 MB · Recommended 400×400px</p>
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
              <Input v-model="form.displayName" placeholder="Your name" />
            </div>
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-foreground/80">Username</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground select-none">@</span>
                <Input v-model="form.username" class="pl-7" placeholder="username" />
              </div>
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
          <div class="space-y-1.5">
            <label class="text-xs font-medium text-foreground/80 flex items-center gap-1.5">
              <Globe class="size-3 text-muted-foreground" /> Website
            </label>
            <Input v-model="form.website" type="url" placeholder="https://yoursite.com" />
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-foreground/80 flex items-center gap-1.5">
                <MapPin class="size-3 text-muted-foreground" /> Location
              </label>
              <Input v-model="form.location" placeholder="City, Country" />
            </div>
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-foreground/80 flex items-center gap-1.5">
                <Building2 class="size-3 text-muted-foreground" /> Company
              </label>
              <Input v-model="form.company" placeholder="Company name" />
            </div>
          </div>
          <div class="h-px bg-border" />
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-foreground/80 flex items-center gap-1.5">
                <Twitter class="size-3 text-muted-foreground" /> Twitter / X
              </label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground select-none">@</span>
                <Input v-model="form.twitter" class="pl-7" placeholder="handle" />
              </div>
            </div>
            <div class="space-y-1.5">
              <label class="text-xs font-medium text-foreground/80 flex items-center gap-1.5">
                <Github class="size-3 text-muted-foreground" /> GitHub
              </label>
              <Input v-model="form.github" placeholder="username" />
            </div>
          </div>
        </div>
      </div>

      <!-- Save -->
      <div class="flex justify-end gap-2">
        <Button variant="outline">Cancel</Button>
        <Button class="gap-1.5" @click="save">
          <Save class="size-3.5" /> Save changes
        </Button>
      </div>

    </div>
  </div>
</template>
