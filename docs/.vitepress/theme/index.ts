import DefaultTheme from 'vitepress/theme'
import './style.css'
import LatticeSandbox from './components/LatticeSandbox.vue'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('LatticeSandbox', LatticeSandbox)
  },
}
