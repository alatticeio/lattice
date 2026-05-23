<script setup lang="ts">
import {
  Sheet, SheetContent, SheetHeader, SheetTitle,
  SheetDescription, SheetTrigger,
} from '@/components/ui/sheet'
import { Separator } from '@/components/ui/separator'
import { Sun, Moon, Monitor, Settings, RotateCcw, Check } from 'lucide-vue-next'
import {
  useAppConfig, COLOR_PRESETS, RADIUS_OPTIONS, FONT_OPTIONS,
} from '@/composables/useAppConfig'
import type { Theme, ColorScheme, RadiusValue, FontFamily } from '@/composables/useAppConfig'

const { config, reset } = useAppConfig()

const themeOptions: { value: Theme; label: string; icon: typeof Sun }[] = [
  { value: 'light',  label: 'Light',  icon: Sun },
  { value: 'dark',   label: 'Dark',   icon: Moon },
  { value: 'system', label: 'System', icon: Monitor },
]

const colorLabels: Record<ColorScheme, string> = {
  zinc: 'Zinc', blue: 'Blue', violet: 'Violet',
  green: 'Green', orange: 'Orange', rose: 'Rose',
}

const radiusPreviewClass: Record<RadiusValue, string> = {
  '0':     'rounded-none',
  '0.25':  'rounded-sm',
  '0.5':   'rounded-md',
  '0.625': 'rounded-lg',
  '0.75':  'rounded-xl',
  '1':     'rounded-2xl',
}
</script>

<template>
  <Sheet>
    <!-- Trigger: Settings button in navbar -->
    <SheetTrigger as-child>
      <button
        class="text-muted-foreground hover:text-foreground hover:bg-muted rounded-lg p-2 transition-colors"
        title="Customization"
      >
        <Settings class="size-4" />
      </button>
    </SheetTrigger>

    <SheetContent side="right" class="flex w-80 flex-col gap-0 p-0 sm:max-w-80">
      <!-- Header -->
      <SheetHeader class="border-border border-b px-5 py-4">
        <div class="flex items-center gap-2">
          <Settings class="text-muted-foreground size-4" />
          <SheetTitle class="text-base font-semibold">Customization</SheetTitle>
        </div>
        <SheetDescription class="text-xs">
          Personalize the look and feel of the interface.
        </SheetDescription>
      </SheetHeader>

      <!-- Scrollable body -->
      <div class="flex-1 space-y-6 overflow-y-auto px-5 py-5">

        <!-- ── Appearance ──────────────────────────────────────────── -->
        <section class="space-y-2.5">
          <p class="text-sm font-semibold">Appearance</p>
          <div class="grid grid-cols-3 gap-2">
            <button
              v-for="opt in themeOptions"
              :key="opt.value"
              @click="config.theme = opt.value"
              class="border-border flex flex-col items-center gap-1.5 rounded-lg border py-3 text-xs font-medium transition-all"
              :class="config.theme === opt.value
                ? 'border-primary bg-primary/8 text-primary shadow-sm'
                : 'text-muted-foreground hover:text-foreground hover:border-foreground/20'"
            >
              <component :is="opt.icon" class="size-4" />
              {{ opt.label }}
            </button>
          </div>
        </section>

        <Separator />

        <!-- ── Color Scheme ────────────────────────────────────────── -->
        <section class="space-y-2.5">
          <p class="text-sm font-semibold">Color Scheme</p>
          <div class="grid grid-cols-3 gap-2">
            <button
              v-for="(preset, key) in COLOR_PRESETS"
              :key="key"
              @click="config.color = key as ColorScheme"
              class="border-border relative flex flex-col items-center gap-2 rounded-lg border p-2.5 text-xs font-medium transition-all"
              :class="config.color === key
                ? 'border-primary shadow-sm'
                : 'text-muted-foreground hover:border-foreground/20'"
            >
              <span
                class="size-6 rounded-full"
                :style="{ backgroundColor: preset.swatch }"
              />
              <span :class="config.color === key ? 'text-foreground' : ''">
                {{ colorLabels[key as ColorScheme] }}
              </span>
              <span
                v-if="config.color === key"
                class="bg-primary text-primary-foreground absolute right-1.5 top-1.5 flex size-3.5 items-center justify-center rounded-full"
              >
                <Check class="size-2.5 stroke-[3]" />
              </span>
            </button>
          </div>
        </section>

        <Separator />

        <!-- ── Border Radius ───────────────────────────────────────── -->
        <section class="space-y-2.5">
          <p class="text-sm font-semibold">Border Radius</p>
          <div class="grid grid-cols-5 gap-1.5">
            <button
              v-for="opt in RADIUS_OPTIONS"
              :key="opt.value"
              @click="config.radius = opt.value"
              class="border-border flex flex-col items-center gap-2 rounded-lg border py-2.5 text-[10px] font-medium transition-all"
              :class="config.radius === opt.value
                ? 'border-primary bg-primary/8 text-primary shadow-sm'
                : 'text-muted-foreground hover:text-foreground hover:border-foreground/20'"
            >
              <!-- Corner-radius preview using border top-left only -->
              <span
                class="size-5 border-2 border-b-0 border-r-0"
                :class="[
                  radiusPreviewClass[opt.value],
                  config.radius === opt.value ? 'border-primary' : 'border-foreground/40'
                ]"
              />
              {{ opt.label }}
            </button>
          </div>
        </section>

        <Separator />

        <!-- ── Font Family ─────────────────────────────────────────── -->
        <section class="space-y-2.5">
          <p class="text-sm font-semibold">Font Family</p>
          <div class="grid grid-cols-3 gap-2">
            <button
              v-for="opt in FONT_OPTIONS"
              :key="opt.value"
              @click="config.font = opt.value as FontFamily"
              class="border-border flex flex-col items-center gap-1.5 rounded-lg border py-3 text-xs font-medium transition-all"
              :class="config.font === opt.value
                ? 'border-primary bg-primary/8 text-primary shadow-sm'
                : 'text-muted-foreground hover:text-foreground hover:border-foreground/20'"
            >
              <span
                class="text-xl font-bold leading-none"
                :style="{ fontFamily: opt.style }"
              >Aa</span>
              {{ opt.label }}
            </button>
          </div>
        </section>

      </div>

      <!-- Footer -->
      <div class="border-border border-t px-5 py-4">
        <button
          @click="reset"
          class="text-muted-foreground hover:text-foreground hover:bg-muted flex w-full items-center justify-center gap-2 rounded-lg py-2 text-sm font-medium transition-colors"
        >
          <RotateCcw class="size-3.5" />
          Reset to defaults
        </button>
      </div>
    </SheetContent>
  </Sheet>
</template>
