<script setup lang="ts">
import { useRouter } from "vue-router"
import {
  BadgeCheck,
  Bell,
  ChevronsUpDown,
  CreditCard,
  LogOut,
  Sparkles,
  User,
} from "lucide-vue-next"
import { useUserStore } from "@/stores/user"
import AvatarPreset from "@/components/AvatarPreset.vue"

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar"

const props = defineProps<{
  user: {
    name: string
    email: string
    avatar: string
  }
}>()

const { isMobile } = useSidebar()
const router = useRouter()
const userStore = useUserStore()

</script>

<template>
  <SidebarMenu>
    <SidebarMenuItem>
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <SidebarMenuButton
              size="lg"
              class="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
          >
            <AvatarPreset :avatar-url="user.avatar" :size="32" :fallback="user.name" />
            <div class="grid flex-1 text-left text-sm leading-tight">
              <span class="truncate font-medium">{{ userStore.userInfo?.username }}</span>
              <span class="truncate text-xs">{{ user.email }}</span>
            </div>
            <ChevronsUpDown class="ml-auto size-4" />
          </SidebarMenuButton>
        </DropdownMenuTrigger>
        <DropdownMenuContent
            class="w-(--reka-dropdown-menu-trigger-width) min-w-56 rounded-lg"
            :side="isMobile ? 'bottom' : 'right'"
            align="end"
            :side-offset="4"
        >
          <DropdownMenuLabel class="p-0 font-normal">
            <div class="flex items-center gap-2 px-1 py-1.5 text-left text-sm">
              <AvatarPreset :avatar-url="user.avatar" :size="32" :fallback="user.name" />
              <div class="grid flex-1 text-left text-sm leading-tight">
                <span class="truncate font-semibold">{{ user.name }}</span>
                <span class="truncate text-xs">{{ user.email }}</span>
              </div>
            </div>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem v-if="userStore.isPro" @click="router.push('/user/billing')">
              <Sparkles class="text-primary" />
              <span>Plan</span>
              <span class="ml-auto text-[10px] font-bold px-2 py-0.5 rounded-full bg-primary text-primary-foreground">Pro</span>
            </DropdownMenuItem>
            <DropdownMenuItem v-else @click="router.push('/user/billing')">
              <Sparkles />
              Upgrade to Pro
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem @click="router.push('/user/profile')">
              <User />
              Profile
            </DropdownMenuItem>
            <DropdownMenuItem @click="router.push('/user/account')">
              <BadgeCheck />
              Account
            </DropdownMenuItem>
            <DropdownMenuItem @click="router.push('/user/billing')">
              <CreditCard />
              Billing
            </DropdownMenuItem>
            <DropdownMenuItem @click="router.push('/user/notifications')">
              <Bell />
              Notifications
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem @click="userStore.logout()">
            <LogOut />
            Log out
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </SidebarMenuItem>
  </SidebarMenu>
</template>
