import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Lattice',
  description: 'Self-hosted WireGuard mesh orchestration platform',
  cleanUrls: true,
  base: '/',
  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'Lattice Docs',
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/quickstart' },
      { text: 'Deploy', link: '/deploy/all-in-one' },
      { text: 'GitHub', link: 'https://github.com/alatticeio/lattice' },
    ],
    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Quick Start', link: '/guide/quickstart' },
          { text: 'Installation', link: '/guide/installation' },
          { text: 'Agent Setup', link: '/guide/agent' },
        ],
      },
      {
        text: 'Deployment',
        items: [
          { text: 'All-in-One', link: '/deploy/all-in-one' },
          { text: 'Helm Chart', link: '/deploy/helm' },
          { text: 'K8s Operator', link: '/deploy/k8s-operator' },
          { text: 'Configuration', link: '/config/reference' },
        ],
      },
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/alatticeio/lattice' },
    ],
  },
})
