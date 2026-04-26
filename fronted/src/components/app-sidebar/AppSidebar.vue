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
  ShieldCheck, Bot,
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
      title: t('common.nav.group.overview'),
      url: "/dashboard",
      icon: LayoutDashboard,
      items: [
        { title: t('common.nav.dashboard'),  url: "/dashboard" },
        { title: t('common.nav.quickstart'), url: "/manage/stepper" },
      ],
    },

    // ── Workspace ─────────────────────────────────────────────────
    {
      title: t('common.nav.group.workspace'),
      url: "#",
      icon: Network,
      items: [
        { title: t('common.nav.members'),  url: "/manage/members" },
        { title: t('common.nav.topology'), url: "/manage/topology" },
        { title: t('common.nav.nodes'),    url: "/manage/nodes" },
        { title: t('common.nav.tokens'),   url: "/manage/tokens" },
        { title: t('common.nav.policies'), url: "/manage/policies" },
        { title: t('common.nav.peers'),    url: "/manage/peers" },
      ],
    },

    // ── Platform Admin ────────────────────────────────────────────
    ...(isAdmin ? [{
      title: t('common.nav.group.platform'),
      url: "#",
      icon: ShieldCheck,
      items: [
        { title: t('common.nav.users'),      url: "/manage/users" },
        { title: t('common.nav.workspaces'), url: "/manage/workspaces" },
        { title: t('common.nav.approvals'),  url: "/settings/approvals" },
      ],
    }] : []),

    // ── AI Assistant ──────────────────────────────────────────────
    {
      title: t('common.nav.group.ai'),
      url: '/ai',
      icon: Bot,
      items: [
        { title: t('common.nav.ai'), url: '/ai' },
      ],
    },

    // ── Settings ──────────────────────────────────────────────────
    {
      title: t('common.nav.group.settings'),
      url: "#",
      icon: Settings2,
      items: [
        { title: t('common.nav.relays'), url: "/settings/relays" },
        { title: t('common.nav.audit'), url: "/settings/audit" },
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
