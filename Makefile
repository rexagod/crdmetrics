# Variables are declared in the order in which they occur.
ARCH ?= $(shell $(GO) env GOARCH)
ASSETS_DIR ?= assets/
BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
BUILD_TAG ?= $(shell git describe --tags --exact-match 2>/dev/null || echo "latest")
COMMON = github.com/prometheus/common
CONTROLLER_GEN ?= $(shell go env GOPATH)/bin/controller-gen
CONTROLLER_GEN_APIS_DIR ?= pkg/apis
CONTROLLER_GEN_OUT_DIR ?= /tmp/crsm/controller-gen
CONTROLLER_GEN_VERSION ?= v0.15.0
LOCAL_NAMESPACE ?= default
GIT_COMMIT = $(shell git rev-parse --short HEAD)
GO ?= go
GOLANGCI_LINT ?= $(shell go env GOPATH)/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.54.2
GO_FILES = $(shell find . -type f -name "*.go")
MARKDOWNFMT ?= $(shell go env GOPATH)/bin/markdownfmt
MARKDOWNFMT_VERSION ?= v3.1.0
MD_FILES = $(shell find . \( -type d -name 'vendor' -o -type d -name $(patsubst %/,%,$(patsubst ./%,%,$(ASSETS_DIR))) \) -prune -o -type f -name "*.md" -print)
OS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
PROJECT_NAME = crsm
RUNNER = $(shell id -u -n)@$(shell hostname)
V ?= 4
VALE ?= $(ASSETS_DIR)vale
VALE_ARCH ?= $(if $(filter $(shell uname -m),arm64),macOS_arm64,Linux_64-bit)
VALE_VERSION ?= 3.1.0
VERSION = $(shell cat VERSION)

all: lint $(PROJECT_NAME)

#########
# Setup #
#########

.PHONY: setup-dependencies
setup-dependencies:
	# Setup vale.
	@wget https://github.com/errata-ai/vale/releases/download/v$(VALE_VERSION)/vale_$(VALE_VERSION)_$(VALE_ARCH).tar.gz && \
	mkdir -p assets && tar -xvzf vale_$(VALE_VERSION)_$(VALE_ARCH).tar.gz -C $(ASSETS_DIR) && \
	rm vale_$(VALE_VERSION)_$(VALE_ARCH).tar.gz && \
	chmod +x $(VALE)
	# Setup markdownfmt.
	@GOOS=$(OS) GOARCH=$(ARCH) $(GO) install github.com/Kunde21/markdownfmt/v3/cmd/markdownfmt@$(MARKDOWNFMT_VERSION)
	# Setup golangci-lint.
	@GOOS=$(OS) GOARCH=$(ARCH) $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	# Setup controller-gen.
	@GOOS=$(OS) GOARCH=$(ARCH) $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

##############
# Generating #
##############

.PHONY: manifests
manifests:
	@# Populate manifests/.
	@$(CONTROLLER_GEN) rbac:roleName=$(PROJECT_NAME) crd paths=./$(CONTROLLER_GEN_APIS_DIR)/... output:crd:dir=$(CONTROLLER_GEN_OUT_DIR) output:rbac:artifacts:config=$(CONTROLLER_GEN_OUT_DIR) && \
	mv "$(CONTROLLER_GEN_OUT_DIR)/crsm.instrumentation.k8s-sigs.io_customresourcestatemetricsresources.yaml" "manifests/custom-resource-definition.yaml" && \
	mv "$(CONTROLLER_GEN_OUT_DIR)/role.yaml" "manifests/cluster-role.yaml"

.PHONY: codegen
codegen:
	@# Populate pkg/generated/.
	@./hack/update-codegen.sh

.PHONY: generate
generate: codegen manifests

############
# Building #
############

.PHONY: image
image: $(PROJECT_NAME)
	@docker build -t $(PROJECT_NAME):$(BUILD_TAG) .

$(PROJECT_NAME): $(GO_FILES)
	@GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -a -installsuffix cgo -ldflags "-s -w \
	-X ${COMMON}/version.Version=v${VERSION} \
	-X ${COMMON}/version.Revision=${GIT_COMMIT} \
	-X ${COMMON}/version.Branch=${BRANCH} \
	-X ${COMMON}/version.BuildUser=${RUNNER} \
	-X ${COMMON}/version.BuildDate=${BUILD_DATE}" \
	-o $@

.PHONY: build
build: $(PROJECT_NAME)

###########
# Running #
###########

.PHONY: load
load: image
	@kind load docker-image $(PROJECT_NAME):$(BUILD_TAG)

.PHONY: apply
apply: manifests
	# Deleting manifests/
	@kubectl delete -f manifests/ || true
	# Applying manifests/
	@kubectl apply -f manifests/custom-resource-definition.yaml && \
	kubectl apply -f manifests/

.PHONY: apply-testdata
apply-testdata:
	# Deleting testdata/
	@kubectl delete -Rf testdata || true
	# Applying testdata/
	@kubectl apply -f testdata/custom-resource-definition/ && \
	kubectl apply -f testdata/custom-resource/

.PHONY: local
local: vet manifests codegen $(PROJECT_NAME)
	@kubectl scale deployment $(PROJECT_NAME)-controller --replicas=0 -n $(LOCAL_NAMESPACE) 2>/dev/null || true
	@./$(PROJECT_NAME) -v=$(V) -kubeconfig $(KUBECONFIG)

###########
# Testing #
###########

.PHONY: test-unit
test-unit:
	@GOOS=$(OS) GOARCH=$(ARCH) $(GO) test -v -race $(shell $(GO) list ./... | grep -v $(E2E_TEST_PKG))

.PHONY: test
test: test-unit

###########
# Linting #
###########

.PHONY: vet
vet:
	@$(GO) vet ./...

.PHONY: clean
clean:
	@git clean -fxd

vale: .vale.ini $(MD_FILES)
	@$(ASSETS_DIR)vale sync && \
	$(ASSETS_DIR)vale $(MD_FILES)

markdownfmt: $(MD_FILES)
	@test -z "$(shell $(MARKDOWNFMT) -l $(MD_FILES))" || (echo "\033[0;31mThe following files need to be formatted with 'markdownfmt -w -gofmt':" $(shell $(MARKDOWNFMT) -l $(MD_FILES)) "\033[0m" && exit 1)

markdownfmt-fix: $(MD_FILES)
	@for file in $(MD_FILES); do markdownfmt -w -gofmt $$file || exit 1; done

.PHONY: lint-md
lint-md: vale markdownfmt

.PHONY: lint-md-fix
lint-md-fix: vale markdownfmt-fix

gofmt: $(GO_FILES)
	@test -z "$(shell gofmt -l $(GO_FILES))" || (echo "\033[0;31mThe following files need to be formatted with 'gofmt -w':" $(shell gofmt -l $(GO_FILES)) "\033[0m" && exit 1)

gofmt-fix: $(GO_FILES)
	@gofmt -w . || exit 1

golangci-lint: $(GO_FILES)
	@$(GOLANGCI_LINT) run

.PHONY: lint-go
lint-go: gofmt golangci-lint

.PHONY: lint-go-fix
lint-go-fix: gofmt-fix golangci-lint

.PHONY: lint
lint: lint-md lint-go

.PHONY: lint-fix
lint-fix: lint-md-fix lint-go-fix
