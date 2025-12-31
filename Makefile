IMAGE ?= ksm-scale-operator:latest
NAMESPACE ?= monitoring

.PHONY: help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Development:"
	@echo "  dev          - Complete dev setup (kind cluster + operator + sample)"
	@echo "  logs-kind    - Stream operator logs from kind cluster"
	@echo "  ksm-sharding - Analyze KSM sharding metrics distribution"
	@echo "  clean-dev    - Clean up development environment"
	@echo ""
	@echo "Building:"
	@echo "  build        - Build the operator binary"
	@echo "  docker-build - Build the Docker image"
	@echo "  docker-push  - Push the Docker image"
	@echo ""
	@echo "Testing:"
	@echo "  test         - Run unit tests"
	@echo "  test-e2e     - Run end-to-end tests"
	@echo "  test-coverage - Generate test coverage report"
	@echo ""
	@echo "Production Deployment:"
	@echo "  deploy       - Deploy to Kubernetes cluster"
	@echo "  undeploy     - Remove from Kubernetes cluster"
	@echo "  sample       - Create sample StateMonitor"

.PHONY: build
build:
	go build -o bin/manager cmd/manager/main.go

.PHONY: run
run:
	go run cmd/manager/main.go

.PHONY: docker-build
docker-build:
	$(CONTAINER_RUNTIME) build -t $(IMAGE) .

.PHONY: docker-push
docker-push:
	docker push $(IMAGE)

.PHONY: deploy
deploy:
	kubectl apply -f config/crd/bases/monitor.example.com_ksmscales.yaml
	kubectl apply -f config/rbac/service_account.yaml
	kubectl apply -f config/rbac/role.yaml
	kubectl apply -f config/rbac/role_binding.yaml
	kubectl apply -f config/rbac/kube_state_metrics_rbac.yaml
	kubectl apply -f config/manager/manager.yaml

.PHONY: undeploy
undeploy:
	kubectl delete -f config/manager/manager.yaml --ignore-not-found
	kubectl delete -f config/rbac/role_binding.yaml --ignore-not-found
	kubectl delete -f config/rbac/role.yaml --ignore-not-found
	kubectl delete -f config/rbac/service_account.yaml --ignore-not-found
	kubectl delete -f config/crd/bases/monitor.example.com_ksmscales.yaml --ignore-not-found

.PHONY: test
test:
	go test ./api/... ./controllers/... -coverprofile cover.out

.PHONY: test-e2e
test-e2e:
	go test ./test/e2e/... -v -tags=e2e

.PHONY: test-all
test-all: test test-e2e

.PHONY: test-coverage
test-coverage:
	go test ./... -coverprofile=cover.out
	go tool cover -func=cover.out
	go tool cover -html=cover.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: install-crd
install-crd:
	kubectl apply -f config/crd/bases/monitor.example.com_ksmscales.yaml

.PHONY: uninstall-crd
uninstall-crd:
	kubectl delete -f config/crd/bases/monitor.example.com_ksmscales.yaml --ignore-not-found

.PHONY: sample
sample:
	kubectl apply -f manifests/sample-ksmscale.yaml

# Kind development targets
KIND_CLUSTER_NAME ?= ksm-scale-dev
KIND_IMAGE_NAME ?= ksm-scale-operator
KIND_IMAGE_TAG ?= dev

.PHONY: kind-setup
kind-setup:
	@./scripts/kind-setup.sh

.PHONY: kind-cluster
kind-cluster:
	@if ! kind get clusters | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
		kind create cluster --config config/kind/cluster.yaml; \
	else \
		echo "Cluster '$(KIND_CLUSTER_NAME)' already exists"; \
	fi

.PHONY: kind-delete
kind-delete:
	kind delete cluster --name $(KIND_CLUSTER_NAME)

# Detect container runtime
CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || command -v docker 2> /dev/null)

.PHONY: docker-build-kind
docker-build-kind:
	$(CONTAINER_RUNTIME) build -f Dockerfile.dev -t $(KIND_IMAGE_NAME):$(KIND_IMAGE_TAG) .

.PHONY: kind-load
kind-load: docker-build-kind
	@if command -v podman > /dev/null 2>&1; then \
		podman tag localhost/$(KIND_IMAGE_NAME):$(KIND_IMAGE_TAG) docker.io/library/$(KIND_IMAGE_NAME):$(KIND_IMAGE_TAG) && \
		podman save docker.io/library/$(KIND_IMAGE_NAME):$(KIND_IMAGE_TAG) -o /tmp/operator.tar && \
		kind load image-archive /tmp/operator.tar --name $(KIND_CLUSTER_NAME) && \
		rm -f /tmp/operator.tar; \
	else \
		kind load docker-image $(KIND_IMAGE_NAME):$(KIND_IMAGE_TAG) --name $(KIND_CLUSTER_NAME); \
	fi

.PHONY: deploy-kind
deploy-kind: kind-load
	kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f config/crd/bases/monitor.example.com_ksmscales.yaml
	kubectl apply -f config/dev/service_account.yaml
	kubectl apply -f config/rbac/role.yaml
	kubectl apply -f config/rbac/role_binding.yaml
	kubectl apply -f config/rbac/kube_state_metrics_rbac.yaml
	kubectl apply -f config/dev/manager_deployment.yaml

.PHONY: undeploy-kind
undeploy-kind:
	kubectl delete -f config/dev/manager_deployment.yaml --ignore-not-found
	kubectl delete -f config/rbac/role_binding.yaml --ignore-not-found
	kubectl delete -f config/rbac/role.yaml --ignore-not-found
	kubectl delete -f config/dev/service_account.yaml --ignore-not-found
	kubectl delete -f config/crd/bases/monitor.example.com_ksmscales.yaml --ignore-not-found

.PHONY: sample-dev
sample-dev:
	kubectl apply -f manifests/sample-ksmscale-sharded.yaml

.PHONY: logs-kind
logs-kind:
	kubectl logs -n monitoring deployment/ksm-scale-operator -f

.PHONY: dev
dev: kind-cluster deploy-kind sample-dev
	@echo "Development environment ready!"
	@echo "Watch logs: make logs-kind"

.PHONY: clean-dev
clean-dev: undeploy-kind kind-delete
	@echo "Development environment cleaned up"

.PHONY: ksm-sharding
ksm-sharding:
	@./scripts/ksm-sharding-analysis.sh $(NAMESPACE)