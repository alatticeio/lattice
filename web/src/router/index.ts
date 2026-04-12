import { createRouter, createWebHistory } from 'vue-router'
import { routes } from 'vue-router/auto-routes'
import { setupLayouts } from 'virtual:generated-layouts'
import {setupRouterGuard} from "@/router/guard";

const router = createRouter({
    history: createWebHistory(),
    // 这里是关键：用 setupLayouts 包裹 routes
    routes: setupLayouts(routes),
})


setupRouterGuard(router)

export default router