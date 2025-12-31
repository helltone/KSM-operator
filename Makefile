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
	@echo "══════════════════════════════════════════════════════════════════════════════"
	@echo "                        KSM Sharding Analysis Report                          "
	@echo "══════════════════════════════════════════════════════════════════════════════"
	@echo ""
	@# Create temp files
	@rm -f /tmp/ksm_*.txt
	@touch /tmp/ksm_analysis.txt
	@# Get all KSM pods
	@kubectl get pods -n monitoring -l app.kubernetes.io/name=kube-state-metrics \
		-o custom-columns=POD:metadata.name,NODE:spec.nodeName,SHARD:metadata.labels.shard-id,REPLICA:metadata.labels.shard-replica,STATUS:status.phase \
		--no-headers > /tmp/ksm_pods.txt || { echo "Error: No KSM pods found"; exit 1; }
	@# Process each pod
	@echo "Collecting metrics from pods..." >&2
	@port=9090; \
	while IFS= read -r line; do \
		pod=$$(echo "$$line" | awk '{print $$1}'); \
		node=$$(echo "$$line" | awk '{print $$2}'); \
		shard=$$(echo "$$line" | awk '{print $$3}'); \
		replica=$$(echo "$$line" | awk '{print $$4}'); \
		status=$$(echo "$$line" | awk '{print $$5}'); \
		[ -z "$$pod" ] && continue; \
		printf "  %-45s" "$$pod..." >&2; \
		if [ "$$status" = "Running" ]; then \
			kubectl port-forward -n monitoring $$pod $$port:8080 > /dev/null 2>&1 & \
			pf_pid=$$!; \
			sleep 2; \
			if kill -0 $$pf_pid 2>/dev/null; then \
				metrics=$$(curl -s localhost:$$port/metrics 2>/dev/null); \
				if [ -n "$$metrics" ]; then \
					total=$$(echo "$$metrics" | grep -c "^kube_" || echo "0"); \
					pods=$$(echo "$$metrics" | grep -c "^kube_pod_info" || echo "0"); \
					deployments=$$(echo "$$metrics" | grep -c "^kube_deployment_" || echo "0"); \
					services=$$(echo "$$metrics" | grep -c "^kube_service_" || echo "0"); \
					nodes=$$(echo "$$metrics" | grep -c "^kube_node_" || echo "0"); \
					namespaces=$$(echo "$$metrics" | grep -c "^kube_namespace_" || echo "0"); \
					echo " ✓" >&2; \
				else \
					total="0"; pods="0"; deployments="0"; services="0"; nodes="0"; namespaces="0"; \
					echo " ✗ (no metrics)" >&2; \
				fi; \
			else \
				total="0"; pods="0"; deployments="0"; services="0"; nodes="0"; namespaces="0"; \
				echo " ✗ (port-forward failed)" >&2; \
			fi; \
			kill $$pf_pid 2>/dev/null || true; \
			wait $$pf_pid 2>/dev/null || true; \
			port=$$((port + 1)); \
		else \
			total="0"; pods="0"; deployments="0"; services="0"; nodes="0"; namespaces="0"; \
			echo " ✗ ($$status)" >&2; \
		fi; \
		echo "$$pod|$$node|$$shard|$$replica|$$total|$$pods|$$deployments|$$services|$$nodes|$$namespaces" >> /tmp/ksm_analysis.txt; \
	done < /tmp/ksm_pods.txt
	@echo "" >&2
	@echo ""
	@printf "%-42s %-22s %8s %8s %10s %8s\n" "Pod Name" "Node" "Shard" "Replica" "Status" "Metrics"
	@printf "%-42s %-22s %8s %8s %10s %8s\n" "--------" "----" "-----" "-------" "------" "-------"
	@while IFS='|' read -r pod node shard replica total pods deployments services nodes namespaces; do \
		if [ -n "$$pod" ] && [ "$$pod" != "POD" ]; then \
			if [ "$$total" = "0" ]; then \
				status="FAILED"; \
			else \
				status="OK"; \
			fi; \
			printf "%-42s %-22s %8s %8s %10s %8s\n" "$$pod" "$$node" "$$shard" "$$replica" "$$status" "$$total"; \
		fi; \
	done < /tmp/ksm_analysis.txt
	@echo ""
	@echo "Metrics Breakdown by Shard:"
	@echo ""
	@printf "%6s %8s %8s %8s %8s %8s %8s %12s %8s\n" "Shard" "Replica" "Total" "Pods" "Deploy" "Service" "Nodes" "Namespaces" "Status"
	@printf "%6s %8s %8s %8s %8s %8s %8s %12s %8s\n" "-----" "-------" "-----" "----" "------" "-------" "-----" "-----------" "------"
	@sort -t'|' -k3,3n -k4,4n /tmp/ksm_analysis.txt | while IFS='|' read -r pod node shard replica total pods deploys services nodes ns; do \
		if [ -n "$$pod" ] && [ "$$pod" != "POD" ] && [ -n "$$shard" ]; then \
			if [ "$$total" = "0" ]; then \
				status="FAILED"; \
			else \
				status="OK"; \
			fi; \
			printf "%6s %8s %8s %8s %8s %8s %8s %12s %8s\n" \
				"$$shard" "$$replica" "$$total" "$$pods" "$$deploys" "$$services" "$$nodes" "$$ns" "$$status"; \
		fi; \
	done
	@echo ""
	@echo "Summary:"
	@echo ""
	@total_pods=$$(wc -l < /tmp/ksm_pods.txt | tr -d ' '); \
	configured_shards=$$(kubectl get ksmscales -n monitoring -o jsonpath='{.items[0].spec.sharding.shardCount}' 2>/dev/null || echo "N/A"); \
	min_replicas=$$(kubectl get ksmscales -n monitoring -o jsonpath='{.items[0].spec.sharding.minReplicasPerShard}' 2>/dev/null || echo "N/A"); \
	max_replicas=$$(kubectl get ksmscales -n monitoring -o jsonpath='{.items[0].spec.sharding.maxReplicasPerShard}' 2>/dev/null || echo "N/A"); \
	total_sum=$$(awk -F'|' '{sum+=$$5} END {print sum}' /tmp/ksm_analysis.txt); \
	avg_metrics=$$(awk -F'|' 'BEGIN{sum=0;count=0} {if($$5>0){sum+=$$5;count++}} END {if(count>0) printf "%.0f", sum/count; else print "0"}' /tmp/ksm_analysis.txt); \
	printf "%-40s: %s\n" "Total KSM Pods" "$$total_pods"; \
	printf "%-40s: %s\n" "Configured Shards" "$$configured_shards"; \
	printf "%-40s: %s\n" "Replicas per Shard" "$$min_replicas-$$max_replicas"; \
	printf "%-40s: %s\n" "Total Metrics (all replicas)" "$$total_sum"; \
	printf "%-40s: %s\n" "Average Metrics per Working Pod" "$$avg_metrics"
	@echo ""
	@echo "Shard Configuration Verification:"
	@configured_shards=$$(kubectl get ksmscales -n monitoring -o jsonpath='{.items[0].spec.sharding.shardCount}' 2>/dev/null || echo "0"); \
	echo "Expected shards: 0 to $$((configured_shards - 1)) (total: $$configured_shards shards)"; \
	echo "Actual shards found: $$(cut -d'|' -f3 /tmp/ksm_analysis.txt | sort -u | grep -v '^$$' | tr '\n' ' ')"; \
	echo ""
	@echo "Shard Consistency Check:"
	@for shard_id in $$(cut -d'|' -f3 /tmp/ksm_analysis.txt | sort -u | grep -v '^$$'); do \
		if [ "$$shard_id" != "<none>" ] && [ -n "$$shard_id" ]; then \
			echo "  Shard $$shard_id:"; \
			grep "|$$shard_id|" /tmp/ksm_analysis.txt | while IFS='|' read -r pod node shard replica total rest; do \
				if [ -n "$$pod" ] && [ -n "$$replica" ]; then \
					printf "    Replica %s: %5s metrics (Pod: %s)\n" "$$replica" "$$total" "$$pod"; \
				fi; \
			done; \
		fi; \
	done
	@# Cleanup
	@rm -f /tmp/ksm_*.txt
	@echo ""
	@echo "Analysis complete!"