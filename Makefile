# Image URL to use all building/pushing image targets

# 获取版本信息
WIREFLOW_VERSION ?= $(shell git describe --tags --always --dirty)
GIT_COMMIT = $(shell git rev-parse HEAD)
BUILD_TIME = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GO_VERSION = $(shell go version | cut -d ' ' -f 3)

# 注入路径（对应 pkg/version 里的变量名）
LDFLAGS = -X 'github.com/your-org/wireflow/pkg/version.Version=$(WIREFLOW_VERSION)' \
          -X 'github.com/your-org/wireflow/pkg/version.GitCommit=$(GIT_COMMIT)' \
          -X 'github.com/your-org/wireflow/pkg/version.BuildTime=$(BUILD_TIME)' \
          -X 'github.com/your-org/wireflow/pkg/version.GoVersion=$(GO_VERSION)'


REGISTRY ?= ghcr.io/wireflowio
SERVICES := manager wireflow
TARGETOS ?= linux
TARGETARCH ?=amd64
VERSION ?= dev
TAG ?= dev
IMG ?= ghcr.io/wireflowio/manager:$(VERSION)

# 默认环境设置为 dev
ENV ?= dev
# 定义 overlays 的根目录
OVERLAYS_PATH = config/wireflow/overlays/$(ENV)

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

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: build-all
build-all: ## 构建所有服务
	@echo " Building all services..."
	@for service in $(SERVICES); do \
		$(MAKE) build SERVICE=$$service; \
	done

.PHONY: build
build: fmt vet ## 构建单个服务 (使用: make build SERVICE=wireflow)
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Error: SERVICE is required. Usage: make build SERVICE=wireflow"; \
		exit 1; \
	fi
	@echo " Building $(SERVICE)..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) \
		go build \
		-ldflags="-s -w $(LDFLAGS)" \
		-o bin/$(SERVICE) \
		./cmd/$(SERVICE)/main.go
	@echo "✅ Built: bin/$(SERVICE)"
	@ls -lh bin/$(SERVICE)

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

# 变量定义
TEST_NS_A ?= wireflow-e2e-source
TEST_NS_B ?= wireflow-e2e-target

.PHONY: test-e2e
test-e2e: ## 运行跨 Namespace 连通性集成测试
	@echo "====> Prepare test env..."
	kubectl create namespace $(TEST_NS_A) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace $(TEST_NS_B) --dry-run=client -o yaml | kubectl apply -f -
	@echo "====> Deploy tests resources (Token/Peers)..."
	# 这里可以根据你的需求，用 sed 替换模板生成 Token 资源
	# kubectl apply -f config/samples/test_token.yaml -n $(TEST_NS_B)
	@echo "====> tests connectivity..."
	chmod +x ./scripts/test-connectivity.sh
	./scripts/test-connectivity.sh $(TEST_NS_A) $(TEST_NS_B)
	@$(MAKE) test-e2e-cleanup

.PHONY: test-e2e-cleanup
test-e2e-cleanup: ## 清理测试残留
	@echo "====> 清理测试 Namespace..."
	kubectl delete namespace $(TEST_NS_A) $(TEST_NS_B) --ignore-not-found=true


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





# ============ Docker 构建 ============
.PHONY: docker-build-all
docker-build-all: ## 构建所有服务的 Docker 镜像
	@echo " Building all Docker images..."
	@for service in $(SERVICES); do \
		$(MAKE) docker-build SERVICE=$$service; \
	done

.PHONY: docker-build
docker-build: ## 构建单个服务的 Docker 镜像 (使用: make docker-build SERVICE=wireflow)
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Error: SERVICE is required. Usage: make docker-build SERVICE=wireflow"; \
		exit 1; \
	fi
	@echo " Building Docker image for $(SERVICE)..."
	$(CONTAINER_TOOL) build \
		--build-arg TARGETSERVICE=$(SERVICE) \
		--build-arg TARGETOS=$(TARGETOS) \
		--build-arg TARGETARCH=$(TARGETARCH) \
		--build-arg VERSION=$(TAG) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(REGISTRY)/$(SERVICE):$(TAG) \
		-f Dockerfile \
		.
	@echo "✅ Built image: $(REGISTRY)/$(SERVICE):$(TAG)"

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
	- $(CONTAINER_TOOL) buildx create --name wireflow-controller-builder
	$(CONTAINER_TOOL) buildx use wireflow-controller-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm wireflow-controller-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

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

.PHONY: deploy
deploy: manifests kustomize ## 根据 ENV 部署 (usage: make deploy ENV=production)
	# 1. 强制创建 Namespace (如果已存在则忽略错误)
	$(KUBECTL) create namespace wireflow-system --dry-run=client -o yaml | $(KUBECTL) apply -f -

	@echo "正在部署到环境: $(ENV)..."
	# 1. 动态修改对应环境的镜像标签
	cd $(OVERLAYS_PATH) && $(KUSTOMIZE) edit set image manager=${IMG}

	# 2. 部署 CRD (通常 CRD 是全局的，可以继续用 base)
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -
	@echo "等待5秒，让CRD完成初始化..."
	@sleep 5

	# 3. 部署指定环境的完整资源
	$(KUSTOMIZE) build $(OVERLAYS_PATH) | $(KUBECTL) apply -f -

	# 3. 立即还原该文件（文件变干净）
	git checkout config/wireflow/overlays/$(ENV)/kustomization.yaml

.PHONY: Yaml
yaml:
	$(KUSTOMIZE) build config/default > config/wireflow.yaml

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
GOLANGCI_LINT_VERSION ?= v2.1.0

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
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))


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
