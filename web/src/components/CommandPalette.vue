<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Icon from './icons/Icon.vue'

const props = defineProps({
  open: { type: Boolean, default: false },
})
const emit = defineEmits(['update:open'])

const router = useRouter()
const q = ref('')
const inputEl = ref(null)
const cursor = ref(0)

const commands = [
  { label: '仪表盘', to: '/', icon: 'dashboard', keywords: 'dashboard home 概览' },
  { label: '节点管理', to: '/nodes', icon: 'nodes', keywords: 'nodes 节点' },
  { label: 'Token 管理', to: '/tokens', icon: 'token', keywords: 'token 密钥' },
  { label: '策略管理', to: '/policies', icon: 'policy', keywords: 'policy 策略 acl' },
  { label: '监控', to: '/monitor', icon: 'monitor', keywords: 'monitor metrics 监控 告警' },
  { label: 'DNS 配置', to: '/dns', icon: 'dns', keywords: 'dns 域名 解析' },
  { label: '个人信息', to: '/profile', icon: 'user', keywords: 'profile account 个人 账户' },
]

const filtered = computed(() => {
  const s = q.value.trim().toLowerCase()
  if (!s) return commands
  return commands.filter(c => (c.label + ' ' + c.keywords).toLowerCase().includes(s))
})

watch(
    () => props.open,
    async (v) => {
      if (v) {
        q.value = ''
        cursor.value = 0
        await nextTick()
        inputEl.value?.focus()
      }
    }
)

function close() {
  emit('update:open', false)
}

async function go(item) {
  close()
  await router.push(item.to)
}

function onKeydown(e) {
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
    return
  }
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    cursor.value = Math.min(cursor.value + 1, filtered.value.length - 1)
    return
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    cursor.value = Math.max(cursor.value - 1, 0)
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    const item = filtered.value[cursor.value]
    if (item) go(item)
  }
}
</script>

<template>
  <dialog class="modal" :open="props.open" @close="close">
    <div class="modal-box p-0 overflow-hidden" @keydown="onKeydown">
      <div class="p-4 border-b border-base-300 flex items-center gap-2">
        <Icon name="search" class="w-4 h-4 opacity-70" />
        <input
            ref="inputEl"
            v-model="q"
            class="input input-ghost w-full"
            placeholder="搜索：节点 / Token / 策略 / 监控 / DNS..."
            autocomplete="off"
        />
        <kbd class="kbd kbd-sm">ESC</kbd>
      </div>

      <div class="max-h-[360px] overflow-auto">
        <div v-if="filtered.length === 0" class="p-6 text-sm opacity-70">
          没有匹配结果
        </div>

        <ul v-else class="menu">
          <li v-for="(c, idx) in filtered" :key="c.to">
            <a
                href="#"
                class="flex items-center gap-2"
                :class="idx === cursor ? 'active' : ''"
                @click.prevent="go(c)"
                @mousemove="cursor = idx"
            >
              <Icon :name="c.icon" class="w-4 h-4" />
              <span class="flex-1">{{ c.label }}</span>
              <span class="text-xs opacity-60">{{ c.to }}</span>
            </a>
          </li>
        </ul>
      </div>
    </div>

    <form method="dialog" class="modal-backdrop" @submit.prevent="close">
      <button aria-label="close">close</button>
    </form>
  </dialog>
</template>