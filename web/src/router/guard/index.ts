import type { Router } from "vue-router";
import {setupAuthGuard} from "@/router/guard/authGuard";
import {setupProgressGuard} from "@/router/guard/progressGuard";

export function setupRouterGuard(router: Router) {
    setupAuthGuard(router)
    setupProgressGuard(router)
}