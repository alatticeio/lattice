<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { ref, reactive, computed } from 'vue'
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

const form = reactive({ username: '', email: '', password: '', confirm: '' })
const loading = ref(false)
const agreedToS = ref(false)

const passwordRules = computed(() => ({
  length:    form.password.length >= 8,
  uppercase: /[A-Z]/.test(form.password),
  lowercase: /[a-z]/.test(form.password),
  digit:     /[0-9]/.test(form.password),
}))

const passwordStrength = computed(() => {
  const rules = passwordRules.value
  const passed = [rules.length, rules.uppercase, rules.lowercase, rules.digit].filter(Boolean).length
  if (passed <= 1) return 'weak'
  if (passed <= 3) return 'medium'
  return 'strong'
})

const passwordValid = computed(() => Object.values(passwordRules.value).every(Boolean))

async function handleSubmit() {
  if (!form.username.trim()) {
    toast.error(t('common.auth.signup.usernameRequiredMsg'))
    return
  }
  if (!form.email.trim()) {
    toast.error(t('common.auth.signup.emailRequiredMsg'))
    return
  }
  if (!passwordValid.value) {
    toast.error(t('common.auth.signup.passwordWeakMsg'))
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
    await registerUser({ username: form.username, email: form.email, password: form.password, tosAccepted: agreedToS.value })
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
              <FieldLabel for="email">{{ t('common.auth.signup.email') }}</FieldLabel>
              <Input
                id="email"
                v-model="form.email"
                type="email"
                :placeholder="t('common.auth.signup.emailPlaceholder')"
                required
                autocomplete="email"
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
                autocomplete="new-password"
              />
              <!-- Password strength bar -->
              <div v-if="form.password" class="mt-1.5 space-y-1.5">
                <div class="flex gap-1">
                  <div
                    v-for="i in 4" :key="i"
                    class="h-1 flex-1 rounded-full transition-colors"
                    :class="{
                      'bg-red-500':    passwordStrength === 'weak'   && i <= 1,
                      'bg-yellow-400': passwordStrength === 'medium' && i <= 3,
                      'bg-green-500':  passwordStrength === 'strong' && i <= 4,
                      'bg-muted':      (passwordStrength === 'weak' && i > 1) || (passwordStrength === 'medium' && i > 3),
                    }"
                  />
                </div>
                <ul class="grid grid-cols-2 gap-x-3 gap-y-0.5">
                  <li
                    v-for="[key, label] in [
                      ['length',    t('common.auth.signup.pwdRuleLength')],
                      ['uppercase', t('common.auth.signup.pwdRuleUpper')],
                      ['lowercase', t('common.auth.signup.pwdRuleLower')],
                      ['digit',     t('common.auth.signup.pwdRuleDigit')],
                    ]"
                    :key="key"
                    class="flex items-center gap-1 text-[11px]"
                    :class="(passwordRules as any)[key] ? 'text-green-600' : 'text-muted-foreground'"
                  >
                    <span class="inline-block size-1.5 rounded-full" :class="(passwordRules as any)[key] ? 'bg-green-500' : 'bg-muted-foreground/40'" />
                    {{ label }}
                  </li>
                </ul>
              </div>
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
