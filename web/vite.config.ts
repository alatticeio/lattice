import {defineConfig} from 'vite';
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite';
import { fileURLToPath, URL } from 'node:url' // 引入工具函数

export default defineConfig({
    plugins: [
        vue(),
        tailwindcss(),
    ],
    resolve: {
        alias: {
            // 告诉 Vite，凡是 @ 开头的路径，都去 src 目录下找
            '@': fileURLToPath(new URL('./src', import.meta.url))
        }
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
});
