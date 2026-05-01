# Image URL to use all building/pushing image targets
.DEFAULT_GOAL := help

# 获取版本信息
LATTICE_VERSION ?= $(shell git describe --tags --always --dirty)
GIT_COMMIT = $(shell git rev-parse HEAD)
BUILD_TIME = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GO_VERSION = $(shell go version | cut -d ' ' -f 3)

# 注入路径（对应 pkg/version 里的变量名）
LDFLAGS = -X 'lattice/pkg/version.Version=$(LATTICE_VERSION)' \
          -X 'lattice/pkg/version.GitCommit=$(GIT_COMMIT)' \
          -X 'lattice/pkg/version.BuildTime=$(BUILD_TIME)' \
          -X 'lattice/pkg/version.GoVersion=$(GO_VERSION)'


REGISTRY ?= ghcr.io/alatticeio
# manager: K8s operator; lattice: edge agent; latticed: all-in-one control plane
SERVICES := manager lattice latticed lrp
TARGETOS ?= linux
TARGETARCH ?=amd64
VERSION ?= dev
TAG ?= dev
IMG ?= ghcr.io/alatticeio/manager:$(VERSION)
LATTICED_IMG ?= $(REGISTRY)/latticed:$(TAG)

# 默认环境设置为 dev
ENV ?= dev
# 定义 overlays 的根目录
OVERLAYS_PATH = config/lattice/overlays/$(ENV)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Optional Docker BuildKit cache flags. Set in CI:
#   BUILD_CACHE_ARGS="--cache-from type=gha --cache-to type=gha,mode=max"
BUILD_CACHE_ARGS ?=

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# ── Edition ───────────────────────────────────────────────────────────────────
# EDITION controls which features are compiled in.
#   community  — default, open-source build (no pro build tag)
#   pro        — includes pro features: TURN server, Dex OIDC, telemetry push
#
# Usage:
#   make build SERVICE=lattice                  # community (default)
#   make build SERVICE=lattice EDITION=pro      # pro
#   make docker-build SERVICE=latticed EDITION=pro
EDITION ?= community

ifeq ($(EDITION),pro)
BUILD_TAGS := -tags pro
else
BUILD_TAGS :=
endif
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: build-all
build-all: ## 构建所有服务
	@echo " Building all services..."
	@for service in $(SERVICES); do \
		$(MAKE) build SERVICE=$$service; \
	done

.PHONY: build
build: ebpf-gen ## 构建单个服务 (使用: make build SERVICE=lattice)
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Error: SERVICE is required. Usage: make build SERVICE=lattice"; \
		exit 1; \
	fi
	@if [ "$(SERVICE)" = "manager" ] || [ "$(SERVICE)" = "latticed" ]; then \
		echo "📦 Service is $(SERVICE), building UI first..."; \
		$(MAKE) build-ui; \
	fi
	@echo " Building $(SERVICE) [edition=$(EDITION)]..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) \
		go build \
		$(BUILD_TAGS) \
		-ldflags="-s -w $(LDFLAGS)" \
		-o bin/$(SERVICE) \
		./cmd/$(SERVICE)/main.go
	@echo "✅ Built: bin/$(SERVICE)"
	@ls -lh bin/$(SERVICE)

# eBPF code generation (bpf2go)
.PHONY: ebpf-gen
ebpf-gen:
# macOS needs Homebrew LLVM; Linux uses system clang
ifeq ($(shell uname -s),Darwin)
	BPF2GO_CC=/opt/homebrew/opt/llvm/bin/clang go generate ./internal/agent/ebpf/
else
	go generate ./internal/agent/ebpf/
endif

.PHONY: test-latticed
test-latticed: ## 运行 latticed 相关单元测试
	go test ./cmd/latticed/... ./internal/nats/... ./internal/db/... -v -count=1

# ============ Docker 构建 ============
##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	protoc --proto_path=internal/proto \
		--go_out=internal/grpc \
		--go_opt=paths=source_relative \
		--go-grpc_out=internal/grpc \
		--go-grpc_opt=paths=source_relative drp.proto signal.proto management.proto

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out


# ============ E2E 配置 ============
MANAGE_URL          ?= http://localhost:8080
MANAGE_NS           ?= lattice-system
MANAGE_SVC          ?= lattice-api-service
NATS_SVC            ?= lattice-nats-service
MANAGE_PORT         ?= 8080
NATS_PORT           ?= 8222
LOCAL_AGENT_IMAGE   ?= $(REGISTRY)/lattice:$(TAG)
LOCAL_MANAGER_IMAGE ?= $(REGISTRY)/manager:$(TAG)
LOCAL_CLUSTER_NAME  ?= lattice-e2e
# 独立 kubeconfig，避免与 ~/.kube/config 中其他集群的 context 冲突
# CI 中通过 `make ... E2E_KUBECONFIG=/tmp/lattice-e2e.kubeconfig` 传入
E2E_KUBECONFIG      ?= /tmp/$(LOCAL_CLUSTER_NAME).kubeconfig
# kubectl 封装：所有 e2e 相关命令使用隔离的 kubeconfig
E2E_KUBECTL         := kubectl --kubeconfig=$(E2E_KUBECONFIG)

.PHONY: e2e-setup
e2e-setup: kustomize ## 一键搭建本地 E2E 环境 (k3d 集群 + 构建/导入镜像 + 部署 Manager)
	@echo "====> [1/5] 创建 k3d 集群 ($(LOCAL_CLUSTER_NAME))..."
	k3d cluster list 2>/dev/null | grep -q "$(LOCAL_CLUSTER_NAME)" || \
		k3d cluster create $(LOCAL_CLUSTER_NAME) \
			--agents 1 --no-lb \
			--k3s-arg "--disable=traefik@server:*"
	@echo "====> [2/5] 导出 kubeconfig → $(E2E_KUBECONFIG)"
	k3d kubeconfig get $(LOCAL_CLUSTER_NAME) > $(E2E_KUBECONFIG)
	@echo "====> [3/5] 构建镜像 manager & lattice ..."
	$(MAKE) docker-build SERVICE=manager  TAG=$(TAG)
	$(MAKE) docker-build SERVICE=lattice TAG=$(TAG)
	@echo "====> [4/5] 导入镜像到 k3d ..."
	k3d image import $(LOCAL_MANAGER_IMAGE) -c $(LOCAL_CLUSTER_NAME)
	k3d image import $(LOCAL_AGENT_IMAGE)   -c $(LOCAL_CLUSTER_NAME)
	@echo "====> [5/5] 部署 Lattice Manager (ENV=dev) ..."
	$(MAKE) deploy ENV=dev IMG=$(LOCAL_MANAGER_IMAGE) KUBECTL="$(E2E_KUBECTL)"
	$(E2E_KUBECTL) rollout status deployment -n $(MANAGE_NS) --timeout=120s
	@echo "✅ E2E 环境就绪。kubeconfig: $(E2E_KUBECONFIG)"
	@echo "   运行测试: make test-e2e"
	@echo "   销毁集群: make e2e-teardown"

.PHONY: test-e2e
test-e2e: ## 运行 E2E 集成测试（自动 port-forward，测试结束后停止）
	@echo "====> 启动 port-forward: $(MANAGE_NS)/svc/$(MANAGE_SVC) -> localhost:$(MANAGE_PORT)"; \
	$(E2E_KUBECTL) port-forward -n $(MANAGE_NS) svc/$(MANAGE_SVC) $(MANAGE_PORT):$(MANAGE_PORT) & \
	PF_PID=$$!; \
	for i in $$(seq 1 15); do \
		nc -z localhost $(MANAGE_PORT) 2>/dev/null && break; \
		echo "  等待 port-forward 就绪... ($$i/15)"; \
		sleep 2; \
	done; \
	echo "====> 运行 E2E 测试"; \
	go test ./test/e2e/... -v -timeout 15m -args \
		--agent-image=$(LOCAL_AGENT_IMAGE) \
		--manage-url=$(MANAGE_URL) \
		--kubeconfig=$(E2E_KUBECONFIG); \
	TEST_EXIT=$$?; \
	echo "====> 停止 port-forward (PID: $$PF_PID)"; \
	kill $$PF_PID 2>/dev/null || true; \
	exit $$TEST_EXIT

.PHONY: e2e
e2e: e2e-setup test-e2e ## 一键运行完整 E2E（搭建环境 + 测试）；完成后集群保留，可用 make e2e-teardown 清理

.PHONY: e2e-teardown
e2e-teardown: ## 销毁 E2E 测试用的 k3d 集群并清理 kubeconfig
	k3d cluster delete $(LOCAL_CLUSTER_NAME) || true
	rm -f $(E2E_KUBECONFIG)
	@echo "✅ E2E 集群已销毁"

.PHONY: test-e2e-load
test-e2e-load: ## 仅构建并导入 lattice agent 镜像（适合镜像变更后快速重跑）
	$(MAKE) docker-build SERVICE=lattice TAG=$(TAG)
	k3d image import $(LOCAL_AGENT_IMAGE) -c $(LOCAL_CLUSTER_NAME)

.PHONY: test-e2e-cleanup
test-e2e-cleanup: ## 清理 E2E 残留的测试 Namespace (前缀 wf-test-)
	@echo "====> 清理测试 Namespace..."
	$(E2E_KUBECTL) get ns -o name | grep "namespace/wf-test-" | xargs -r $(E2E_KUBECTL) delete --ignore-not-found=true


.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	$(GOLANGCI_LINT) config verify

##@ Build

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# ============ Web / Full-stack ============
.PHONY: build-ui
build-ui: ## 打包前端 Vue3 产物（输出到 internal/web/dist，供 go:embed 使用）
	@echo ">>> Building UI..."
	cd fronted && pnpm install && pnpm build
	touch internal/web/dist/.gitkeep
	@echo ">>> UI built → internal/web/dist"


# ============ Docker 构建 ============
.PHONY: docker-build-all
docker-build-all: ## 构建所有服务的 Docker 镜像
	@echo " Building all Docker images..."
	@for service in $(SERVICES); do \
		$(MAKE) docker-build SERVICE=$$service; \
	done

.PHONY: docker-build
docker-build: ## 构建单个服务的 Docker 镜像 (使用: make docker-build SERVICE=lattice)
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Error: SERVICE is required. Usage: make docker-build SERVICE=lattice"; \
		exit 1; \
	fi
	@echo " Building Docker image for $(SERVICE)..."
	@# 如果构建的是 manager 或 latticed，先执行 UI 构建
	@if [ "$(SERVICE)" = "manager" ] || [ "$(SERVICE)" = "latticed" ]; then \
		echo "📦 Service is $(SERVICE), building UI first..."; \
		$(MAKE) build-ui; \
	fi
	$(CONTAINER_TOOL) build \
		$(BUILD_CACHE_ARGS) \
		--build-arg TARGETSERVICE=$(SERVICE) \
		--build-arg TARGETOS=$(TARGETOS) \
		--build-arg TARGETARCH=$(TARGETARCH) \
		--build-arg VERSION=$(TAG) \
		--build-arg BUILD_DATE=$(BUILD_TIME) \
		--build-arg BUILD_TAGS=$(if $(filter pro,$(EDITION)),pro,) \
		-t $(REGISTRY)/$(SERVICE):$(TAG) \
		-f Dockerfile \
		.
	@echo "✅ Built image: $(REGISTRY)/$(SERVICE):$(TAG) [edition=$(EDITION)]"

# ============ Docker 推送 ============
.PHONY: docker-push-all
docker-push-all: ## 推送所有服务的 Docker 镜像
	@echo " Pushing all Docker images..."
	@for service in $(SERVICES); do \
		$(MAKE) docker-push SERVICE=$$service; \
	done

.PHONY: docker-push
docker-push: ## 推送单个服务的 Docker 镜像
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Error: SERVICE is required"; \
		exit 1; \
	fi
	@echo " Pushing $(REGISTRY)/$(SERVICE):$(TAG)..."
	$(CONTAINER_TOOL) push $(REGISTRY)/$(SERVICE):$(TAG)
	@echo "✅ Pushed: $(REGISTRY)/$(SERVICE):$(TAG)"

# ============ Docker 构建并推送 ============
.PHONY: docker-all
docker-all: docker-build-all docker-push-all ## 构建并推送所有镜像

.PHONY: docker
docker: docker-build docker-push ## 构建并推送单个镜像

INSTALLER_IMG ?= lattice/installer
INSTALLER_PLATFORMS ?= linux/amd64,linux/arm64

.PHONY: docker-installer
docker-installer: ## 构建 installer 镜像 (本地 load，仅当前平台)
	$(CONTAINER_TOOL) build \
		-t $(INSTALLER_IMG):latest \
		-f deploy/installer/Dockerfile \
		deploy/installer/
	@echo "✅ Built: $(INSTALLER_IMG):latest"

.PHONY: docker-installer-push
docker-installer-push: ## 构建并推送多架构 installer 镜像 (buildx)
	$(CONTAINER_TOOL) buildx build \
		--platform $(INSTALLER_PLATFORMS) \
		-t $(INSTALLER_IMG):latest \
		--push \
		-f deploy/installer/Dockerfile \
		deploy/installer/
	@echo "✅ Pushed: $(INSTALLER_IMG):latest ($(INSTALLER_PLATFORMS))"

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name lattice-controller-builder
	$(CONTAINER_TOOL) buildx use lattice-controller-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm lattice-controller-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Build kustomize manifests into deploy/quickstart/ for installer to fetch via GITHUB_RAW.
	mkdir -p deploy/quickstart
	$(KUSTOMIZE) build config/crd > deploy/quickstart/lattice-crds.yaml
	$(KUSTOMIZE) build config/lattice/overlays/all-in-one > deploy/quickstart/lattice-all-in-one.yaml
	@echo "✅ Manifests written to deploy/quickstart/"

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy-aio
deploy-aio: manifests kustomize ## 部署 all-in-one 模式到已有 K8s 集群 (usage: make deploy-aio IMG=ghcr.io/alatticeio/latticed:v0.1.0)
	$(KUBECTL) create namespace lattice-system --dry-run=client -o yaml | $(KUBECTL) apply -f -
	@echo "正在安装 CRDs..."
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -
	@echo "等待 CRDs 就绪..."
	$(KUBECTL) wait --for=condition=Established crd --all --timeout=60s
	@echo "正在部署 all-in-one latticed (IMG=$(IMG))..."
	$(KUSTOMIZE) build config/lattice/overlays/all-in-one | \
		sed "s|ghcr.io/alatticeio/latticed:.*|$(IMG)|g" | \
		$(KUBECTL) apply -f -
	$(KUBECTL) rollout status deployment/lattice-aio -n lattice-system --timeout=180s
	@echo "✅ All-in-one 部署完成。API: $(KUBECTL) port-forward svc/lattice-api-service 8080:8080 -n lattice-system"

.PHONY: undeploy-aio
undeploy-aio: kustomize ## 卸载 all-in-one 部署
	$(KUSTOMIZE) build config/lattice/overlays/all-in-one | $(KUBECTL) delete -f - --ignore-not-found=true

.PHONY: deploy
deploy: manifests kustomize ## 根据 ENV 部署 (usage: make deploy ENV=production)
	$(KUBECTL) create namespace lattice-system --dry-run=client -o yaml | $(KUBECTL) apply -f -
	@echo "正在部署到环境: $(ENV)..."
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -
	@echo "等待 CRDs 就绪..."
	$(KUBECTL) wait --for=condition=Established crd --all --timeout=60s
	$(KUSTOMIZE) build $(OVERLAYS_PATH) | \
		sed "s|ghcr.io/alatticeio/manager:.*|$(IMG)|g" | \
		$(KUBECTL) apply -f -

.PHONY: yaml
yaml:
	$(KUSTOMIZE) build config/lattice/overlays/all-in-one > config/lattice.yaml

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build $(OVERLAYS_PATH) | $(KUBECTL) delete -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v5.6.0
CONTROLLER_TOOLS_VERSION ?= v0.18.0
#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')
GOLANGCI_LINT_VERSION ?= v1.64.5

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))


# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef
