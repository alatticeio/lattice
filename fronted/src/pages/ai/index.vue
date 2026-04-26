<script setup lang="ts">
import ChatSidebar from '@/components/ai/ChatSidebar.vue'
import ChatWindow from '@/components/ai/ChatWindow.vue'

definePage({
  meta: {
    titleKey: 'common.nav.ai',
    descKey: 'common.nav.ai',
  },
})
</script>

<template>
  <div class="flex h-[calc(100vh-var(--header-height,56px))] overflow-hidden">
    <ChatSidebar />
    <div class="flex-1 min-w-0">
      <ChatWindow />
    </div>
  </div>
</template>
