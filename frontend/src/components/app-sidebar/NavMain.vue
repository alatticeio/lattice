<script setup lang="ts">
import type { LucideIcon } from "lucide-vue-next"
import { ChevronRight } from "lucide-vue-next"
import { ref, watch } from "vue"
import { RouterLink } from "vue-router"
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar"

const props = defineProps<{
  label?: string
  items: {
    title: string
    url: string
    icon?: LucideIcon
    isActive?: boolean
    items?: {
      title: string
      url: string
    }[]
  }[]
}>()

// Track each group's open state independently.
// Initialized once from isActive; never overwritten by subsequent route changes.
const openState = ref<Record<string, boolean>>({})

watch(
  () => props.items,
  (items) => {
    items.forEach(item => {
      if (!(item.title in openState.value)) {
        openState.value[item.title] = item.isActive ?? false
      }
    })
  },
  { immediate: true },
)
</script>

<template>
  <SidebarGroup>
    <SidebarGroupLabel>{{ label ?? 'Platform' }}</SidebarGroupLabel>
    <SidebarMenu>
      <template v-for="item in items" :key="item.title">
        <!-- Leaf item (no children): plain link -->
        <SidebarMenuItem v-if="!item.items?.length">
          <SidebarMenuButton as-child :tooltip="item.title">
            <RouterLink :to="item.url">
              <component :is="item.icon" v-if="item.icon" />
              <span>{{ item.title }}</span>
            </RouterLink>
          </SidebarMenuButton>
        </SidebarMenuItem>

        <!-- Group item (has children): fully manual expand/collapse -->
        <SidebarMenuItem v-else>
          <SidebarMenuButton
            :tooltip="item.title"
            @click="openState[item.title] = !openState[item.title]"
          >
            <component :is="item.icon" v-if="item.icon" />
            <span>{{ item.title }}</span>
            <ChevronRight
              class="ml-auto transition-transform duration-200"
              :class="{ 'rotate-90': openState[item.title] }"
            />
          </SidebarMenuButton>
          <SidebarMenuSub v-show="openState[item.title]">
            <SidebarMenuSubItem v-for="subItem in item.items" :key="subItem.title">
              <SidebarMenuSubButton as-child>
                <RouterLink :to="subItem.url">
                  <span>{{ subItem.title }}</span>
                </RouterLink>
              </SidebarMenuSubButton>
            </SidebarMenuSubItem>
          </SidebarMenuSub>
        </SidebarMenuItem>
      </template>
    </SidebarMenu>
  </SidebarGroup>
</template>
