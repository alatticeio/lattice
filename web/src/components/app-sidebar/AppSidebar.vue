<script setup lang="ts">
import type {SidebarProps} from "@/components/ui/sidebar"
import {Sidebar, SidebarContent, SidebarFooter, SidebarHeader, SidebarRail,} from "@/components/ui/sidebar"

import { LayoutDashboard, Server, Settings2, Zap } from "lucide-vue-next"
import NavMain from "@/components/app-sidebar/NavMain.vue"
import NavUser from "@/components/app-sidebar/NavUser.vue"
import TeamSwitcher from "@/components/app-sidebar/TeamSwitcher.vue"
import { useUserStore } from '@/stores/user'

const props = withDefaults(defineProps<SidebarProps>(), {
  collapsible: "icon",
})

const userStore = useUserStore()
const navUser = computed(() => ({
  name: userStore.userInfo?.username ?? '...',
  email: userStore.userInfo?.email ?? '',
  avatar: userStore.userInfo?.avatarUrl ?? '',
}))

const data = {
  navMain: [
    {
      title: "Quickstart",
      url: "/manage/stepper",
      icon: Zap,
      items: [],
    },
    {
      title: "Dashboard",
      url: "/dashboard",
      icon: LayoutDashboard,
      items: [],
    },
    {
      title: "Management",
      url: "#",
      icon: Server,
      isActive: true,
      items: [
        {
          title: "Memberes",
          url: "/manage/members",
        },
        {
          title: "topology",
          url: "/manage/topology",
        },
        {
          title: "Nodes",
          url: "/manage/nodes",
        },
        {
          title: "Workspaces",
          url: "/manage/workspaces",
        },
        {
          title: "Tokens",
          url: "/manage/tokens",
        },
        {
          title: "Policies",
          url: "/manage/policies",
        },
        {
          title: "Peering",
          url: "/manage/peers",
        },
      ],
    },
    {
      title: "Settings",
      url: "#",
      icon: Settings2,
      items: [
        {
          title: "General",
          url: "#",
        },
        {
          title: "Team",
          url: "#",
        },
        {
          title: "Billing",
          url: "#",
        },
        {
          title: "Limits",
          url: "#",
        },
      ],
    },
  ],
}
</script>

<template>
  <Sidebar v-bind="props">
    <SidebarHeader>
      <TeamSwitcher />
    </SidebarHeader>
    <SidebarContent>
      <NavMain :items="data.navMain"/>
      <!--      <NavMain label="网络管理" :items="data.navNetwork" />-->
      <!--      <NavProjects :projects="data.projects" />-->
    </SidebarContent>
    <SidebarFooter>
      <NavUser :user="navUser"/>
    </SidebarFooter>
    <SidebarRail/>
  </Sidebar>
</template>
