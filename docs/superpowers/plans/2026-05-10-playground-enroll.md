# Playground Guided Enrollment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Connect Your First Real Node" guided UI to the nodes page, backed by a self-hosted install script, so playground users can join their own machines with a single copy-paste command.

**Architecture:** The Go backend embeds `scripts/install.sh` and serves it at `/install.sh` (no auth required). The nodes page shows a dismissable enrollment banner when all nodes are seed-only; clicking "Generate Command" calls `POST /api/v1/agent-enroll` to produce a short-lived token, then assembles and displays the one-liner install command using `window.location.origin` as the server URL.

**Tech Stack:** Go (embed, Gin), Vue 3.5 + TypeScript, vue-i18n, lucide-vue-next, shadcn-vue (Dialog, Card, Button, Input, Badge)

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `scripts/install.sh` | Add `--server` parameter, pass to agent as `--server-url` |
| Create | `internal/server/server/install.go` | Serve `/install.sh` with embed |
| Modify | `internal/server/server/api.go` | Register the `/install.sh` route |
| Modify | `fronted/src/api/user.ts` | Add `enrollAgent` / `revokeAgent` API functions |
| Create | `fronted/src/components/NodeEnrollBanner.vue` | Guided enrollment card component |
| Modify | `fronted/src/pages/manage/nodes/index.vue` | Mount `NodeEnrollBanner` when all nodes are seeds |
| Modify | `fronted/src/locales/zh-CN/manage.json` | Add i18n keys under `nodes.enroll.*` |
| Modify | `fronted/src/locales/en/manage.json` | Add i18n keys under `nodes.enroll.*` |

---

### Task 1: Add `--server` parameter to `scripts/install.sh`

**Files:**
- Modify: `scripts/install.sh`

- [ ] **Step 1: Read the current file**

```bash
cat -n scripts/install.sh
```

- [ ] **Step 2: Add `--server` to the arg-parsing block**

Replace the arg-parsing section (lines 18–24) with:

```sh
SERVER_URL=""
TOKEN=""
NODE_NAME=""

while [ $# -gt 0 ]; do
  case "$1" in
    --server) SERVER_URL="$2"; shift 2 ;;
    --token)  TOKEN="$2";      shift 2 ;;
    --name)   NODE_NAME="$2";  shift 2 ;;
    *)        echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [ -z "$SERVER_URL" ] || [ -z "$TOKEN" ] || [ -z "$NODE_NAME" ]; then
  echo "Usage: install.sh --server <SERVER_URL> --token <ENROLL_TOKEN> --name <NODE_NAME>"
  exit 1
fi
```

- [ ] **Step 3: Pass `--server-url` to systemd service ExecStart**

In the `[Service]` block (currently line 72), change:
```sh
ExecStart=${INSTALL_DIR}/lattice-agent --token "${TOKEN}" --name "${NODE_NAME}"
```
to:
```sh
ExecStart=${INSTALL_DIR}/lattice-agent up --server-url "${SERVER_URL}" --token "${TOKEN}" --name "${NODE_NAME}" --save
```

- [ ] **Step 4: Pass `--server-url` to launchd plist**

In the macOS `ProgramArguments` array (currently lines 91–94), change to:
```xml
  <array>
    <string>${INSTALL_DIR}/lattice-agent</string>
    <string>up</string>
    <string>--server-url</string> <string>${SERVER_URL}</string>
    <string>--token</string>      <string>${TOKEN}</string>
    <string>--name</string>       <string>${NODE_NAME}</string>
    <string>--save</string>
  </array>
```

- [ ] **Step 5: Update the fallback manual-run line** (currently line 103)

```sh
  echo "Run manually: ${INSTALL_DIR}/lattice-agent up --server-url ${SERVER_URL} --token ${TOKEN} --name ${NODE_NAME}"
```

- [ ] **Step 6: Verify the script parses correctly**

```bash
bash -n scripts/install.sh && echo "syntax OK"
```

Expected: `syntax OK`

- [ ] **Step 7: Commit**

```bash
git add scripts/install.sh
git commit -s -m "feat(playground): add --server parameter to install.sh"
```

---

### Task 2: Embed and serve `install.sh` from the Go backend

**Files:**
- Create: `internal/server/server/install.go`
- Modify: `internal/server/server/api.go`

The Go binary embeds `scripts/install.sh` so the same server that runs the playground can serve the install script at `/install.sh`, making the one-liner entirely self-contained.

- [ ] **Step 1: Create `internal/server/server/install.go`**

```go
// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed ../../../scripts/install.sh
var installScript []byte

// installScriptHandler serves the Lattice agent install script.
// No authentication required — it is a public shell script.
func installScriptHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "text/x-shellscript; charset=utf-8", installScript)
	}
}
```

- [ ] **Step 2: Verify the embed path resolves**

From the repo root, confirm the relative path from `internal/server/server/` to `scripts/install.sh` is `../../../scripts/install.sh`:

```bash
ls internal/server/server/../../../scripts/install.sh
```

Expected: file exists.

- [ ] **Step 3: Register the route in `api.go`**

In `internal/server/server/api.go`, inside `apiRouter()`, add this line **before** the SPA registration block (`web.RegisterHandlers`):

```go
// Public install script — served without authentication.
s.GET("/install.sh", installScriptHandler())
```

The registration block already reads:
```go
// SPA static resources: must be registered last, catch all unmatched paths via NoRoute
s.logger.Info("Registering SPA static files")
web.RegisterHandlers(s.Engine)
```

Insert the new `GET` call on the line directly above the comment.

- [ ] **Step 4: Build to verify embed compiles**

```bash
make build SERVICE=latticed
```

Expected: no errors, binary produced at `bin/latticed`.

- [ ] **Step 5: Smoke-test the endpoint**

Start the server locally and curl the script:

```bash
# In a separate terminal, start latticed with a test config
# Then:
curl -s http://localhost:8080/install.sh | head -5
```

Expected: first line is `#!/usr/bin/env sh`.

- [ ] **Step 6: Commit**

```bash
git add internal/server/server/install.go internal/server/server/api.go
git commit -s -m "feat(playground): serve /install.sh from embedded script"
```

---

### Task 3: Add `enrollAgent` and `revokeAgent` API functions to frontend

**Files:**
- Modify: `fronted/src/api/user.ts`

- [ ] **Step 1: Read `fronted/src/api/user.ts`**

Confirm the current end of the file (look for the last export line).

- [ ] **Step 2: Add type definitions and API functions**

Append to the end of `fronted/src/api/user.ts`:

```ts
// ── Agent Enrollment ──────────────────────────────────────────────────────────

export interface AgentEnrollRequest {
  agentName: string
  agentType: string
  workspaceId: string
  ttlSeconds?: number
}

export interface AgentEnrollResponse {
  peerName: string
  enrollmentToken: string
  expiresAt?: string
}

export const enrollAgent = (data: AgentEnrollRequest) =>
  request.post<AgentEnrollResponse>('/agent-enroll', data)

export const revokeAgent = (peerName: string, workspaceId: string) =>
  request.delete(`/agent-enroll/${peerName}`, { workspaceId })
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd fronted && pnpm typecheck 2>&1 | head -20
```

Expected: no errors (or only pre-existing ones unrelated to this change).

- [ ] **Step 4: Commit**

```bash
git add fronted/src/api/user.ts
git commit -s -m "feat(playground): add enrollAgent API client"
```

---

### Task 4: Add i18n keys for enrollment banner

**Files:**
- Modify: `fronted/src/locales/zh-CN/manage.json`
- Modify: `fronted/src/locales/en/manage.json`

- [ ] **Step 1: Read the existing `nodes` section in `zh-CN/manage.json`**

Confirm the closing `}` of the `nodes` object is at which line. Look for `"nodes": {` and find its closing brace.

- [ ] **Step 2: Add keys to `zh-CN/manage.json`**

Inside the `nodes` object, before the final closing `}`, add an `enroll` section:

```json
    "enroll": {
      "bannerTitle": "连接你的第一台真实节点",
      "bannerDesc": "当前网络仅包含演示数据。运行下方命令，30 秒内你的机器就会出现在节点列表中。",
      "namePlaceholder": "节点名称（如 my-laptop）",
      "generateBtn": "生成安装命令",
      "generating": "生成中...",
      "commandTitle": "在目标机器上运行以下命令：",
      "copied": "已复制",
      "copyBtn": "复制",
      "dismiss": "暂时隐藏",
      "tokenExpiry": "Token 有效期 24 小时",
      "errorNoWorkspace": "请先选择工作空间",
      "errorGenerate": "生成失败，请重试"
    }
```

The resulting structure under `"nodes"` looks like:

```json
"nodes": {
  "title": "节点管理",
  ...
  "detail": { ... },
  "enroll": {
    "bannerTitle": "连接你的第一台真实节点",
    ...
  }
}
```

- [ ] **Step 3: Add keys to `en/manage.json`**

Same location inside `nodes`, add:

```json
    "enroll": {
      "bannerTitle": "Connect Your First Real Node",
      "bannerDesc": "This network contains only demo data. Run the command below — your machine will appear in the node list within 30 seconds.",
      "namePlaceholder": "Node name (e.g. my-laptop)",
      "generateBtn": "Generate Install Command",
      "generating": "Generating...",
      "commandTitle": "Run this command on your machine:",
      "copied": "Copied",
      "copyBtn": "Copy",
      "dismiss": "Dismiss",
      "tokenExpiry": "Token valid for 24 hours",
      "errorNoWorkspace": "Please select a workspace first",
      "errorGenerate": "Failed to generate, please try again"
    }
```

- [ ] **Step 4: Validate JSON syntax**

```bash
node -e "JSON.parse(require('fs').readFileSync('fronted/src/locales/zh-CN/manage.json','utf8'))" && echo "zh-CN OK"
node -e "JSON.parse(require('fs').readFileSync('fronted/src/locales/en/manage.json','utf8'))" && echo "en OK"
```

Expected: both print `OK`.

- [ ] **Step 5: Commit**

```bash
git add fronted/src/locales/zh-CN/manage.json fronted/src/locales/en/manage.json
git commit -s -m "feat(playground): add enroll i18n keys"
```

---

### Task 5: Create `NodeEnrollBanner.vue` component

**Files:**
- Create: `fronted/src/components/NodeEnrollBanner.vue`

The banner shows at the top of the nodes page when all visible nodes are seed nodes. It lets the user enter a node name, generates an enrollment token, and shows a copyable one-liner install command.

- [ ] **Step 1: Create the file**

```vue
<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Terminal, Copy, Check, ChevronDown, ChevronUp, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { enrollAgent } from '@/api/user'
import { useWorkspaceStore } from '@/stores/workspace'
import { toast } from 'vue-sonner'

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()

const nodeName    = ref('my-laptop')
const loading     = ref(false)
const dismissed   = ref(false)
const expanded    = ref(true)
const command     = ref('')
const copied      = ref(false)

const serverURL = computed(() => window.location.origin)

async function generate() {
  const ws = workspaceStore.currentWorkspace
  if (!ws) {
    toast.error(t('manage.nodes.enroll.errorNoWorkspace'))
    return
  }
  const name = nodeName.value.trim()
  if (!name) return

  loading.value = true
  command.value = ''
  try {
    const { data, code } = await enrollAgent({
      agentName:   name,
      agentType:   'node',
      workspaceId: ws.id,
    })
    if (code !== 200 || !data?.enrollmentToken) {
      toast.error(t('manage.nodes.enroll.errorGenerate'))
      return
    }
    command.value = [
      `curl -sSL ${serverURL.value}/install.sh |`,
      `  sh -s -- \\`,
      `  --server ${serverURL.value} \\`,
      `  --token  ${data.enrollmentToken} \\`,
      `  --name   ${name}`,
    ].join('\n')
  } catch {
    toast.error(t('manage.nodes.enroll.errorGenerate'))
  } finally {
    loading.value = false
  }
}

async function copyCommand() {
  if (!command.value) return
  await navigator.clipboard.writeText(command.value)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}
</script>

<template>
  <div
    v-if="!dismissed"
    class="rounded-xl border border-blue-500/20 bg-blue-500/5 px-5 py-4 transition-all"
  >
    <!-- Header row -->
    <div class="flex items-center justify-between gap-3">
      <div class="flex items-center gap-2.5">
        <div class="size-8 rounded-lg bg-blue-500/10 flex items-center justify-center shrink-0">
          <Terminal class="size-4 text-blue-500" />
        </div>
        <div>
          <p class="text-sm font-semibold leading-none">{{ t('manage.nodes.enroll.bannerTitle') }}</p>
          <p class="text-xs text-muted-foreground mt-0.5">{{ t('manage.nodes.enroll.bannerDesc') }}</p>
        </div>
      </div>
      <div class="flex items-center gap-1 shrink-0">
        <button
          class="p-1 rounded hover:bg-muted/60 text-muted-foreground transition-colors"
          @click="expanded = !expanded"
        >
          <ChevronUp v-if="expanded" class="size-4" />
          <ChevronDown v-else class="size-4" />
        </button>
        <button
          class="p-1 rounded hover:bg-muted/60 text-muted-foreground transition-colors"
          @click="dismissed = true"
        >
          <X class="size-4" />
        </button>
      </div>
    </div>

    <!-- Body: input + generate + command -->
    <div v-if="expanded" class="mt-4 space-y-3">
      <div class="flex gap-2 items-center max-w-md">
        <Input
          v-model="nodeName"
          :placeholder="t('manage.nodes.enroll.namePlaceholder')"
          class="h-8 text-xs"
          @keydown.enter="generate"
        />
        <Button
          size="sm"
          class="shrink-0"
          :disabled="loading || !nodeName.trim()"
          @click="generate"
        >
          {{ loading ? t('manage.nodes.enroll.generating') : t('manage.nodes.enroll.generateBtn') }}
        </Button>
      </div>

      <!-- Generated command block -->
      <div v-if="command" class="rounded-lg bg-neutral-950 dark:bg-black border border-neutral-800 p-3">
        <div class="flex items-center justify-between mb-2">
          <span class="text-[11px] text-neutral-400 font-medium">{{ t('manage.nodes.enroll.commandTitle') }}</span>
          <div class="flex items-center gap-2">
            <span class="text-[11px] text-neutral-500">{{ t('manage.nodes.enroll.tokenExpiry') }}</span>
            <Button
              size="sm"
              variant="outline"
              class="h-6 px-2 text-[11px] border-neutral-700 text-neutral-300 hover:bg-neutral-800 hover:text-white bg-transparent"
              @click="copyCommand"
            >
              <Check v-if="copied" class="size-3 mr-1 text-emerald-400" />
              <Copy v-else class="size-3 mr-1" />
              {{ copied ? t('manage.nodes.enroll.copied') : t('manage.nodes.enroll.copyBtn') }}
            </Button>
          </div>
        </div>
        <pre class="font-mono text-xs text-neutral-200 whitespace-pre-wrap break-all leading-relaxed">{{ command }}</pre>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Verify the component file has no obvious TypeScript errors**

```bash
cd fronted && pnpm typecheck 2>&1 | grep NodeEnrollBanner
```

Expected: no output (no errors referencing this file).

- [ ] **Step 3: Commit**

```bash
git add fronted/src/components/NodeEnrollBanner.vue
git commit -s -m "feat(playground): add NodeEnrollBanner guided enrollment component"
```

---

### Task 6: Integrate `NodeEnrollBanner` into the nodes page

**Files:**
- Modify: `fronted/src/pages/manage/nodes/index.vue`

Show the banner at the top of the page when every node in the list has the `lattice.io/is-seed: "true"` K8s label (i.e., there are no real nodes yet).

- [ ] **Step 1: Read `fronted/src/pages/manage/nodes/index.vue`**

Confirm the import block and the `<template>` opening (around line 1–40 and line 343–348).

- [ ] **Step 2: Add the import at the top of `<script setup>`**

After the existing import block (after the `import { usePeerPageStore } from '@/stores/peerPage'` line), add:

```ts
import NodeEnrollBanner from '@/components/NodeEnrollBanner.vue'
```

- [ ] **Step 3: Add the computed property**

After the `const store = usePeerPageStore()` line, add:

```ts
// Show enrollment banner when every node is a seed (no real nodes yet).
const showEnrollBanner = computed(() => {
  if (store.loading) return false
  if (store.rows.length === 0) return true
  return store.rows.every((n: any) => {
    const labels = n.labels
    if (!labels || typeof labels !== 'object' || Array.isArray(labels)) return false
    return labels['lattice.io/is-seed'] === 'true'
  })
})
```

- [ ] **Step 4: Insert the banner in the template**

In the `<template>` section, immediately after the opening `<div class="flex flex-col gap-5 p-6 ...">` (line ~344), add:

```html
<!-- Guided enrollment banner (shown when all nodes are seed-only) -->
<NodeEnrollBanner v-if="showEnrollBanner" />
```

The final result looks like:

```html
<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">

    <!-- Guided enrollment banner (shown when all nodes are seed-only) -->
    <NodeEnrollBanner v-if="showEnrollBanner" />

    <!-- ── Stat cards ─────────────────────────────── -->
    ...
```

- [ ] **Step 5: Build the frontend to verify no errors**

```bash
cd fronted && pnpm build 2>&1 | tail -20
```

Expected: build succeeds with no TypeScript or Vite errors.

- [ ] **Step 6: Run the dev server and visually verify**

```bash
cd fronted && pnpm dev
```

Open `http://localhost:5173` in a browser, navigate to the Nodes page. Confirm:
1. The blue enrollment banner appears at the top of the page.
2. Entering a name and clicking "Generate Install Command" calls the API.
3. The generated command block appears with a dark code background.
4. The "Copy" button copies the command to clipboard.
5. The "×" dismiss button hides the banner.

- [ ] **Step 7: Commit**

```bash
git add fronted/src/pages/manage/nodes/index.vue
git commit -s -m "feat(playground): show guided enrollment banner on nodes page"
```

---

### Task 7: Commit all remaining playground changes and verify end-to-end

This task gathers any uncommitted playground changes (playground mode, seed IP fix, Dockerfile fixes) and does a final build verification.

**Files:** (all previously modified in this branch)

- [ ] **Step 1: Check git status for any unstaged playground changes**

```bash
git status
git log --oneline -15
```

Confirm all 6 preceding tasks are committed and there are no unstaged changes.

- [ ] **Step 2: Build the Docker image**

```bash
docker build \
  --build-arg BUILD_TAGS=pro \
  -t lattice-k3s:playground-test \
  -f deploy/k3s/Dockerfile .
```

Expected: build succeeds, no errors.

- [ ] **Step 3: Run the playground container**

```bash
docker run -d --rm \
  --name lattice-playground-test \
  --privileged \
  --cgroupns=host \
  -p 8080:8080 \
  -e LATTICE_ADMIN_USER=admin \
  -e LATTICE_ADMIN_PASS=123456 \
  lattice-k3s:playground-test
```

- [ ] **Step 4: Wait for startup (about 60 seconds) and check logs**

```bash
docker logs -f lattice-playground-test 2>&1 | grep -E "ready|error|WARNING" | head -20
```

Expected: see `latticed HTTP API ready` and `playground mode: all Pro features unlocked`.

- [ ] **Step 5: Verify `/install.sh` is served**

```bash
curl -s http://localhost:8080/install.sh | head -3
```

Expected:
```
#!/usr/bin/env sh
# Lattice Agent Quick Install Script
# Usage: curl -sSL https://get.lattice.io | sh -s -- --server <SERVER_URL> --token <TOKEN> --name <NODE_NAME>
```

- [ ] **Step 6: Verify the nodes page shows the enrollment banner**

Open `http://localhost:8080` in a browser, log in as `admin/123456`, navigate to a workspace → Nodes page.

Confirm:
1. Seed nodes appear in the list (8 nodes with IPs like 10.0.0.x)
2. The blue "连接你的第一台真实节点" banner appears at the top
3. Entering `test-node` and clicking "生成安装命令" returns a command containing the enrollment token and `http://localhost:8080` as the server URL

- [ ] **Step 7: Stop test container**

```bash
docker stop lattice-playground-test
```
