<script setup lang="ts">
import { useRoute } from 'vue-router'
import { computed } from 'vue'
import { User, Settings, CreditCard, Bell } from 'lucide-vue-next'

const route = useRoute()

const tabs = [
  { label: 'Profile',       path: '/user/profile',       icon: User },
  { label: 'Account',       path: '/user/account',       icon: Settings },
  { label: 'Billing',       path: '/user/billing',       icon: CreditCard },
  { label: 'Notifications', path: '/user/notifications', icon: Bell },
]

const active = computed(() => route.path)
</script>

<template>
  <div class="border-b border-border bg-card/60 backdrop-blur-sm sticky top-0 z-20">
    <nav class="flex gap-0 px-6 overflow-x-auto scrollbar-none">
      <RouterLink
        v-for="tab in tabs"
        :key="tab.path"
        :to="tab.path"
        class="relative flex items-center gap-2 px-4 py-3.5 text-sm font-medium whitespace-nowrap transition-colors"
        :class="active === tab.path
          ? 'text-foreground'
          : 'text-muted-foreground hover:text-foreground'"
      >
        <component :is="tab.icon" class="size-3.5" />
        {{ tab.label }}
        <span
          v-if="active === tab.path"
          class="absolute bottom-0 left-0 right-0 h-0.5 bg-primary rounded-full"
        />
      </RouterLink>
    </nav>
  </div>
</template>
