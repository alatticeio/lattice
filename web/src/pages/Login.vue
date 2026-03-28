<script setup lang="ts">
import {reactive, ref} from 'vue'
import { useRouter } from 'vue-router'
import { useAction } from '@/composables/useApi'
import { login, User } from '@/api/user';

const router = useRouter()

const { loading: isLoading, execute: runLogin } = useAction(login, {
  successMsg: '验证通过，正在同步控制面...',
  errorMsg: '凭证无效，请检查后重试',
  onSuccess: (data:any) => {
    localStorage.setItem("wf_user", data.user || data.email)
    localStorage.setItem("role", data.role)
    localStorage.setItem("wf_token", data.token)
    router.push("/dashboard")
  }
});

const form = reactive({
  username: '',
  password: '',
  remember: '',
})

const handleLogin = async () => {
  await runLogin(form)
}

const handleSocialLogin = (provider: string) => {
  // 保持原有 Dex 逻辑
  const clientId = 'wireflow-server'
  const redirectUri = encodeURIComponent('http://localhost:8080/auth/callback')
  const dexUrl = `http://localhost:5556/dex/auth?client_id=${clientId}&response_type=code&scope=openid+profile+email&redirect_uri=${redirectUri}&state=login_${provider}`
  window.location.href = dexUrl
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center p-6 bg-base-200 transition-colors duration-500 antialiased font-sans">

    <div class="fixed inset-0 overflow-hidden pointer-events-none opacity-20">
      <div class="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary rounded-full blur-[120px]"></div>
      <div class="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary rounded-full blur-[120px]"></div>
    </div>

    <div class="card lg:card-side w-full max-w-5xl bg-base-100 shadow-2xl overflow-hidden border border-base-300 relative z-10 rounded-[2rem]">

      <div class="lg:w-[55%] p-12 lg:p-16 flex flex-col justify-between bg-base-200/50 border-r border-base-300 relative overflow-hidden">
        <div class="absolute inset-0 opacity-[0.05] pointer-events-none bg-[radial-gradient(var(--bc)_1px,transparent_1px)] bg-[size:24px_24px]"></div>

        <div class="relative z-10">
          <div class="flex items-center gap-3 mb-20">
            <div class="w-10 h-10 bg-primary rounded-xl flex items-center justify-center shadow-lg shadow-primary/20">
              <svg width="22" height="22" viewBox="0 0 32 32" fill="none" class="text-primary-content">
                <path d="M18 6L10 18H16L14 26L22 14H16L18 6Z" fill="currentColor"/>
              </svg>
            </div>
            <span class="text-2xl font-black tracking-tighter uppercase italic">Wireflow</span>
          </div>

          <h2 class="text-4xl lg:text-5xl font-black tracking-tighter leading-tight mb-6">
            声明式网络<br/>
            <span class="text-primary">自动化编排中心</span>
          </h2>
          <p class="text-sm font-medium opacity-50 max-w-xs leading-relaxed">
            基于 K8s SA 映射的零信任架构，为您的集群提供安全、高性能且透明的私有 Mesh 组网。
          </p>
        </div>

        <div class="relative z-10 space-y-4">
          <div class="flex items-center gap-3">
            <span class="badge badge-success badge-sm badge-outline font-black tracking-widest text-[9px] uppercase">
              Operational
            </span>
            <span class="text-[10px] font-black opacity-30 uppercase tracking-widest">v2.4.0 Enterprise</span>
          </div>
        </div>
      </div>

      <div class="lg:w-[45%] card-body p-10 lg:p-14 justify-center">
        <div class="mb-10">
          <h3 class="text-2xl font-black tracking-tight">身份认证</h3>
          <p class="text-[10px] font-bold opacity-40 uppercase tracking-[0.2em] mt-1">请输入您的访问凭证</p>
        </div>

        <form @submit.prevent="handleLogin" class="space-y-6">
          <div class="form-control w-full">
            <label class="label py-1">
              <span class="label-text text-[10px] font-black uppercase tracking-widest opacity-40">User Identity</span>
            </label>
            <div class="relative group">
              <div class="absolute inset-y-0 left-4 flex items-center pointer-events-none opacity-30 group-focus-within:text-primary group-focus-within:opacity-100 transition-all">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                  <path d="M16 12a4 4 0 10-8 0 4 4 0 008 0zm0 0v1.5a2.5 2.5 0 005 0V12a9 9 0 10-9 9m4.5-1.206a8.959 8.959 0 01-4.5 1.206"/>
                </svg>
              </div>
              <input v-model="form.username" type="text" placeholder="name@company.com" required
                     class="input input-bordered w-full h-14 pl-12 rounded-2xl bg-base-200/50 border-base-300 focus:border-primary transition-all text-sm font-medium"/>
            </div>
          </div>

          <div class="form-control w-full">
            <div class="flex justify-between items-center py-1">
              <span class="label-text text-[10px] font-black uppercase tracking-widest opacity-40">Access Key</span>
              <router-link to="/forgot-password" class="text-[10px] font-black text-primary uppercase hover:underline">Forgot?</router-link>
            </div>
            <div class="relative group">
              <div class="absolute inset-y-0 left-4 flex items-center pointer-events-none opacity-30 group-focus-within:text-primary group-focus-within:opacity-100 transition-all">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                  <path d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
                </svg>
              </div>
              <input v-model="form.password" type="password" placeholder="••••••••" required
                     class="input input-bordered w-full h-14 pl-12 rounded-2xl bg-base-200/50 border-base-300 focus:border-primary transition-all text-sm font-medium"/>
            </div>
          </div>

          <div class="flex items-center gap-3 px-1">
            <input type="checkbox" v-model="form.remember" class="checkbox checkbox-primary checkbox-sm rounded-lg" />
            <span class="text-xs font-bold opacity-50">保持登录状态</span>
          </div>

          <button type="submit" :disabled="isLoading"
                  class="btn btn-primary w-full h-14 rounded-2xl font-black text-xs uppercase tracking-[0.2em] shadow-lg shadow-primary/20 transition-all active:scale-[0.98]">
            <span v-if="isLoading" class="loading loading-spinner loading-xs"></span>
            {{ isLoading ? '正在建立连接...' : '进入控制面' }}
          </button>

          <div class="divider text-[9px] font-black opacity-30 uppercase tracking-[0.3em]">Quick Access</div>

          <div class="grid grid-cols-3 gap-3">
            <button @click="handleSocialLogin('github')" type="button" class="btn btn-ghost bg-base-200 hover:bg-base-300 rounded-2xl h-14">
              <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/></svg>
            </button>
            <button @click="handleSocialLogin('google')" type="button" class="btn btn-ghost bg-base-200 hover:bg-base-300 rounded-2xl h-14">
              <svg class="w-5 h-5" viewBox="0 0 24 24"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/><path d="M12 23c3.11 0 5.72-1.01 7.64-2.74l-3.57-2.77c-.98.66-2.23 1.06-3.87 1.06-2.98 0-5.51-2.02-6.41-4.74H2.18v2.87C4.09 20.41 7.76 23 12 23z" fill="#34A853"/><path d="M5.59 13.81c-.23-.66-.36-1.37-.36-2.11 0-.74.13-1.45.36-2.11V6.73H2.18C1.41 8.28 1 10.09 1 12s.41 3.72 1.18 5.27l3.41-2.66z" fill="#FBBC05"/><path d="M12 5.38c1.69 0 3.21.58 4.41 1.72l3.31-3.31C17.71 1.86 15.11 1 12 1 7.76 1 4.09 3.59 2.18 6.73l3.41 2.66C6.49 6.64 9.02 5.38 12 5.38z" fill="#EA4335"/></svg>
            </button>
            <button @click="handleSocialLogin('ldap')" type="button" class="btn btn-ghost bg-base-200 hover:bg-base-300 rounded-2xl h-14">
              <svg class="w-5 h-5 opacity-70" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24"><path d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </button>
          </div>
        </form>

        <div class="mt-10 pt-6 border-t border-base-300 flex items-center justify-between opacity-40">
          <span class="text-[8px] font-black uppercase tracking-[0.2em]">© 2026 Wireflow SDN</span>
          <div class="flex gap-4">
            <a href="#" class="text-[8px] font-black uppercase hover:text-primary transition-colors">Privacy</a>
            <a href="#" class="text-[8px] font-black uppercase hover:text-primary transition-colors">Docs</a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>