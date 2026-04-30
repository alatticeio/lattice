<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { GalleryVerticalEnd, Loader2, XCircle, ShieldAlert } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'vue-sonner'
import { previewInvitation, acceptInvitation, registerAndAccept, type InvitePreviewVo } from '@/api/invitation'
import { useUserStore } from '@/stores/user'
import { setToken } from '@/utils/auth'

definePage({
  meta: { layout: 'blank' },
})

const route  = useRoute()
const router = useRouter()
const userStore = useUserStore()

const token   = route.params.token as string
const preview = ref<InvitePreviewVo | null>(null)
const loadErr = ref('')
const loading = ref(true)

onMounted(async () => {
  try {
    const res = await previewInvitation(token)
    preview.value = res.data
  } catch {
    loadErr.value = '邀请链接无效或已过期'
  } finally {
    loading.value = false
  }
})

// ── Determine UI state ────────────────────────────────────────────
// 'register' | 'accept' | 'mismatch' | 'done' | 'invalid'
const state = computed(() => {
  if (loadErr.value) return 'invalid'
  if (!preview.value) return 'loading'
  if (preview.value.status !== 'pending') return 'invalid'

  if (!userStore.isLoggedIn) return 'register'

  const myEmail = userStore.userInfo?.email ?? ''
  if (myEmail.toLowerCase() !== preview.value.email.toLowerCase()) return 'mismatch'

  return 'accept'
})

// ── Register form ─────────────────────────────────────────────────
const form = ref({ username: '', password: '' })
const submitting = ref(false)

async function submitRegister() {
  submitting.value = true
  try {
    const res = await registerAndAccept(token, form.value)
    setToken(res.data.token)
    await userStore.fetchUserInfo()
    toast.success('注册成功，已加入工作空间！')
    router.push('/')
  } catch (e: any) {
    toast.error(e?.response?.data?.message ?? '注册失败，请重试')
  } finally {
    submitting.value = false
  }
}

// ── Accept (already logged in) ────────────────────────────────────
async function submitAccept() {
  submitting.value = true
  try {
    await acceptInvitation(token)
    toast.success('已加入工作空间！')
    router.push('/')
  } catch (e: any) {
    toast.error(e?.response?.data?.message ?? '接受邀请失败')
  } finally {
    submitting.value = false
  }
}

const roleLabel: Record<string, string> = {
  admin: '管理员', editor: '编辑者', member: '成员', viewer: '访客',
}
</script>

<template>
  <div class="bg-muted min-h-svh flex flex-col items-center justify-center gap-6 p-6 md:p-10">
    <div class="w-full max-w-sm flex flex-col gap-6">

      <!-- Logo -->
      <div class="flex items-center gap-2 self-center font-medium">
        <div class="bg-primary text-primary-foreground flex size-6 items-center justify-center rounded-md">
          <GalleryVerticalEnd class="size-4" />
        </div>
        Lattice
      </div>

      <!-- Card -->
      <div class="bg-card border border-border rounded-xl shadow-sm p-6 flex flex-col gap-5">

        <!-- Loading -->
        <div v-if="loading" class="flex items-center justify-center py-8">
          <Loader2 class="size-6 animate-spin text-muted-foreground" />
        </div>

        <!-- Invalid / expired -->
        <template v-else-if="state === 'invalid'">
          <div class="flex flex-col items-center gap-3 py-4 text-center">
            <XCircle class="size-10 text-red-500" />
            <p class="font-semibold">邀请无效</p>
            <p class="text-sm text-muted-foreground">{{ loadErr || '该邀请已过期、已被接受或已撤销。' }}</p>
            <Button variant="outline" size="sm" class="mt-2" @click="router.push('/auth/login')">返回登录</Button>
          </div>
        </template>

        <!-- Email mismatch -->
        <template v-else-if="state === 'mismatch'">
          <div class="flex flex-col items-center gap-3 py-4 text-center">
            <ShieldAlert class="size-10 text-amber-500" />
            <p class="font-semibold">账号不匹配</p>
            <p class="text-sm text-muted-foreground">
              此邀请发送给 <strong>{{ preview!.email }}</strong>，<br>
              但您当前登录的是 <strong>{{ userStore.userInfo?.email }}</strong>。
            </p>
            <p class="text-sm text-muted-foreground">请退出后使用正确的账号登录，或联系管理员重新发送邀请。</p>
            <Button variant="outline" size="sm" class="mt-2" @click="userStore.logout(false); router.push(`/auth/login?redirect=/invite/${token}`)">
              切换账号
            </Button>
          </div>
        </template>

        <!-- Accept (logged in, email matches) -->
        <template v-else-if="state === 'accept'">
          <div class="flex flex-col gap-4">
            <div class="text-center">
              <p class="text-lg font-semibold">接受邀请</p>
              <p class="text-sm text-muted-foreground mt-1">
                您受邀加入工作空间
              </p>
            </div>

            <!-- Invitation summary -->
            <div class="rounded-lg bg-muted/60 border border-border p-4 text-sm space-y-2">
              <div class="flex justify-between">
                <span class="text-muted-foreground">工作空间</span>
                <span class="font-medium">{{ preview!.workspaceName }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">角色</span>
                <span class="font-medium">{{ roleLabel[preview!.role] ?? preview!.role }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">邀请人</span>
                <span class="font-medium">{{ preview!.inviterName || preview!.inviterEmail }}</span>
              </div>
            </div>

            <Button :disabled="submitting" @click="submitAccept">
              <Loader2 v-if="submitting" class="size-4 mr-2 animate-spin" />
              确认加入
            </Button>
          </div>
        </template>

        <!-- Register (not logged in) -->
        <template v-else-if="state === 'register'">
          <div class="flex flex-col gap-4">
            <div class="text-center">
              <p class="text-lg font-semibold">接受邀请</p>
              <p class="text-sm text-muted-foreground mt-1">创建账号后加入工作空间</p>
            </div>

            <!-- Invitation summary -->
            <div class="rounded-lg bg-muted/60 border border-border p-4 text-sm space-y-2">
              <div class="flex justify-between">
                <span class="text-muted-foreground">工作空间</span>
                <span class="font-medium">{{ preview!.workspaceName }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">角色</span>
                <span class="font-medium">{{ roleLabel[preview!.role] ?? preview!.role }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted-foreground">邀请邮箱</span>
                <span class="font-medium">{{ preview!.email }}</span>
              </div>
            </div>

            <!-- Register form -->
            <form class="flex flex-col gap-3" @submit.prevent="submitRegister">
              <!-- Email pre-filled and locked -->
              <div class="flex flex-col gap-1.5">
                <label class="text-sm font-medium">邮箱</label>
                <Input :value="preview!.email" disabled class="bg-muted" />
              </div>
              <div class="flex flex-col gap-1.5">
                <label class="text-sm font-medium">用户名</label>
                <Input v-model="form.username" placeholder="设置用户名" required />
              </div>
              <div class="flex flex-col gap-1.5">
                <label class="text-sm font-medium">密码</label>
                <Input v-model="form.password" type="password" placeholder="至少 6 位" required minlength="6" />
              </div>
              <Button type="submit" :disabled="submitting" class="mt-1">
                <Loader2 v-if="submitting" class="size-4 mr-2 animate-spin" />
                注册并加入
              </Button>
            </form>

            <p class="text-center text-sm text-muted-foreground">
              已有账号？
              <router-link
                :to="`/auth/login?redirect=/invite/${token}`"
                class="underline underline-offset-4 hover:text-foreground"
              >登录后接受邀请</router-link>
            </p>
          </div>
        </template>

      </div>
    </div>
  </div>
</template>
