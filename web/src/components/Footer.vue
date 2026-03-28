<template>
  <footer class="w-full py-12 border-t border-slate-200/60 dark:border-base-300/30">
    <div class="mx-auto max-w-[1600px] px-4 flex flex-col items-center gap-8">

      <nav class="flex flex-wrap justify-center gap-x-8 gap-y-2">
        <a
            v-for="link in footerLinks"
            :key="link.name"
            :href="link.href"
            class="text-sm font-medium text-base-content/60 hover:text-primary transition-colors duration-300"
        >
          {{ link.name }}
        </a>
      </nav>

      <div class="flex flex-col items-center gap-3">
        <div class="flex items-center gap-2 opacity-80">
          <div class="w-6 h-6 bg-slate-200 dark:bg-base-300 rounded-lg flex items-center justify-center">
            <svg width="14" height="14" viewBox="0 0 32 32" fill="none" class="text-base-content/60">
              <path d="M18 6L10 18H16L14 26L22 14H16L18 6Z" fill="currentColor" />
            </svg>
          </div>
          <span class="text-sm font-black tracking-widest uppercase text-base-content/40">Wireflow</span>
        </div>
        <p class="text-xs text-base-content/40 font-medium tracking-wide">
          下一代自动化网络编排控制台
        </p>
      </div>

      <div class="flex flex-col items-center gap-4">
        <div class="flex items-center gap-4 px-4 py-2 bg-slate-100/50 dark:bg-base-200/50 rounded-full border border-slate-200/40">
          <div class="flex items-center gap-1.5">
            <span class="w-1.5 h-1.5 rounded-full bg-success"></span>
            <span class="text-[10px] font-bold text-base-content/60 uppercase tracking-tight">System Operational</span>
          </div>
          <div class="w-px h-3 bg-slate-300/50"></div>
          <div class="text-[10px] font-mono text-base-content/40 uppercase">Latency: 14ms</div>
        </div>

        <div class="text-[10px] text-base-content/40/60 font-medium text-center space-y-1">
          <p>© 2026 Wireflow Project. All rights reserved.</p>
          <p class="italic">提示：本项目为自托管服务，数据存储于您的本地实例。</p>
        </div>
      </div>

    </div>
  </footer>
</template>

<script setup>
const footerLinks = [
  { name: '使用文档', href: '#' },
  { name: 'GitHub 仓库', href: '#' },
  { name: '隐私政策', href: '#' },
  { name: '系统状态', href: '#' },
  { name: '更新日志', href: '#' },
]
</script>

<style scoped>
/* 悬浮链接的微小动画 */
footer a {
  position: relative;
}
footer a::after {
  content: '';
  position: absolute;
  width: 0;
  height: 1px;
  bottom: -2px;
  left: 0;
  background-color: currentColor;
  transition: width 0.3s ease;
  opacity: 0.3;
}
footer a:hover::after {
  width: 100%;
}
</style>