import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Lattice',
  description: 'WireGuard overlay network for AI workloads and infrastructure',
  cleanUrls: true,
  vite: {
    ssr: {
      noExternal: ['@xterm/xterm', '@xterm/addon-fit', '@xterm/addon-web-links'],
    },
  },
  base: '/',
  srcExclude: ['superpowers/**', 'demo/**'],
  ignoreDeadLinks: [
    /^http:\/\/localhost/,
    /^\/demo(?:\/|$)/,
    /^\/features\//,
    /superpowers\//,
  ],
  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'Lattice Docs',
    nav: [
      { text: 'Docs', link: '/guide/quickstart' },
      { text: 'Deploy', link: '/deploy/all-in-one' },
      { text: 'Agent', link: '/agent/' },
      { text: 'AI', link: '/ai/' },
      { text: 'Blog', link: '/blog/' },
      { text: 'Compare', link: '/comparison' },
    ],
    sidebar: {
      // ── User-facing docs ──────────────────────────────────────────────────
      '/guide/': userSidebar(),
      '/deploy/': userSidebar(),
      '/config/': userSidebar(),
      '/features/': userSidebar(),
      '/faq/': userSidebar(),

      // ── Agent Platform ────────────────────────────────────────────────────
      '/agent/': agentSidebar(),

      // ── AI capabilities ───────────────────────────────────────────────────
      '/ai/': aiSidebar(),

      // ── Internal / developer docs ─────────────────────────────────────────
      '/design/': designSidebar(),
      '/adr/': designSidebar(),
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/alatticeio/lattice' },
    ],
    footer: {
      message: 'Built with Lattice · <a href="https://alattice.io">Console</a>',
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
        { text: 'AI Agent Security — Capability Verification', link: '/competitiveness' },
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

function agentSidebar() {
  return [
    {
      text: 'Agent Platform',
      items: [
        { text: 'Overview', link: '/agent/' },
        { text: 'Sandbox (Community)', link: '/agent/sandbox' },
        { text: 'Sandbox (Pro)', link: '/agent/sandbox-pro' },
        { text: 'Sub-agent Delegate API', link: '/agent/delegate-api' },
      ],
    },
  ]
}

function aiSidebar() {
  return [
    {
      text: 'AI Capabilities',
      items: [
        { text: 'Overview', link: '/ai/' },
        { text: 'MCP Server & ChatOps', link: '/ai/mcp-server' },
        { text: 'Agent Enrollment API', link: '/ai/agent-enrollment' },
        { text: 'Intent Engine (Pro)', link: '/ai/intent-engine' },
        { text: 'Time-Travel Debugging (Pro)', link: '/ai/debugging' },
        { text: 'Compliance (Pro)', link: '/ai/compliance' },
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
        { text: 'Sandbox Architecture', link: '/design/sandbox' },
        { text: 'ICE Connection', link: '/design/ice-connection' },
        { text: 'ICE + WireGuard Mux', link: '/design/ice-wireguard-mux' },
        { text: 'WRRP / QUIC', link: '/design/wrrp-quic' },
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
