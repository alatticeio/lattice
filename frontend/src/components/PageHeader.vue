<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  Breadcrumb, BreadcrumbItem, BreadcrumbLink,
  BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'

defineProps<{
  title: string
  description?: string
}>()

const route = useRoute()
const { t, te } = useI18n()

const breadcrumbs = computed(() => {
  const segments = route.path.split('/').filter(Boolean)
  return segments.map((seg, i) => {
    const path = '/' + segments.slice(0, i + 1).join('/')
    const key = `common.breadcrumb.${seg}`
    const label = te(key) ? t(key) : seg.charAt(0).toUpperCase() + seg.slice(1)
    return { label, path, isLast: i === segments.length - 1 }
  })
})
</script>

<template>
  <div class="border-border flex items-center justify-between gap-4 px-6 py-4">
    <!-- Left: Title + Description -->
    <div class="shrink-0 text-left">
      <h1 class="text-lg font-semibold leading-none tracking-tight">{{ title }}</h1>
      <p v-if="description" class="text-muted-foreground mt-0.5 text-xs">{{ description }}</p>
    </div>

    <!-- Right: Breadcrumb -->
    <Breadcrumb>
      <BreadcrumbList>
        <BreadcrumbItem>
          <BreadcrumbLink href="/">{{ t('common.breadcrumb.home') }}</BreadcrumbLink>
        </BreadcrumbItem>
        <template v-for="crumb in breadcrumbs" :key="crumb.path">
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage v-if="crumb.isLast">{{ crumb.label }}</BreadcrumbPage>
            <BreadcrumbLink v-else :href="crumb.path">{{ crumb.label }}</BreadcrumbLink>
          </BreadcrumbItem>
        </template>
      </BreadcrumbList>
    </Breadcrumb>
  </div>
</template>
