import path from 'node:path'
import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import VueRouter from 'vue-router/vite'
import Layouts from 'vite-plugin-vue-layouts'
import Component from 'unplugin-vue-components/vite'
import AutoImport from 'unplugin-auto-import/vite'
import { VueRouterAutoImports } from 'vue-router/unplugin'

export default defineConfig({
    plugins: [
        VueRouter({
            // how and what folders to scan for files
            routesFolder: [
                {
                    src: 'src/pages',
                    path: '',
                    // override globals
                    exclude: excluded => excluded,
                    filePatterns: filePatterns => filePatterns,
                    extensions: extensions => extensions,
                },
            ],
            // what files should be considered as a pages
            extensions: ['.vue'],
            // what files to include
            filePatterns: ['**/*'],
            // where to generate the types
            dts: './src/types/typed-router.d.ts',
        }),
        vue(),
        Layouts({
            layoutsDirs: 'src/layouts', // 布局文件存放目录
            defaultLayout: 'default'    // 默认布局文件名
        }),
        tailwindcss(),
        AutoImport({
            include: [
                /\.[tj]sx?$/,
                /\.vue$/,
            ],
            imports: [
                'vue',
                VueRouterAutoImports,
            ],
            dirs: [
                'src/composables/**/*.ts',
                'src/constants/**/*.ts',
                'src/stores/**/*.ts',
            ],
            defaultExportByFilename: true,
            dts: 'src/types/auto-import.d.ts',
        }),
        Component({
            dirs: [
                'src/components',
            ],
            collapseSamePrefixes: true,
            directoryAsNamespace: true,
            dts: 'src/types/auto-import-components.d.ts',
        }),
    ],

    resolve: {
        alias: {
            '@': path.resolve(__dirname, './src'),
        },
    },
    build: {
        // 直接输出到 Go embed 目标目录，省去拷贝步骤
        outDir: '../internal/web/dist',
        emptyOutDir: true,
    },
    server: {
        proxy: {
            // 这里的 '/api' 必须和你 request.js 里的 baseURL 匹配
            '/api/v1': {
                target: 'http://localhost:8080', // 你的 Gin 后端地址
                changeOrigin: true,
                // 如果你的 Gin 接口本身没有 /api 前缀，需要重写路径
                // rewrite: (path) => path.replace(/^\/api/, '')
            }
        }
    }
})
