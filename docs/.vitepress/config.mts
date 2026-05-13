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
      { text: 'Docs', link: '/guide/quickstart' },
      { text: 'Deploy', link: '/deploy/all-in-one' },
      { text: 'AI', link: '/ai/' },
      { text: 'Blog', link: '/blog/' },
      { text: 'Compare', link: '/comparison' },
      { text: 'Console', link: 'https://alattice.io' },
    ],
    sidebar: {
      // ── User-facing docs ──────────────────────────────────────────────────
      '/guide/': userSidebar(),
      '/deploy/': userSidebar(),
      '/config/': userSidebar(),
      '/features/': userSidebar(),
      '/ai/': userSidebar(),
      '/faq/': userSidebar(),

      // ── Internal / developer docs ─────────────────────────────────────────
      '/design/': designSidebar(),
      '/adr/': designSidebar(),
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/alatticeio/lattice' },
    ],
    footer: {
      message: 'Built with Lattice',
      copyright: '© 2026 The Lattice Authors',
    },
  },
})

function userSidebar() {
  return [
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
      text: 'AI',
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
      text: 'Features',
      items: [
        {
          text: 'Networking',
          collapsed: false,
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
          collapsed: false,
          items: [
            { text: 'Workspaces & Members', link: '/features/workspaces' },
            { text: 'Relays', link: '/features/relays' },
            { text: 'Monitoring', link: '/features/monitoring' },
            { text: 'Alerts & Rules', link: '/features/alerts' },
            { text: 'Audit Logging', link: '/features/audit' },
            { text: 'Notifications', link: '/features/notifications' },
            { text: 'Approvals', link: '/features/approvals' },
          ],
        },
        {
          text: 'Account',
          collapsed: true,
          items: [
            { text: 'Profile & Settings', link: '/features/account' },
            { text: 'Billing', link: '/features/billing' },
          ],
        },
      ],
    },
    {
      text: 'FAQ',
      items: [
        { text: 'eBPF & Agent Sandbox', link: '/faq/ebpf-sandbox' },
      ],
    },
    {
      text: 'How-to Guides',
      items: [
        { text: 'Multi-Cloud Peering', link: '/guide/multi-cloud-peering' },
        { text: 'Remote Device Onboarding', link: '/guide/remote-device-onboarding' },
        { text: 'AI Agent Zero-Trust', link: '/guide/ai-agent-zero-trust' },
      ],
    },
  ]
}

function designSidebar() {
  return [
    {
      text: 'Architecture',
      items: [
        { text: 'Overview', link: '/design/architecture' },
        { text: 'ICE Connection', link: '/design/ice-connection' },
        { text: 'LRP Relay', link: '/design/lrp' },
        { text: 'ICE + WireGuard Mux', link: '/design/ice-wireguard-mux' },
        { text: 'WRRP / QUIC', link: '/design/wrrp-quic' },
      ],
    },
    {
      text: 'Development',
      items: [
        { text: 'Build Reference', link: '/design/build-reference' },
        { text: 'CI/CD Reference', link: '/design/ci-cd-reference' },
        { text: 'Performance Testing', link: '/design/performance-testing-plan' },
      ],
    },
    {
      text: 'ADR',
      items: [
        { text: '0001 - Performance Benchmark', link: '/adr/0001-performance-benchmark-design' },
      ],
    },
  ]
}
