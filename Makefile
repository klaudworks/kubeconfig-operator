# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.30
IMG ?= kubeconfig-operator:latest

# go-get-tool will 'go get' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
echo "Downloading $(2)" ;\
GOBIN=$(LOCALBIN) go install -modfile=tools/go.mod $(2) ;\
}
endef

.PHONY: lint
lint: gosimports
	cd tools && go mod tidy
	go mod tidy
	go fmt ./...
	go list -buildvcs=false ./... | xargs go vet
	$(GOSIMPORTS) -local github.com/klaudworks/kubeconfig-operator -l -w .

# Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
.PHONY: generate
generate: controller-gen kustomize
	$(CONTROLLER_GEN) object rbac:roleName=kubeconfig-operator-role crd webhook paths="./..." \
		output:crd:artifacts:config=manifests/crd/bases \
		output:webhook:artifacts:config=manifests/webhook/ \
		output:rbac:artifacts:config=manifests/base/rbac
	# generate root kustomization.yaml for crds
	cd manifests/crd/bases && rm -f kustomization.yaml && $(KUSTOMIZE) create --autodetect --recursive
	# add all rbac resources to kustomize
	cd manifests/base/rbac && rm -f kustomization.yaml && $(KUSTOMIZE) create --autodetect
	# generate complete kubeconfig-operator manifest
	cd manifests/base && $(KUSTOMIZE) build > manager.yaml

.PHONY: test
test: generate lint envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./...

# If you wish built the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64 ). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker
docker:
	docker build -t ${IMG} .

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOSIMPORTS ?= $(LOCALBIN)/gosimports

## Tool Versions
KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest)

.PHONY: gosimports
gosimports: $(GOSIMPORTS)
$(GOSIMPORTS): $(LOCALBIN)
	$(call go-get-tool,$(GOSIMPORTS),github.com/rinchsan/gosimports/cmd/gosimports)
