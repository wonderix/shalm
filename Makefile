
# Image URL to use all building/pushing image targets
OS := $(shell uname )
VERSION := $(shell git describe --tags --always --dirty)
REPOSITORY := wonderix/shalm
IMG ?= ${REPOSITORY}:${VERSION}
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd"

KUBERNETES_VERSION := v0.18.13

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GO_FILES:=$(shell git ls-files '*.go')

all: bin/shalm manager 

# Run tests
test: generate fmt vet 
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet 
	go run ./main.go

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: 
	go run ./main.go apply charts/shalm


# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./..."

test-e2e : test-watch test-certificate test-controller 

test-controller: docker-build bin/shalm
#	bin/shalm apply charts/shalm --set local=True
#	bin/shalm apply --proxy local charts/example/simple/hello
#	while [ "$$(kubectl get shalmchart hello -o 'jsonpath={.status.lastOp.progress}')" != "100" ] ; do sleep 5 ; done
#	kubectl get secret secret
#	bin/shalm delete --proxy local charts/example/simple/hello
#	bin/shalm delete charts/shalm --set local=True

test-watch: bin/shalm 
	bin/shalm apply charts/test/watch
	bin/shalm delete charts/test/watch

test-certificate: bin/shalm 
	bin/shalm apply charts/test/certificate
	bin/shalm apply charts/test/certificate
	bin/shalm delete charts/test/certificate

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	cp config/crd/bases/wonderix.github.com_shalmcharts.yaml charts/shalm/templates/crd.yaml

docker-context/kubectl: Makefile
	curl -SsL https://storage.googleapis.com/kubernetes-release/release/$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl -o docker-context/kubectl
	chmod +x docker-context/kubectl

docker-context/kapp:  Makefile
	curl -SsL https://github.com/k14s/kapp/releases/download/v0.34.0/kapp-linux-amd64 -o docker-context/kapp
	chmod +x docker-context/kapp


docker-prepare:: docker-context/shalm docker-context/kubectl docker-context/kapp

docker-build:: docker-context/build

# Build the docker image
docker-context/build:  docker-context/shalm docker-context/kubectl docker-context/kapp
	docker build docker-context -f Dockerfile -t ${IMG}
	docker tag ${IMG} ${REPOSITORY}:latest
	touch docker-context/build

# Push the docker image
docker-push: docker-context/build
	docker push ${IMG}
	docker push ${REPOSITORY}:latest

chart:
	rm -rf /tmp/shalm
	cp -r charts/shalm /tmp/shalm
ifeq ($(OS),Darwin)
	sed -i '' -e 's|version:.*|version: ${VERSION}|g' /tmp/shalm/Chart.yaml
	sed -i '' -e 's|image: wonderix/shalm:.*|image: wonderix/shalm:${VERSION}|g' /tmp/shalm/templates/deployment.yaml
else
	sed -i -e 's|version:.*|version: ${VERSION}|g' /tmp/shalm/Chart.yaml
	sed -i -e 's|image: wonderix/shalm:.*|image: wonderix/shalm:${VERSION}|g' /tmp/shalm/templates/deployment.yaml
endif
	mkdir -p bin
	cd bin && go run .. package /tmp/shalm

shalm:: bin/shalm

VERSION_FLAGS := "-X github.com/wonderix/shalm/pkg/shalm.version=${VERSION} -X github.com/wonderix/shalm/pkg/shalm.kubeVersion=${KUBERNETES_VERSION}"

bin/shalm: $(GO_FILES)  go.sum go.mod Makefile
	CGO_ENABLED=0 GOARCH=amd64 GO111MODULE=on go build -ldflags ${VERSION_FLAGS} -o bin/shalm . 

docker-context/shalm:  bin/linux/shalm
	mkdir -p docker-context/
	cp bin/linux/shalm docker-context/shalm


define BOZO
bin/$(1)/shalm: $(GO_FILES)  go.sum go.mod Makefile
	mkdir -p bin/$(1)
	CGO_ENABLED=0 GOOS=$(1) GOARCH=amd64 GO111MODULE=on go build -ldflags ${VERSION_FLAGS} -o bin/$(1)/shalm .
bin/shalm-binary-$(1).tgz: bin/$(1)/shalm
	cd bin/$(1) &&  tar czf ../shalm-binary-$(1).tgz shalm
endef

$(foreach i,linux darwin windows,$(eval $(call BOZO,$(i))))

binaries: $(foreach i,linux darwin windows,bin/shalm-binary-$(i).tgz)

formula: homebrew-tap/shalm.rb

homebrew-tap/shalm.rb: bin/shalm-binary-darwin.tgz bin/shalm-binary-linux.tgz
	@mkdir -p homebrew-tap
	@sed  \
	-e "s/{{sha256-darwin}}/$$(shasum -b -a 256 bin/shalm-binary-darwin.tgz  | awk '{print $$1}')/g" \
	-e "s/{{sha256-linux}}/$$(shasum -b -a 256 bin/shalm-binary-linux.tgz  | awk '{print $$1}')/g" \
	-e "s/{{version}}/$(VERSION)/g" homebrew-formula.rb \
	> homebrew-tap/shalm.rb

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
