<script setup lang="ts">
import type { DialogTitleProps } from "reka-ui"
import type { HTMLAttributes } from "vue"
import { reactiveOmit } from "@vueuse/core"
import { DialogTitle, useForwardProps } from "reka-ui"
import { cn } from "@/lib/utils"

const props = defineProps<DialogTitleProps & { class?: HTMLAttributes["class"] }>()
const delegatedProps = reactiveOmit(props, "class")
const forwarded = useForwardProps(delegatedProps)
</script>

<template>
  <DialogTitle
    data-slot="dialog-title"
    v-bind="forwarded"
    :class="cn('text-lg font-semibold leading-none', props.class)"
  >
    <slot />
  </DialogTitle>
</template>
