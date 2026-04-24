<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Card, CardContent, CardDescription, CardHeader, CardTitle,
} from '@/components/ui/card'
import {
  Field, FieldDescription, FieldGroup, FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { toast } from 'vue-sonner'
import { registerUser } from '@/api/user'

const props = defineProps<{ class?: HTMLAttributes['class'] }>()

const router = useRouter()

const form = reactive({ username: '', password: '', confirm: '' })
const loading = ref(false)

async function handleSubmit() {
  if (form.password.length < 6) {
    toast.error('Password must be at least 6 characters')
    return
  }
  if (form.password !== form.confirm) {
    toast.error('Passwords do not match')
    return
  }

  loading.value = true
  try {
    await registerUser({ username: form.username, password: form.password })
    toast.success('Account created! Please sign in.')
    router.push('/auth/login')
  } catch (e: any) {
    toast.error(e?.response?.data?.message ?? 'Registration failed, please try again')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div :class="cn('flex flex-col gap-6', props.class)">
    <Card>
      <CardHeader class="text-center">
        <CardTitle class="text-xl">Create your account</CardTitle>
        <CardDescription>Enter a username and password to get started</CardDescription>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="handleSubmit">
          <FieldGroup>
            <Field>
              <FieldLabel for="username">Username</FieldLabel>
              <Input
                id="username"
                v-model="form.username"
                type="text"
                placeholder="Choose a username"
                required
                autocomplete="username"
              />
            </Field>
            <Field>
              <FieldLabel for="password">Password</FieldLabel>
              <Input
                id="password"
                v-model="form.password"
                type="password"
                placeholder="At least 6 characters"
                required
                minlength="6"
                autocomplete="new-password"
              />
            </Field>
            <Field>
              <FieldLabel for="confirm-password">Confirm Password</FieldLabel>
              <Input
                id="confirm-password"
                v-model="form.confirm"
                type="password"
                placeholder="Re-enter your password"
                required
                autocomplete="new-password"
              />
            </Field>
            <Field>
              <Button type="submit" :disabled="loading" class="w-full">
                {{ loading ? 'Creating account...' : 'Create Account' }}
              </Button>
              <FieldDescription class="text-center">
                Already have an account?
                <router-link to="/auth/login" class="underline underline-offset-4 hover:text-foreground">
                  Sign in
                </router-link>
              </FieldDescription>
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
