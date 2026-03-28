<template>
  <div class="login-success-container">
    <div class="loader"></div>
    <p>正在同步登录状态，请稍候...</p>
  </div>
</template>

<script setup>
import { onMounted } from 'vue';
import { useRouter, useRoute } from 'vue-router';
// 如果你用了 Pinia，可以引入 userStore
// import { useUserStore } from '@/store/user';

const router = useRouter();
const route = useRoute();
// const userStore = useUserStore();

onMounted(async () => {
  // 1. 从 URL 获取 token (例如: /login/success?token=xxx)
  const token = route.query.token;

  console.log(token)
  if (token) {
    try {
      // 2. 存储 Token
      // 方式 A: 直接存 localStorage (简单)
      localStorage.setItem('wf_token', token);

      // 方式 B: 存入状态管理 (推荐)
      // await userStore.setToken(token);

      // 3. 登录成功后的跳转
      // 建议跳到 dashboard 或 节点列表页
      router.push({ name: 'dashboard' });
    } catch (error) {
      console.error('登录解析失败:', error);
      router.push('/register?error=auth_failed');
    }
  } else {
    // 如果没有 token，说明非法访问，跳回注册或登录页
    router.push('/register');
  }
});
</script>

<style scoped>
.login-success-container {
  height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  font-family: sans-serif;
}
/* 简单的加载动画 */
.loader {
  border: 4px solid #f3f3f3;
  border-top: 4px solid #3498db;
  border-radius: 50%;
  width: 40px;
  height: 40px;
  animation: spin 1s linear infinite;
  margin-bottom: 20px;
}
@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}
</style>