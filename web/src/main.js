import { createApp} from 'vue'
import {createPinia} from "pinia";
import App from './App.vue'
import router from './router/index.ts'
// import { createPinia } from 'pinia' // 1. 引入核心方法
import './assets/main.css'
import { useTheme } from './composables/useTheme.ts'

// 初始化主题（必须）
useTheme()
const app = createApp(App)
const pinia = createPinia() // 2. 创建 pinia 实例

app.use(pinia) // 3. 挂载到 Vue 实例（这就是你要找的那行代码）
app.use(router)

app.mount('#app')
