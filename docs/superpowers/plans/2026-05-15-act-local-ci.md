# act Local CI Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Drone CI with act (nektos/act) so one GitHub Actions workflow definition works both locally and on GitHub, eliminating `.drone.yml` and all Drone-specific code paths.

**Architecture:** `lib.sh` is stripped of Drone branches to become a two-mode script (github / local). `lint.yml` switches from a shell script call to `golangci-lint-action` for native caching. A new `unit-test.yml` fills the gap in GitHub Actions. `.actrc` maps `ubuntu-latest` to host mode so act runs directly on the developer's machine.

**Tech Stack:** GitHub Actions, nektos/act, golangci-lint-action@v6, bash

---

### Task 1: Simplify lib.sh — remove Drone branches

**Files:**
- Modify: `ci/scripts/lib.sh`

- [ ] **Step 1: Replace detect_ci**

In `ci/scripts/lib.sh`, replace the entire `detect_ci()` function:

```bash
detect_ci() {
  if [ -n "${GITHUB_ACTIONS:-}" ]; then
    echo "github"
  else
    echo "local"
  fi
}
```

- [ ] **Step 2: Replace detect_registry**

Replace the entire `detect_registry()` function (removes `DRONE_REPO_OWNER` branch):

```bash
detect_registry() {
  local owner
  if [ -n "${GITHUB_REPOSITORY_OWNER:-}" ]; then
    owner=$(echo "$GITHUB_REPOSITORY_OWNER" | tr '[:upper:]' '[:lower:]')
  else
    owner=$(git remote get-url origin 2>/dev/null | sed -E 's|.*[:/]([^/]+)/[^/]+(\\.git)?$|\\1|' | tr '[:upper:]' '[:lower:]')
    if [ -z "$owner" ]; then
      echo "ERROR: could not detect registry owner" >&2
      exit 1
    fi
  fi
  echo "ghcr.io/${owner}"
}
```

- [ ] **Step 3: Replace detect_tag**

Replace the entire `detect_tag()` function (removes `drone)` case):

```bash
detect_tag() {
  if [ -n "${TAG:-}" ]; then
    echo "$TAG"
    return
  fi

  local ci
  ci=$(detect_ci)
  case "$ci" in
    github)
      if [ "${GITHUB_EVENT_NAME:-}" = "pull_request" ] || [ "${GITHUB_EVENT_NAME:-}" = "pull_request_target" ]; then
        echo "pr-${PR_NUMBER:-unknown}"
      elif [ "${GITHUB_EVENT_NAME:-}" = "push" ]; then
        if [ "${GITHUB_REF:-}" = "refs/heads/master" ] || [ "${GITHUB_REF:-}" = "refs/heads/main" ]; then
          echo "latest"
        else
          echo "${GITHUB_REF_NAME:-dev}"
        fi
      else
        echo "dev"
      fi
      ;;
    local) echo "${TAG:-dev}" ;;
  esac
}
```

- [ ] **Step 4: Replace detect_is_release**

Replace the entire `detect_is_release()` function (removes `drone)` case):

```bash
detect_is_release() {
  if [ "${IS_RELEASE:-}" = "true" ]; then
    echo "true"
    return
  fi
  local ci
  ci=$(detect_ci)
  case "$ci" in
    github)
      if [ "${GITHUB_EVENT_NAME:-}" = "push" ] && echo "${GITHUB_REF:-}" | grep -q '^refs/tags/v'; then
        echo "true"
        return
      fi
      ;;
    local)
      if git describe --exact-match --tags 2>/dev/null | grep -q '^v'; then
        echo "true"
        return
      fi
      ;;
  esac
  echo "false"
}
```

- [ ] **Step 5: Replace detect_is_push_master**

Replace the entire `detect_is_push_master()` function (removes `drone)` case):

```bash
detect_is_push_master() {
  local ci
  ci=$(detect_ci)
  case "$ci" in
    github)
      if [ "${GITHUB_EVENT_NAME:-}" = "push" ]; then
        case "${GITHUB_REF:-}" in
          refs/heads/master|refs/heads/main) echo "true"; return ;;
        esac
      fi
      ;;
    local)
      local branch
      branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
      case "$branch" in
        master|main) echo "true"; return ;;
      esac
      ;;
  esac
  echo "false"
}
```

- [ ] **Step 6: Replace detect_is_pr**

Replace the entire `detect_is_pr()` function (removes `drone)` case):

```bash
detect_is_pr() {
  local ci
  ci=$(detect_ci)
  case "$ci" in
    github)
      if [ "${GITHUB_EVENT_NAME:-}" = "pull_request" ] || [ "${GITHUB_EVENT_NAME:-}" = "pull_request_target" ]; then
        echo "true"
        return
      fi
      ;;
  esac
  echo "false"
}
```

- [ ] **Step 7: Replace docker_login_ghcr**

Replace the entire `docker_login_ghcr()` function (removes Drone secret variable fallbacks):

```bash
docker_login_ghcr() {
  local registry_host="ghcr.io"
  local username="${GITHUB_ACTOR:-}"
  local password="${GITHUB_TOKEN:-}"

  if [ -z "$username" ] || [ -z "$password" ]; then
    log_error "Missing GHCR credentials. Set GITHUB_TOKEN and GITHUB_ACTOR."
    exit 1
  fi
  echo "$password" | docker login "$registry_host" -u "$username" --password-stdin
  log_info "Logged in to $registry_host as $username"
}
```

- [ ] **Step 8: Verify syntax**

```bash
bash -n ci/scripts/lib.sh
```

Expected: no output, exit code 0.

- [ ] **Step 9: Smoke-test locally**

```bash
source ci/scripts/lib.sh
detect_ci          # should print: local
detect_registry    # should print: ghcr.io/<your-github-username>
detect_tag         # should print: dev
detect_is_pro      # should print: false
```

- [ ] **Step 10: Commit**

```bash
git add ci/scripts/lib.sh
git commit -s -m "ci: remove Drone branches from lib.sh"
```

---

### Task 2: Update lint.yml to use golangci-lint-action

**Files:**
- Modify: `.github/workflows/lint.yml`

- [ ] **Step 1: Overwrite lint.yml**

Replace the entire contents of `.github/workflows/lint.yml` with:

```yaml
name: lint
on: [pull_request, push]

jobs:
  golangci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - uses: golangci/golangci-lint-action@v6
        with:
          version: v1.64.5
          args: --timeout 5m
```

- [ ] **Step 2: Verify YAML syntax**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/lint.yml'))" && echo "OK"
```

Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/lint.yml
git commit -s -m "ci: replace run-lint.sh with golangci-lint-action for native caching"
```

---

### Task 3: Add unit-test.yml

**Files:**
- Create: `.github/workflows/unit-test.yml`

- [ ] **Step 1: Create unit-test.yml**

Create `.github/workflows/unit-test.yml`:

```yaml
name: Unit Test

on: [pull_request, push]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Run unit tests
        run: bash ci/scripts/run-unit-tests.sh
```

- [ ] **Step 2: Verify YAML syntax**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/unit-test.yml'))" && echo "OK"
```

Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/unit-test.yml
git commit -s -m "ci: add unit-test workflow to GitHub Actions"
```

---

### Task 4: Add act configuration

**Files:**
- Create: `.actrc`
- Modify: `.gitignore`

- [ ] **Step 1: Create .actrc**

Create `.actrc` in the project root:

```
-P ubuntu-latest=-self-hosted
--secret-file .secrets.act
```

- [ ] **Step 2: Add .secrets.act to .gitignore**

Append to `.gitignore`:

```
# act local secrets (never commit)
.secrets.act
```

- [ ] **Step 3: Create .secrets.act template as documentation**

Create `.secrets.act.example` (committed, safe to share):

```
# Copy to .secrets.act and fill in values
# Never commit .secrets.act
GITHUB_TOKEN=your_github_personal_access_token
GITHUB_ACTOR=your_github_username
IS_PRO=false
```

- [ ] **Step 4: Install act (macOS)**

```bash
brew install act
act --version
```

Expected output: `act version 0.x.x` (any version ≥ 0.2.60)

- [ ] **Step 5: Commit**

```bash
git add .actrc .secrets.act.example .gitignore
git commit -s -m "ci: add act configuration for local workflow testing"
```

---

### Task 5: Delete .drone.yml

**Files:**
- Delete: `.drone.yml`

- [ ] **Step 1: Delete .drone.yml**

```bash
git rm .drone.yml
```

- [ ] **Step 2: Commit**

```bash
git commit -s -m "ci: remove Drone CI — replaced by act for local testing"
```

---

### Task 6: Validate with act

- [ ] **Step 1: Create .secrets.act from example**

```bash
cp .secrets.act.example .secrets.act
# Edit .secrets.act and fill in your real GITHUB_TOKEN and GITHUB_ACTOR
```

- [ ] **Step 2: Run lint workflow locally**

```bash
act push -W .github/workflows/lint.yml
```

Expected: workflow runs, golangci-lint passes, no "Downloading" step visible on second run.

- [ ] **Step 3: Run unit-test workflow locally**

```bash
act push -W .github/workflows/unit-test.yml
```

Expected: `go test ./internal/... ./pkg/...` runs and passes.

- [ ] **Step 4: Verify lint caching works on second run**

```bash
act push -W .github/workflows/lint.yml
```

Expected: golangci-lint-action reuses cached binary, step completes noticeably faster than first run.

- [ ] **Step 5: Push to GitHub and confirm both CI systems pass**

```bash
git push origin dev
```

Expected: GitHub Actions shows `lint` and `Unit Test` checks both green. Drone is gone.
