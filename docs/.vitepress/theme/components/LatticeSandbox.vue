<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'

const terminalEl = ref<HTMLDivElement>()
let term: Terminal | null = null
let timeouts: ReturnType<typeof setTimeout>[] = []
let handleResize: (() => void) | null = null

onMounted(() => {
  term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: '"JetBrains Mono", "Fira Code", monospace',
    theme: {
      background: '#1a1b26',
      foreground: '#a9b1d6',
      cursor: '#c0caf5',
    },
  })

  const fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.loadAddon(new WebLinksAddon())

  term.open(terminalEl.value!)
  fitAddon.fit()

  // Simulated demo experience
  const lines = [
    '$ lattice --version\r\n',
    'lattice v0.3.0\r\n\n',
    '$ latticed start --dev\r\n',
    'INF Starting LatticeD all-in-one...\r\n',
    'INF NATS server started on :4222\r\n',
    'INF Web UI available at http://localhost:8080\r\n\n',
    '$ lattice sandbox start --name agent-001 --token lt_demo\r\n',
    'INF Registering sandbox via NATS name=agent-001\r\n',
    'INF gVisor sandbox initialized localIP=10.42.0.5\r\n',
    'INF ICE tunnel established peer=10.42.0.1\r\n',
    'INF Tunnel status: READY\r\n\n',
    '$ lattice policy create allow-tools --port 443 --target app=tools\r\n',
    'INF Policy "allow-tools" created\r\n',
    'INF Policy active on 2 nodes\r\n',
  ]

  let i = 0
  const typeNextLine = () => {
    if (i < lines.length) {
      term!.write(lines[i])
      i++
      if (lines[i - 1].startsWith('$')) {
        timeouts.push(setTimeout(typeNextLine, 800))
      } else {
        timeouts.push(setTimeout(typeNextLine, 300))
      }
    } else {
      term!.write('\r\n$ _')
    }
  }

  timeouts.push(setTimeout(typeNextLine, 500))

  handleResize = () => fitAddon.fit()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  timeouts.forEach(clearTimeout)
  timeouts = []
  if (handleResize) {
    window.removeEventListener('resize', handleResize)
    handleResize = null
  }
  if (term) {
    term.dispose()
    term = null
  }
})
</script>

<template>
  <div class="sandbox-container">
    <div class="sandbox-header">
      <span class="dot red"></span>
      <span class="dot yellow"></span>
      <span class="dot green"></span>
      <span class="title">Lattice Sandbox — lattice sandbox start</span>
    </div>
    <div ref="terminalEl" class="sandbox-terminal"></div>
  </div>
</template>

<style scoped>
.sandbox-container {
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.3);
  margin: 2rem 0;
}

.sandbox-header {
  background: #2a2b3d;
  padding: 10px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.dot.red    { background: #ff5f56; }
.dot.yellow { background: #ffbd2e; }
.dot.green  { background: #27c93f; }

.title {
  margin-left: 12px;
  font-size: 13px;
  color: #787c99;
  font-family: system-ui, sans-serif;
}

.sandbox-terminal {
  height: 420px;
}

.sandbox-terminal :deep(.xterm) {
  height: 100%;
  padding: 8px;
}
</style>
