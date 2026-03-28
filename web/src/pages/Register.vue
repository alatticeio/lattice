<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useApi } from '@/composables/useApi'
import { registerUser } from '@/api/user';

const router = useRouter()

const { loading: isLoading, error, execute: register } = useApi(registerUser)

const form = ref({
  orgName: '',
  fullName: '',
  email: '',
  password: '',
  agree: false
})

const passwordStrength = computed(() => {
  if (form.value.password.length === 0) return 0
  if (form.value.password.length < 8) return 40
  return 100
})

const strengthColor = computed(() => passwordStrength.value < 100 ? 'bg-amber-500' : 'bg-emerald-500')

const handleRegister = async () => {
  if (!form.value.agree) return
  console.log("提交的数据：", form.value) // <--- 关键排查：看这里有没有值
  const { success } = await register(form.value)
  if (success) {
    isLoading.value = true // 再次开启加载状态，模拟“正在跳转...”

    setTimeout(() => {
      isLoading.value = false
      // 3. 执行路由跳转
      router.push('/login')
    }, 1200) // 1.2秒后执行
  }
}

</script>

<template>
  <div class="min-h-screen flex items-center justify-center px-4 py-4 bg-slate-50 dark:bg-slate-950 transition-colors duration-500">

    <div class="w-full max-w-4xl max-h-[90vh] grid grid-cols-1 lg:grid-cols-[0.8fr_1.2fr] gap-0 rounded-[2rem] overflow-hidden shadow-2xl border border-slate-200/60 dark:border-white/5 bg-base-100 dark:bg-slate-900 relative z-10">

      <div class="relative p-8 lg:p-10 flex flex-col justify-between bg-blue-600 hidden lg:flex">
        <div class="relative z-10">
          <router-link to="/login" class="flex items-center gap-2 mb-10 text-white/90">
            <div class="p-1 bg-base-100/20 rounded-lg">
              <svg width="20" height="20" viewBox="0 0 32 32" fill="none"><path d="M18 6L10 18H16L14 26L22 14H16L18 6Z" fill="white" /></svg>
            </div>
            <span class="text-lg font-black tracking-tighter">Wireflow</span>
          </router-link>

          <h2 class="text-3xl font-black tracking-tighter text-white leading-tight mb-4">
            构建您的<br />专用网格
          </h2>

          <div class="space-y-3">
            <div v-for="item in ['全球组网', 'WireGuard 加密', 'K8s 原生']" :key="item" class="flex items-center gap-2 text-white/70 text-[11px] font-bold uppercase tracking-wider">
              <svg class="w-4 h-4 text-emerald-400" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24"><path d="M5 13l4 4L19 7"/></svg>
              {{ item }}
            </div>
          </div>
        </div>

        <div class="relative z-10">
          <div class="p-5 bg-base-100/5 backdrop-blur-md rounded-2xl border border-white/10">
            <p class="text-white/80 text-[10px] leading-relaxed italic">
              "Wireflow 极大地简化了我们跨区域集群的通信成本。"
            </p>
          </div>
        </div>
      </div>

      <div class="p-8 lg:p-10 bg-base-100 dark:bg-slate-900 overflow-y-auto">
        <div class="mb-6">
          <h3 class="text-xl font-black tracking-tight text-base-content dark:text-white uppercase">创建账户</h3>
          <p class="text-[10px] font-bold text-base-content/40 uppercase mt-1 tracking-[0.15em]">完成注册以初始化隔离空间</p>
        </div>

        <form @submit.prevent="handleRegister" class="grid grid-cols-2 gap-4">
          <div class="col-span-2 space-y-1.5">
            <label class="text-[9px] font-black uppercase tracking-widest text-base-content/40 ml-1">组织名称</label>
            <input v-model="form.orgName" type="text" placeholder="Acme Corp" required
                   class="w-full h-11 px-4 bg-slate-100 dark:bg-base-100/5 border-none rounded-xl text-sm outline-none focus:ring-2 focus:ring-blue-500/20 transition-all dark:text-white" />
          </div>

          <div class="col-span-2 lg:col-span-1 space-y-1.5">
            <label class="text-[9px] font-black uppercase tracking-widest text-base-content/40 ml-1">姓名</label>
            <input v-model="form.fullName" type="text" placeholder="Jason" required
                   class="w-full h-11 px-4 bg-slate-100 dark:bg-base-100/5 border-none rounded-xl text-sm outline-none focus:ring-2 focus:ring-blue-500/20 transition-all dark:text-white" />
          </div>

          <div class="col-span-2 lg:col-span-1 space-y-1.5">
            <label class="text-[9px] font-black uppercase tracking-widest text-base-content/40 ml-1">工作邮箱</label>
            <input v-model="form.email" type="email" placeholder="admin@acme.com" required
                   class="w-full h-11 px-4 bg-slate-100 dark:bg-base-100/5 border-none rounded-xl text-sm outline-none focus:ring-2 focus:ring-blue-500/20 transition-all dark:text-white" />
          </div>

          <div class="col-span-2 space-y-1.5">
            <label class="text-[9px] font-black uppercase tracking-widest text-base-content/40 ml-1">设置密码</label>
            <input v-model="form.password" type="password" placeholder="Min 8 characters" required
                   class="w-full h-11 px-4 bg-slate-100 dark:bg-base-100/5 border-none rounded-xl text-sm outline-none focus:ring-2 focus:ring-blue-500/20 transition-all dark:text-white" />
            <div class="h-1 w-full bg-slate-100 dark:bg-base-100/5 rounded-full overflow-hidden mt-1">
              <div class="h-full transition-all duration-500" :class="strengthColor" :style="{ width: passwordStrength + '%' }"></div>
            </div>
          </div>

          <div class="col-span-2 flex items-start gap-2.5 mt-2 px-1">
            <input
                type="checkbox"
                v-model="form.agree"
                required
                class="checkbox checkbox-xs checkbox-primary rounded-md flex-shrink-0 mt-[1px] border-slate-300 dark:border-white/20"
            />

            <label class="text-[10px] font-medium text-base-content/60 dark:text-base-content/40 leading-normal cursor-pointer select-none">
              我已阅读并同意
              <a href="#" class="text-blue-600 font-bold hover:underline">服务条款</a>
              与
              <a href="#" class="text-blue-600 font-bold hover:underline">隐私政策</a>，
              并确认初始化 K8s 资源。
            </label>
          </div>

          <button type="submit" :disabled="isLoading || !form.agree"
                  class="col-span-2 h-12 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white rounded-xl font-black text-[10px] uppercase tracking-widest shadow-lg shadow-blue-500/10 transition-all active:scale-95 mt-2 flex items-center justify-center gap-2">
            <span v-if="isLoading" class="loading loading-spinner loading-xs"></span>
            {{ isLoading ? '正在拨备资源...' : '创建并初始化' }}
          </button>
        </form>

        <p class="mt-6 text-center text-[10px] font-bold text-base-content/40 uppercase tracking-widest">
          已有账号? <router-link to="/login" class="text-blue-600 ml-1">登录</router-link>
        </p>
      </div>
    </div>
  </div>
</template>