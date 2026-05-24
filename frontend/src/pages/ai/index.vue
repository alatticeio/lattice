<script setup lang="ts">
import ChatSidebar from '@/components/ai/ChatSidebar.vue'
import ChatWindow from '@/components/ai/ChatWindow.vue'

definePage({
  meta: { fullHeight: true },
})
</script>

<template>
  <div class="flex h-full">
    <!-- 侧边栏 -->
    <div class="shrink-0 h-full">
      <ChatSidebar />
    </div>
    <!-- 主聊天区 -->
    <div class="flex-1 min-w-0 h-full">
      <ChatWindow />
    </div>
  </div>
</template>
