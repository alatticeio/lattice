<script setup lang="ts">
definePage({ meta: { layout: 'blank' } })
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <div class="max-w-3xl mx-auto px-6 py-16 prose dark:prose-invert">
      <h1>隐私政策</h1>
      <p class="text-muted-foreground">最后更新: 2026-05-13</p>

      <h2>1. 我们收集的信息</h2>
      <p>注册时收集：邮箱地址、用户名（可选）。Agent 运行时收集：设备公钥、VPN IP、流量审计日志。</p>

      <h2>2. 信息使用</h2>
      <p>用于提供 WireGuard 网络服务和 AI Agent 沙箱功能。流量审计日志仅用于安全合规和异常检测。</p>

      <h2>3. 数据存储与安全</h2>
      <p>数据存储在加密的数据库中。密码使用 bcrypt 哈希存储。API 通信强制 HTTPS。</p>

      <h2>4. 数据保留与删除</h2>
      <p>账户删除时，关联的 Peer、Token、审计日志在 30 天内清除。可联系 hello@lattice.run 请求数据导出或提前删除。</p>

      <h2>5. 第三方服务</h2>
      <p>SSO/OIDC 登录通过您配置的 Identity Provider 完成。Lattice 不存储第三方密码。</p>

      <h2>6. 联系我们</h2>
      <p>隐私相关问题请发送邮件至 hello@lattice.run</p>
    </div>
  </div>
</template>
