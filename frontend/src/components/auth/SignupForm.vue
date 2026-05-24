<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
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

const { t } = useI18n()
const router = useRouter()

const form = reactive({ username: '', password: '', confirm: '' })
const loading = ref(false)
const agreedToS = ref(false)

async function handleSubmit() {
  if (!form.username.trim()) {
    toast.error(t('common.auth.signup.usernameRequiredMsg'))
    return
  }
  if (form.password.length < 6) {
    toast.error(t('common.auth.signup.passwordShortMsg'))
    return
  }
  if (form.password !== form.confirm) {
    toast.error(t('common.auth.signup.passwordMismatchMsg'))
    return
  }
  if (!agreedToS.value) {
    toast.error(t('common.auth.signup.tosRequiredMsg'))
    return
  }

  loading.value = true
  try {
    await registerUser({ username: form.username, password: form.password, tosAccepted: agreedToS.value })
    toast.success(t('common.auth.signup.successMsg'))
    router.push('/auth/login')
  } catch (e: any) {
    toast.error(e?.response?.data?.message ?? t('common.auth.signup.errorMsg'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div :class="cn('flex flex-col gap-6', props.class)">
    <Card>
      <CardHeader class="text-center">
        <CardTitle class="text-xl">{{ t('common.auth.signup.title') }}</CardTitle>
        <CardDescription>{{ t('common.auth.signup.subtitle') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="handleSubmit">
          <FieldGroup>
            <Field>
              <FieldLabel for="username">{{ t('common.auth.signup.username') }}</FieldLabel>
              <Input
                id="username"
                v-model="form.username"
                type="text"
                :placeholder="t('common.auth.signup.usernamePlaceholder')"
                required
                autocomplete="username"
              />
            </Field>
            <Field>
              <FieldLabel for="password">{{ t('common.auth.signup.password') }}</FieldLabel>
              <Input
                id="password"
                v-model="form.password"
                type="password"
                :placeholder="t('common.auth.signup.passwordPlaceholder')"
                required
                minlength="6"
                autocomplete="new-password"
              />
            </Field>
            <Field>
              <FieldLabel for="confirm-password">{{ t('common.auth.signup.confirmPassword') }}</FieldLabel>
              <Input
                id="confirm-password"
                v-model="form.confirm"
                type="password"
                :placeholder="t('common.auth.signup.confirmPasswordPlaceholder')"
                required
                autocomplete="new-password"
              />
            </Field>
            <Field>
              <div class="flex items-start gap-2">
                <input type="checkbox" v-model="agreedToS" class="mt-0.5 accent-primary" />
                <span class="text-xs text-muted-foreground">
                  {{ t('common.auth.signup.tosPrefix') }}
                  <a href="/legal/terms" target="_blank" class="text-primary hover:underline">{{ t('common.auth.signup.tosLink') }}</a>
                  {{ t('common.auth.signup.tosAnd') }}
                  <a href="/legal/privacy" target="_blank" class="text-primary hover:underline">{{ t('common.auth.signup.privacyLink') }}</a>
                </span>
              </div>
            </Field>
            <Field>
              <Button type="submit" :disabled="loading" class="w-full">
                {{ loading ? t('common.auth.signup.submitting') : t('common.auth.signup.submit') }}
              </Button>
              <FieldDescription class="text-center">
                {{ t('common.auth.signup.hasAccount') }}
                <router-link to="/auth/login" class="underline underline-offset-4 hover:text-foreground">
                  {{ t('common.auth.signup.signIn') }}
                </router-link>
              </FieldDescription>
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
