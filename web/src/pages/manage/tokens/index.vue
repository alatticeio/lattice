<script setup lang="ts">
import { ref, computed } from 'vue'
import { Plus, Copy, Check, Terminal, Trash2, Key } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription,
} from '@/components/ui/sheet'

definePage({
  meta: { title: 'Token 管理', description: '管理接入网络的访问令牌。' },
})

interface TokenItem {
  id: string
  name: string
  initials: string
  color: string
  token: string
  createdAt: string
  expiresAt: string | null
  usages: number
}

const tokens = ref<TokenItem[]>([
  { id: '1', name: 'Production Gateway', initials: 'PG', color: 'bg-violet-500', token: 'tkn_prod_aB3xYz9mN2kL', createdAt: '2024-01-15', expiresAt: '2025-01-15', usages: 42 },
  { id: '2', name: 'Dev Environment', initials: 'DE', color: 'bg-sky-500', token: 'tkn_dev_cD5wVu8pQ1jR', createdAt: '2024-03-20', expiresAt: null, usages: 7 },
  { id: '3', name: 'CI Runner Alpha', initials: 'CA', color: 'bg-emerald-500', token: 'tkn_ci_eF7tSs6oP0iT', createdAt: '2024-05-10', expiresAt: '2024-12-31', usages: 198 },
  { id: '4', name: 'Mobile Client', initials: 'MC', color: 'bg-amber-500', token: 'tkn_mob_gH9rQr4nM9hU', createdAt: '2024-06-01', expiresAt: null, usages: 5 },
  { id: '5', name: 'Relay Node Beta', initials: 'RB', color: 'bg-rose-500', token: 'tkn_rel_iJ1pPq2lK8gV', createdAt: '2024-07-22', expiresAt: '2025-07-22', usages: 3 },
])

const createDrawerOpen = ref(false)
const joinModalOpen = ref(false)
const selectedToken = ref<TokenItem | null>(null)
const copied = ref(false)

const newToken = ref({ name: '', expiresAt: '' })

const installCommand = computed(() => selectedToken.value
  ? `wireflow join --token ${selectedToken.value.token}`
  : '')

function openJoinModal(token: TokenItem) {
  selectedToken.value = token
  joinModalOpen.value = true
}

async function copyCommand() {
  await navigator.clipboard.writeText(installCommand.value)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

function createToken() {
  tokens.value.push({
    id: String(Date.now()),
    name: newToken.value.name || 'New Token',
    initials: (newToken.value.name || 'NT').slice(0, 2).toUpperCase(),
    color: ['bg-violet-500', 'bg-sky-500', 'bg-emerald-500', 'bg-amber-500', 'bg-rose-500'][Math.floor(Math.random() * 5)],
    token: `tkn_new_${Math.random().toString(36).slice(2, 14)}`,
    createdAt: new Date().toISOString().slice(0, 10),
    expiresAt: newToken.value.expiresAt || null,
    usages: 0,
  })
  newToken.value = { name: '', expiresAt: '' }
  createDrawerOpen.value = false
}

function deleteToken(id: string) {
  tokens.value = tokens.value.filter(t => t.id !== id)
}

function isExpired(expiresAt: string | null): boolean {
  if (!expiresAt) return false
  return new Date(expiresAt) < new Date()
}
</script>

<template>
  <div class="p-6 space-y-5">
    <!-- Header toolbar -->
    <div class="flex items-center gap-3">
      <div class="flex-1">
        <p class="text-sm text-muted-foreground">共 {{ tokens.length }} 个 Token</p>
      </div>
      <Button size="sm" class="gap-1.5" @click="createDrawerOpen = true">
        <Plus class="size-3.5" /> 创建 Token
      </Button>
    </div>

    <!-- Token list -->
    <div class="bg-card border border-border rounded-xl overflow-hidden">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-border bg-muted/30">
            <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">名称</th>
            <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground hidden md:table-cell">Token</th>
            <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground hidden lg:table-cell">创建时间</th>
            <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">到期时间</th>
            <th class="text-left px-4 py-3 text-xs font-medium uppercase tracking-wider text-muted-foreground hidden sm:table-cell">使用次数</th>
            <th class="px-4 py-3" />
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="token in tokens"
            :key="token.id"
            class="border-b border-border last:border-0 hover:bg-muted/20 transition-colors"
          >
            <td class="px-4 py-3">
              <div class="flex items-center gap-3">
                <div class="size-9 rounded-lg flex items-center justify-center text-white text-sm font-semibold shrink-0" :class="token.color">
                  {{ token.initials }}
                </div>
                <div>
                  <p class="font-medium">{{ token.name }}</p>
                  <p class="text-xs text-muted-foreground flex items-center gap-1">
                    <Key class="size-3" /> Token
                  </p>
                </div>
              </div>
            </td>
            <td class="px-4 py-3 hidden md:table-cell">
              <span class="font-mono text-xs bg-muted px-2 py-1 rounded">{{ token.token.slice(0, 20) }}...</span>
            </td>
            <td class="px-4 py-3 text-muted-foreground hidden lg:table-cell">{{ token.createdAt }}</td>
            <td class="px-4 py-3">
              <span v-if="!token.expiresAt" class="text-xs text-muted-foreground">永久有效</span>
              <span v-else-if="isExpired(token.expiresAt)"
                class="text-xs rounded-full px-2 py-0.5 bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400 font-medium">
                已过期 · {{ token.expiresAt }}
              </span>
              <span v-else class="text-xs text-muted-foreground">{{ token.expiresAt }}</span>
            </td>
            <td class="px-4 py-3 hidden sm:table-cell text-muted-foreground">{{ token.usages }}</td>
            <td class="px-4 py-3">
              <div class="flex items-center justify-end gap-2">
                <Button variant="outline" size="sm" class="gap-1.5" @click="openJoinModal(token)">
                  <Terminal class="size-3.5" /> 一键加入
                </Button>
                <Button variant="ghost" size="sm" class="size-8 p-0 text-muted-foreground hover:text-destructive" @click="deleteToken(token.id)">
                  <Trash2 class="size-4" />
                </Button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create Token Drawer -->
    <Sheet v-model:open="createDrawerOpen">
      <SheetContent class="w-[380px]">
        <SheetHeader>
          <SheetTitle>创建 Token</SheetTitle>
          <SheetDescription>新建一个节点接入令牌</SheetDescription>
        </SheetHeader>
        <div class="mt-6 space-y-4">
          <div class="space-y-1.5">
            <label class="text-sm font-medium">Token 名称</label>
            <Input v-model="newToken.name" placeholder="例如：Production Gateway" />
          </div>
          <div class="space-y-1.5">
            <label class="text-sm font-medium">到期时间 <span class="text-muted-foreground font-normal">(留空表示永久有效)</span></label>
            <Input v-model="newToken.expiresAt" type="date" />
          </div>
          <div class="flex justify-end gap-2 pt-2">
            <Button variant="outline" @click="createDrawerOpen = false">取消</Button>
            <Button @click="createToken">创建</Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>

    <!-- Join Command Sheet -->
    <Sheet v-model:open="joinModalOpen">
      <SheetContent class="w-[420px]">
        <SheetHeader>
          <SheetTitle class="flex items-center gap-2">
            <Terminal class="size-5" /> 节点接入命令
          </SheetTitle>
          <SheetDescription>在目标机器上运行以下命令将节点加入网络。</SheetDescription>
        </SheetHeader>
        <div class="mt-6 space-y-4">
          <div v-if="selectedToken" class="flex items-center gap-3 p-3 bg-muted/50 rounded-lg">
            <div class="size-9 rounded-lg flex items-center justify-center text-white text-sm font-semibold shrink-0" :class="selectedToken.color">
              {{ selectedToken.initials }}
            </div>
            <div>
              <p class="text-sm font-medium">{{ selectedToken.name }}</p>
              <p class="text-xs text-muted-foreground">{{ selectedToken.token }}</p>
            </div>
          </div>

          <div class="relative">
            <div class="bg-zinc-950 dark:bg-zinc-900 rounded-lg p-4 pr-12 font-mono text-sm text-emerald-400 border border-zinc-800">
              <span class="text-zinc-500 select-none">$ </span>{{ installCommand }}
            </div>
            <button
              @click="copyCommand"
              class="absolute right-2 top-2 p-2 rounded-md text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800 transition-colors"
            >
              <Check v-if="copied" class="size-4 text-emerald-400" />
              <Copy v-else class="size-4" />
            </button>
          </div>

          <p class="text-xs text-muted-foreground">
            确保目标机器已安装 wireflow agent。命令执行成功后节点将出现在节点列表中。
          </p>
        </div>
      </SheetContent>
    </Sheet>
  </div>
</template>
