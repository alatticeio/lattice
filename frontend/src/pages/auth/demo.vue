<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { setToken, setRefreshToken } from '@/utils/auth'

definePage({ meta: { layout: 'blank' } })

const router = useRouter()
const error = ref('')

onMounted(async () => {
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token')

  if (!token) {
    error.value = 'Missing demo token.'
    return
  }

  try {
    const res = await fetch(`/api/v1/demo/auth?token=${encodeURIComponent(token)}`)
    const body = await res.json()

    if (!res.ok || body.code !== 200) {
      error.value = body.message ?? 'Demo session is invalid or has expired.'
      return
    }

    setToken(body.data.token)
    if (body.data.refreshToken) {
      setRefreshToken(body.data.refreshToken)
    }

    // Set the demo workspace as the active workspace so the dashboard loads it directly
    if (body.data.workspace) {
      const ws = body.data.workspace
      localStorage.setItem('active_ws', JSON.stringify(ws))
      localStorage.setItem('active_ws_id', ws.id)
    }

    const redirect = params.get('redirect')
    const target = (redirect && redirect.startsWith('/')) ? redirect : '/dashboard'
    router.replace(target)
  } catch {
    error.value = 'Network error. Please try again.'
  }
})
</script>

<template>
  <div class="min-h-screen flex items-center justify-center">
    <div v-if="error" class="text-center space-y-4">
      <p class="text-destructive text-sm">{{ error }}</p>
      <a href="/" class="text-sm text-muted-foreground underline">Back to home</a>
    </div>
    <div v-else class="text-muted-foreground text-sm">
      Opening demo console...
    </div>
  </div>
</template>
