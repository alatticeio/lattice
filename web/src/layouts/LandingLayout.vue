<script setup>
import TopNav from '../components/TopNav.vue'
import Footer from '../components/Footer.vue'
</script>

<template>
  <div class="min-h-screen bg-base-200 flex flex-col">
    <TopNav />

    <main class="flex-1">
      <div class="mx-auto max-w-[1400px] w-full px-4 py-6">
        <router-view />
      </div>
    </main>

    <Footer />
  </div>
</template>