import {createApp} from 'vue'
import App from './App.vue'
import {createPinia} from 'pinia'

import './style.css'

import router from './router'

const pinia = createPinia()
const app = createApp(App)
app.use(router)
app.use(pinia)
app.mount('#app')