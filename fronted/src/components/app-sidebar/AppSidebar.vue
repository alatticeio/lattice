<script setup lang="ts">
import { computed } from "vue"
import { useRoute } from "vue-router"
import { useI18n } from "vue-i18n"
import type { SidebarProps } from "@/components/ui/sidebar"
import {
  Sidebar, SidebarContent, SidebarFooter, SidebarHeader, SidebarRail,
} from "@/components/ui/sidebar"
import {
  LayoutDashboard, Network, Settings2,
  ShieldCheck,
} from "lucide-vue-next"
import NavMain from "@/components/app-sidebar/NavMain.vue"
import NavUser from "@/components/app-sidebar/NavUser.vue"
import TeamSwitcher from "@/components/app-sidebar/TeamSwitcher.vue"
import { useUserStore } from '@/stores/user'

const props = withDefaults(defineProps<SidebarProps>(), {
  collapsible: "icon",
})

const route = useRoute()
const userStore = useUserStore()
const { t } = useI18n()

const navUser = computed(() => ({
  name: userStore.userInfo?.username ?? '...',
  email: userStore.userInfo?.email ?? '',
  avatar: userStore.userInfo?.avatarUrl ?? '',
}))

const navMain = computed(() => {
  const path = route.path
  const isAdmin = userStore.isPlatformAdmin

  const groups = [
    // ── Overview ──────────────────────────────────────────────────
    {
      title: t('nav.group.overview'),
      url: "/dashboard",
      icon: LayoutDashboard,
      items: [
        { title: t('nav.dashboard'),  url: "/dashboard" },
        { title: t('nav.quickstart'), url: "/manage/stepper" },
      ],
    },

    // ── Workspace ─────────────────────────────────────────────────
    {
      title: t('nav.group.workspace'),
      url: "#",
      icon: Network,
      items: [
        { title: t('nav.members'),  url: "/manage/members" },
        { title: t('nav.topology'), url: "/manage/topology" },
        { title: t('nav.nodes'),    url: "/manage/nodes" },
        { title: t('nav.tokens'),   url: "/manage/tokens" },
        { title: t('nav.policies'), url: "/manage/policies" },
        { title: t('nav.peers'),    url: "/manage/peers" },
      ],
    },

    // ── Platform Admin ────────────────────────────────────────────
    ...(isAdmin ? [{
      title: t('nav.group.platform'),
      url: "#",
      icon: ShieldCheck,
      items: [
        { title: t('nav.users'),      url: "/manage/users" },
        { title: t('nav.workspaces'), url: "/manage/workspaces" },
        { title: t('nav.approvals'),  url: "/settings/approvals" },
      ],
    }] : []),

    // ── Settings ──────────────────────────────────────────────────
    {
      title: t('nav.group.settings'),
      url: "#",
      icon: Settings2,
      items: [
        { title: t('nav.relays'), url: "/settings/relays" },
        { title: t('nav.audit'), url: "/settings/audit" },
      ],
    },
  ]

  return groups.map(g => ({
    ...g,
    isActive: g.url !== '#' && path.startsWith(g.url)
      ? true
      : (g.items?.some(i => i.url !== '#' && path.startsWith(i.url)) ?? false),
  }))
})
</script>

<template>
  <Sidebar v-bind="props">
    <SidebarHeader>
      <TeamSwitcher />
    </SidebarHeader>
    <SidebarContent>
      <NavMain :items="navMain" />
    </SidebarContent>
    <SidebarFooter>
      <NavUser :user="navUser" />
    </SidebarFooter>
    <SidebarRail />
  </Sidebar>
</template>
