<template>
  <teleport to="body">
    <transition name="fade">
      <div
          v-if="modelValue"
          class="fixed inset-0 bg-black/40 backdrop-blur-sm z-[60]"
          @click="$emit('update:modelValue', false)"
      ></div>
    </transition>

    <aside
        :class="[
        'fixed top-0 right-0 h-full bg-base-100 shadow-2xl z-[70] transition-all duration-500 ease-[cubic-bezier(0.16,1,0.3,1)] border-l border-base-300 flex flex-col',
        modelValue ? 'translate-x-0' : 'translate-x-full'
      ]"
        :style="{ width: responsiveWidth }"
    >
      <div class="h-20 flex items-center justify-between px-8 border-b border-base-300 bg-base-200/50 shrink-0">
        <div class="flex items-center gap-4">
          <slot name="icon"></slot>
          <div class="flex flex-col">
            <h2 class="text-lg font-black tracking-tight leading-none">{{ title }}</h2>
            <p class="text-[10px] uppercase font-bold opacity-40 tracking-[0.2em] mt-1">{{ subtitle }}</p>
          </div>
        </div>
        <button class="btn btn-sm btn-circle btn-ghost hover:rotate-90 transition-transform" @click="$emit('update:modelValue', false)">✕</button>
      </div>

      <div class="flex-1 overflow-y-auto custom-scrollbar">
        <div class="p-8">
          <slot></slot>
        </div>
      </div>

      <div v-if="$slots.footer" class="p-8 border-t border-base-300 bg-base-100 shrink-0 shadow-[0_-10px_30px_-15px_rgba(0,0,0,0.1)]">
        <slot name="footer"></slot>
      </div>
    </aside>
  </teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'

const props = defineProps({
  modelValue: Boolean,
  title: String,
  subtitle: { type: String, default: 'Network Intelligence' },
  // 传入级别：sm, md, lg, xl
  level: {
    type: String,
    default: 'md'
  }
})

defineEmits(['update:modelValue'])

// 响应式屏幕宽度监测
const windowWidth = ref(window.innerWidth)
const updateWidth = () => windowWidth.value = window.innerWidth

onMounted(() => window.addEventListener('resize', updateWidth))
onUnmounted(() => window.removeEventListener('resize', updateWidth))

// 核心自适应逻辑
const responsiveWidth = computed(() => {
  // 1. 如果是移动端（屏宽 < 768px），强制全屏
  if (windowWidth.value < 768) return '100vw'

  // 2. 根据 Level 定义的基础宽度
  const baseWidths = {
    'sm': 400,
    'md': 560,
    'lg': 820,
    'xl': 1100
  }

  const targetWidth = baseWidths[props.level] || 560

  // 3. 安全边界：确保侧边栏不会盖住整个屏幕（至少给左侧留 15% 的空白看背景）
  const maxWidth = windowWidth.value * 0.85
  const finalWidth = Math.min(targetWidth, maxWidth)

  return `${finalWidth}px`
})
</script>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.4s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

.custom-scrollbar::-webkit-scrollbar { width: 5px; }
.custom-scrollbar::-webkit-scrollbar-track { background: transparent; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: hsl(var(--bc) / 0.15); border-radius: 10px; }
.custom-scrollbar::-webkit-scrollbar-thumb:hover { background: hsl(var(--p)); }

/* 为内容切换添加平滑过渡 */
.v-enter-active, .v-leave-active { transition: all 0.3s ease; }
.v-enter-from, .v-leave-to { opacity: 0; transform: translateY(10px); }
</style>