<script setup>
import TopNav from '../components/TopNav.vue'
import SideNav from '../components/SideNav.vue'
import BreadcrumbBar from '../components/BreadcrumbBar.vue'
import Footer from "../components/Footer.vue";
</script>

<template>
  <div class="min-h-screen flex flex-col bg-base-200/50 transition-colors duration-300">

    <header class="sticky top-0 z-50 w-full bg-base-100/80 backdrop-blur-md border-b border-base-content/5">
      <TopNav />
    </header>

    <div class="flex-grow w-full max-w-[1536px] mx-auto px-4 lg:px-8">
      <div class="flex flex-col lg:flex-row gap-8 py-8 relative">

        <aside class="w-full lg:w-72 shrink-0 z-40">
          <SideNav />
        </aside>

        <main class="flex-1 min-w-0 z-10">
          <nav class="mb-6"><BreadcrumbBar /></nav>
          <router-view v-slot="{ Component }">
            <transition name="fade-slide" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </main>
      </div>
    </div>

    <footer class="w-full border-t border-base-content/5 bg-base-100/50 py-10 mt-auto z-0">
      <div class="max-w-[1536px] mx-auto text-center">
        <Footer />
      </div>
    </footer>
  </div>
</template>

<!--<style scoped>-->
<!--/* 修复：移除所有会引起报错的 @apply 指令，改用标准 CSS 转场 */-->
<!--.page-fade-enter-active,-->
<!--.page-fade-leave-active {-->
<!--  transition: opacity 0.25s ease, transform 0.25s ease;-->
<!--}-->

<!--.page-fade-enter-from {-->
<!--  opacity: 0;-->
<!--  transform: translateY(8px);-->
<!--}-->

<!--.page-fade-leave-to {-->
<!--  opacity: 0;-->
<!--  transform: translateY(-4px);-->
<!--}-->

<!--/* 如果需要全局调整子组件统计卡片的投影，建议在全局 app.css 中处理，或如下直接写原生 CSS */-->
<!--:deep(.stats), :deep(.card) {-->
<!--  box-shadow: 0 1px 2px 0 rgb(0 0 0 / 0.05); /* 这等同于 shadow-sm */-->
<!--  border-color: #e2e8f0; /* 等同于 border-slate-200 */-->
<!--}-->
<!--</style>-->