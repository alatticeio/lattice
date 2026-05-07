import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Lattice',
  description: 'Self-hosted WireGuard mesh orchestration platform',
  cleanUrls: true,
  vite: {
    ssr: {
      noExternal: ['@xterm/xterm', '@xterm/addon-fit', '@xterm/addon-web-links'],
    },
  },
  base: '/',
  ignoreDeadLinks: [
    /^http:\/\/localhost/,
    /^\/demo(?:\/|$)/,
    /^\/features\//,
  ],
  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'Lattice Docs',
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/quickstart' },
      { text: 'Deploy', link: '/deploy/all-in-one' },
      { text: 'Demo', link: '/demo/' },
      { text: 'Guides', link: '/guides/multi-cloud-peering' },
      { text: 'Blog', link: '/blog/' },
      { text: 'vs.', link: '/comparison' },
      { text: 'Console', link: 'https://alattice.io' },
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
      {
        text: 'AI Capabilities',
        items: [
          { text: 'Overview', link: '/ai/' },
          { text: 'MCP Server & ChatOps', link: '/ai/mcp-server' },
          { text: 'Agent Enrollment', link: '/ai/agent-enrollment' },
          { text: 'Intent Engine (Pro)', link: '/ai/intent-engine' },
          { text: 'Time-Travel Debugging (Pro)', link: '/ai/debugging' },
          { text: 'Compliance (Pro)', link: '/ai/compliance' },
        ],
      },
      {
        text: 'Network Management',
        items: [
          { text: 'Nodes & Peers', link: '/features/nodes' },
          { text: 'Network Policies', link: '/features/policies' },
          { text: 'Topology Viewer', link: '/features/topology' },
          { text: 'Cluster Peering', link: '/features/cluster-peering' },
          { text: 'Network Peering', link: '/features/network-peering' },
        ],
      },
      {
        text: 'Platform',
        items: [
          { text: 'Workspaces & Members', link: '/features/workspaces' },
          { text: 'Alerts & Rules', link: '/features/alerts' },
          { text: 'Monitoring', link: '/features/monitoring' },
          { text: 'Relays', link: '/features/relays' },
          { text: 'Audit Logging', link: '/features/audit' },
        ],
      },
      {
        text: 'User Settings',
        items: [
          { text: 'Account & Profile', link: '/features/account' },
          { text: 'Billing', link: '/features/billing' },
          { text: 'Notifications', link: '/features/notifications' },
          { text: 'Approvals', link: '/features/approvals' },
        ],
      },
      {
        text: 'Guides',
        items: [
          { text: 'Multi-Cloud Peering', link: '/guides/multi-cloud-peering' },
          { text: 'Remote Device Onboarding', link: '/guides/remote-device-onboarding' },
          { text: 'AI Agent Zero-Trust', link: '/guides/ai-agent-zero-trust' },
        ],
      },
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/alatticeio/lattice' },
    ],
    footer: {
      message: 'Built with Lattice',
      copyright: '© 2026 The Lattice Authors',
    },
  },
})
