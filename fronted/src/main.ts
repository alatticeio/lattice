import {createApp} from 'vue'
import App from './App.vue'
import {createPinia} from 'pinia'

import './style.css'

import router from './router'
import { i18n } from './locales'

const pinia = createPinia()
const app = createApp(App)
app.use(router)
app.use(pinia)
app.use(i18n)
app.mount('#app')