<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { reactive, ref } from 'vue'
import { useUserStore } from "@/stores/user";

const props = defineProps<{
  class?: HTMLAttributes["class"]
}>()

const form = reactive({
  username: '',
  password: '',
})
const useStore = useUserStore()
const isLoading = ref(false)
</script>

<template>
  <div :class="cn('flex flex-col gap-6', props.class)">
    <Card>
      <CardHeader class="text-center">
        <CardTitle class="text-xl">
          Welcome back
        </CardTitle>
      </CardHeader>
      <CardContent>
        <form @submit.prevent="useStore.login(form)">
          <FieldGroup>
            <Field>
              <FieldLabel>Username</FieldLabel>
              <Input
                id="username"
                v-model="form.username"
                type="text"
                placeholder="your username"
                required
              />
            </Field>
            <Field>
              <div class="flex items-center">
                <FieldLabel for="password">Password</FieldLabel>
                <a href="#" class="ml-auto text-sm underline-offset-4 hover:underline">
                  Forgot your password?
                </a>
              </div>
              <Input
                id="password"
                v-model="form.password"
                type="password"
                required
              />
            </Field>
            <Field>
              <Button type="submit" :disabled="isLoading">
                {{ isLoading ? 'Logging in...' : 'Login' }}
              </Button>
              <FieldDescription class="text-center">
                Don't have an account?
                <router-link to="/auth/signup" class="underline">Sign up</router-link>
              </FieldDescription>
            </Field>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  </div>
</template>
