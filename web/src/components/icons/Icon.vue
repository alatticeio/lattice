<script setup>
import { computed } from 'vue'

const props = defineProps({
  loading: {
    type: Boolean,
    default: false
  },
  name: { type: String, required: true },
  class: { type: String, default: 'w-4 h-4' },
})

const paths = computed(() => {
  // 轻量内置 SVG 图标（stroke 风格），不依赖第三方库
  switch (props.name) {
    case 'dashboard':
      return [
        'M3 13h8V3H3v10zm10 8h8V11h-8v10zM3 21h8v-6H3v6zm10-18v6h8V3h-8z',
      ]
    case 'nodes':
      return [
        'M12 3l9 5-9 5-9-5 9-5z',
        'M21 8v8l-9 5-9-5V8',
        'M12 13v8',
      ]
    case 'token':
      return [
        'M12 3a5 5 0 100 10 5 5 0 000-10z',
        'M14 12.5V21l-2-1-2 1v-8.5',
      ]
    case 'policy':
      return [
        'M7 4h10v16H7V4z',
        'M9 8h6',
        'M9 12h6',
        'M9 16h4',
      ]
    case 'monitor':
      return [
        'M4 19h16',
        'M6 16l3-4 3 3 4-6 2 3',
      ]
    case 'dns':
      return [
        'M12 2a10 10 0 100 20 10 10 0 000-20z',
        'M2 12h20',
        'M12 2c3 3 5 6 5 10s-2 7-5 10c-3-3-5-6-5-10s2-7 5-10z',
      ]
    case 'user':
      return [
        'M12 12a4 4 0 100-8 4 4 0 000 8z',
        'M4 20a8 8 0 0116 0',
      ]
    case 'search':
      return [
        'M11 19a8 8 0 110-16 8 8 0 010 16z',
        'M21 21l-4.35-4.35',
      ]
    case 'refresh':
      return ['M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15']
    case 'topology':
      return [
        'M6 7a2 2 0 104 0 2 2 0 10-4 0z',
        'M14 7a2 2 0 104 0 2 2 0 10-4 0z',
        'M10 17a2 2 0 104 0 2 2 0 10-4 0z',
        'M8 8l6 0',
        'M9 9l3 6',
        'M15 9l-3 6',
      ]
    case 'zoom-in':
      return ['M11 19a8 8 0 110-16 8 8 0 010 16z', 'M21 21l-4.35-4.35', 'M11 8v6', 'M8 11h6']
    case 'zoom-out':
      return ['M11 19a8 8 0 110-16 8 8 0 010 16z', 'M21 21l-4.35-4.35', 'M8 11h6']
    case 'center':
      return ['M12 2v4', 'M12 18v4', 'M2 12h4', 'M18 12h4', 'M8 12a4 4 0 118 0a4 4 0 01-8 0z']
    case 'workspace': // 工作空间：类似图层或三个方块叠加的感官
      return [
        'M12 2L2 7l10 5 10-5-10-5z',
        'M2 17l10 5 10-5',
        'M2 12l10 5 10-5'
      ]

    case 'arrow-right':
      return [
        'M5 12h14',
        'M12 5l7 7-7 7'
      ]

    case 'plus': // 顺便帮你补上你 Header 里用到的 plus
      return [
        'M12 5v14',
        'M5 12h14'
      ]

      // 删掉之前所有的 case 'trash'，统一换成这一个
    case 'trash':
      return [
        'M3 6h18',                             // 顶栏
        'M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6', // 桶身
        'M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2',    // 盖子
        'M10 11v6',                            // 内部线条1
        'M14 11v6'                             // 内部线条2
      ]

    case 'info': // 抽屉里安全提示用到的 info
      return [
        'M12 8v4',
        'M12 16h.01',
        'M22 12c0 5.523-4.477 10-10 10S2 17.523 2 12 6.477 2 12 2s10 4.477 10 10z'
      ]

    case 'peering': // 对等连接：两个互联的节点或交换箭头
      return [
        'M2 8L7 3l5 5',
        'M17 16l5 5-5-5', // 这里的逻辑可以写成双向箭头
        'M22 16l-5 5-5-5',
        'M7 3v13',
        'M17 21V8',
        'M2 8h10',
        'M12 16h10'
      ]

    case 'exit':
      return ['M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1']

      // 为了美观，我再顺手帮你补一个通用的 'edit' 和 'trash' 以备不时之需
    case 'edit':
      return ['M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7', 'M18.5 2.5a2.121 2.121 0 113 3L12 15l-4 1 1-4 9.5-9.5z']

    // case 'trash':
    //   return ['M3 6h18', 'M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2']

    case 'chevron-right': // 你 Sidebar 底部用到了这个
      return ['M9 18l6-6-6-6']

    default:
      return ['M4 4h16v16H4V4z']
  }
})
</script>

<template>
  <svg
      :class="[
      // 1. 始终应用传入的尺寸/样式类
      props.class,
      // 2. 只有当 name 是 refresh 且 loading 为 true 时才旋转
      // 或者你希望所有图标 loading 时都转，可以去掉 name 判断
      (name === 'refresh' && loading) ? 'animate-spin text-primary' : ''
    　]"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.8"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
  >
    <path v-for="(d, i) in paths" :key="i" :d="d" />
  </svg>
</template>