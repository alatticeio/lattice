// src/env.d.ts
declare module '*.vue' {
    import type { DefineComponent } from 'vue'
    // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unused-vars
    const component: DefineComponent<{}, {}, any>
    export default component
}