import path from 'node:path'
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      'virtual:generated-layouts': path.resolve(__dirname, './src/__mocks__/virtual-layouts.ts'),
      'vue-router/auto-routes': path.resolve(__dirname, './src/__mocks__/auto-routes.ts'),
    },
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: ['./vitest.setup.ts'],
    include: ['src/**/*.{test,spec}.{js,ts}'],
  },
})
