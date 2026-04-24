<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { SidebarInset, SidebarProvider, SidebarTrigger } from '@/components/ui/sidebar'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { Separator } from '@/components/ui/separator'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuGroup,
  DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Input } from '@/components/ui/input'
import AppSidebar from '@/components/app-sidebar/AppSidebar.vue'
import PageHeader from '@/components/PageHeader.vue'
import SettingsPanel from '@/components/SettingsPanel.vue'
import { useAppConfig } from '@/composables/useAppConfig'
import { Search, Sun, Moon, Bell, User, LogOut, CreditCard, LifeBuoy, Settings, Check } from 'lucide-vue-next'
import { useUserStore } from '@/stores/user'
import { setLocale } from '@/locales'
import type { Locale } from '@/locales'

const { config } = useAppConfig()
const router = useRouter()
const userStore = useUserStore()
const { userInfo, logout } = userStore
const { t, locale } = useI18n()

const avatarFallback = computed(() => {
  const name = userInfo?.username ?? userInfo?.email ?? '?'
  return name.slice(0, 2).toUpperCase()
})

const isDark = computed(() => config.value.theme === 'dark'
  || (config.value.theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches))

function toggleTheme() {
  config.value.theme = isDark.value ? 'light' : 'dark'
}

function switchLocale(lang: Locale) {
  setLocale(lang)
}

const localeOptions: { value: Locale; label: string }[] = [
  { value: 'zh-CN', label: '中文' },
  { value: 'en',    label: 'English' },
]

const route = useRoute()
const pageTitle = computed(() => {
  const key = route.meta.titleKey as string | undefined
  return key ? t(key) : (route.meta.title as string ?? '')
})
const pageDescription = computed(() => {
  const key = route.meta.descKey as string | undefined
  return key ? t(key) : (route.meta.description as string | undefined)
})

</script>

<template>
  <SidebarProvider>
    <AppSidebar />

    <SidebarInset class="bg-muted/90 flex flex-col">
      <!-- ── Top Navbar ─────────────────────────────────────────────── -->
      <header class="border-border bg-card sticky top-0 z-30 flex h-14 shrink-0 items-center gap-3 border-b px-4">
        <SidebarTrigger class="-ml-1 shrink-0" />
        <Separator orientation="vertical" class="data-[orientation=vertical]:h-5" />

        <!-- Search -->
        <div class="relative max-w-sm flex-1">
          <Search class="text-muted-foreground absolute left-2.5 top-1/2 size-4 -translate-y-1/2" />
          <Input type="search" placeholder="Search..." class="h-8 rounded-lg pl-8 text-sm" />
        </div>

        <div class="ml-auto flex items-center gap-1">
          <!-- Language switcher -->
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <button class="text-muted-foreground hover:text-foreground hover:bg-muted rounded-lg px-2 py-1.5 text-xs font-medium transition-colors">
                {{ locale === 'zh-CN' ? '中文' : 'EN' }}
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="min-w-32">
              <DropdownMenuItem
                v-for="opt in localeOptions"
                :key="opt.value"
                @click="switchLocale(opt.value)"
                class="flex items-center justify-between"
              >
                {{ opt.label }}
                <Check v-if="locale === opt.value" class="size-3.5 text-primary" />
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <!-- Light / Dark quick toggle -->
          <button
            @click="toggleTheme"
            class="text-muted-foreground hover:text-foreground hover:bg-muted rounded-lg p-2 transition-colors"
            :title="isDark ? 'Switch to light' : 'Switch to dark'"
          >
            <Sun v-if="isDark" class="size-4" />
            <Moon v-else class="size-4" />
          </button>

          <!-- Notifications -->
          <button class="text-muted-foreground hover:text-foreground hover:bg-muted relative rounded-lg p-2 transition-colors">
            <Bell class="size-4" />
            <span class="bg-destructive absolute right-1.5 top-1.5 size-1.5 rounded-full" />
          </button>

          <!-- Settings Panel (slide-out) -->
          <SettingsPanel />

          <Separator orientation="vertical" class="data-[orientation=vertical]:h-5 mx-1" />

          <!-- User dropdown -->
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <button class="hover:ring-border flex items-center gap-2 rounded-lg px-1 py-0.5 transition-colors hover:ring-2">
                <Avatar class="size-7">
                  <AvatarFallback class="bg-primary text-primary-foreground text-xs font-semibold">
                    {{ avatarFallback }}
                  </AvatarFallback>
                </Avatar>
                <div class="hidden text-left md:block">
                  <p class="text-sm font-medium leading-none">{{ userInfo?.username ?? '...' }}</p>
                  <p class="text-muted-foreground text-xs">{{ userInfo?.email ?? '' }}</p>
                </div>
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent class="w-56" align="end">
              <DropdownMenuLabel>
                <div class="flex flex-col">
                  <span>{{ userInfo?.username }}</span>
                  <span class="text-muted-foreground text-xs font-normal">{{ userInfo?.email }}</span>
                </div>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuItem @click="router.push('/user/profile')">
                  <User class="mr-2 size-4" />
                  <span>Profile</span>
                </DropdownMenuItem>
                <DropdownMenuItem @click="router.push('/user/account')">
                  <Settings class="mr-2 size-4" />
                  <span>Account</span>
                </DropdownMenuItem>
                <DropdownMenuItem @click="router.push('/user/billing')">
                  <CreditCard class="mr-2 size-4" />
                  <span>Billing</span>
                </DropdownMenuItem>
                <DropdownMenuItem @click="router.push('/user/notifications')">
                  <Bell class="mr-2 size-4" />
                  <span>Notifications</span>
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <DropdownMenuItem>
                <LifeBuoy class="mr-2 size-4" />
                <span>Support</span>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem class="text-destructive focus:text-destructive" @click="logout">
                <LogOut class="mr-2 size-4" />
                <span>Log out</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>

      <!-- ── Page Header (Title ←→ Breadcrumb) ─────────────────────── -->
      <PageHeader
        v-if="pageTitle"
        :title="pageTitle"
        :description="pageDescription"
      />

      <!-- ── Page Content ───────────────────────────────────────────── -->
      <main class="flex-1 overflow-auto">
        <RouterView />
      </main>
    </SidebarInset>
  </SidebarProvider>
  <ConfirmDialog />
</template>
