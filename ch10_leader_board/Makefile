# Makefile
.PHONY: all setup cluster deploy test clean logs port-forward

CLUSTER_NAME := leaderboard-demo
NAMESPACE := leaderboard

all: setup cluster deploy wait-ready

wait-ready:
	@echo "Waiting for all pods to be ready..."
	kubectl wait --for=condition=ready pod -l app=leaderboard --timeout=120s -n $(NAMESPACE) || true
	@echo ""
	@echo "=========================================="
	@echo "  Deployment Complete!"
	@echo "=========================================="
	@echo ""
	@echo "Access Points:"
	@echo "  - Grafana:     http://localhost:30300 (admin/admin)"
	@echo "  - Prometheus:  http://localhost:30090"
	@echo "  - App:         http://localhost:30080"
	@echo ""
	@echo "API Endpoints:"
	@echo "  - /v1/scores   -> PostgreSQL Only"
	@echo "  - /v2/scores   -> PostgreSQL + Valkey"
	@echo ""
	@echo "Run Tests:"
	@echo "  make test-scenario1   # Test /v1 (PostgreSQL only)"
	@echo "  make test-scenario2   # Test /v2 (PostgreSQL + Valkey)"
	@echo "  make test-both        # Run both scenarios"
	@echo ""

setup:
	@echo "Installing required tools..."
	@command -v k3d >/dev/null 2>&1 || (echo "Installing k3d..." && curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash)
	@command -v kubectl >/dev/null 2>&1 || (echo "Please install kubectl" && exit 1)
	@command -v helm >/dev/null 2>&1 || (echo "Installing helm..." && curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash)
	@command -v k6 >/dev/null 2>&1 || (echo "Installing k6..." && sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69 && echo "deb https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list && sudo apt-get update && sudo apt-get install k6)

cluster:
	@echo "Creating k3d cluster..."
	k3d cluster create --config k3d-config.yaml

	@echo "Waiting for cluster to be ready..."
	kubectl wait --for=condition=ready node --all --timeout=60s

deploy: deploy-namespace deploy-monitoring deploy-postgresql deploy-valkey deploy-app

deploy-namespace:
	kubectl apply -f k8s/namespace.yaml

deploy-monitoring:
	@echo "Deploying Prometheus, Grafana, and Tempo..."
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm repo add grafana https://grafana.github.io/helm-charts
	helm repo update

	helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
		--namespace $(NAMESPACE) \
		--set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
		--set 'prometheus.prometheusSpec.enableFeatures={remote-write-receiver}' \
		--set 'prometheus.prometheusSpec.additionalArgs[0].name=web.enable-remote-write-receiver' \
		--set grafana.adminPassword=admin \
		--set grafana.service.type=NodePort \
		--set grafana.service.nodePort=30300 \
		--set prometheus.service.type=NodePort \
		--set prometheus.service.nodePort=30090 \
		--set grafana.sidecar.datasources.enabled=true \
		--set grafana.sidecar.datasources.searchNamespace=$(NAMESPACE)

	@echo "Deploying Grafana dashboards and datasources..."
	kubectl apply -f k8s/monitoring/

deploy-postgresql:
	@echo "Deploying PostgreSQL via Helm..."
	helm repo add bitnami https://charts.bitnami.com/bitnami
	helm upgrade --install postgresql bitnami/postgresql \
		--namespace $(NAMESPACE) \
		--values k8s/postgresql-values.yaml \
		--wait --timeout 5m

deploy-valkey:
	@echo "Deploying Valkey..."
	kubectl apply -f k8s/valkey/

deploy-app:
	@echo "Building and deploying application..."
	docker build -t leaderboard-app:latest -f docker/Dockerfile.app .
	k3d image import leaderboard-app:latest -c $(CLUSTER_NAME)
	kubectl apply -f k8s/app/
	kubectl apply -f k8s/app/hpa.yaml

test-scenario1:
	@echo "Running Scenario 1: RDB Only (500 VUs, 5 min)"
	@echo "Service available at http://localhost:8081"
	k6 run k6/scenario1-rdb.js

test-scenario2:
	@echo "Running Scenario 2: With Valkey (500 VUs, 5 min)"
	@echo "Service available at http://localhost:8082"
	k6 run k6/scenario2-redis.js

test-both:
	@echo "Running both scenarios sequentially..."
	@echo ""
	@echo "=== Scenario 1: PostgreSQL Only (500 VUs, 5 min) ==="
	@echo "Service: http://localhost:8081"
	k6 run k6/scenario1-rdb.js
	@echo ""
	@echo "Waiting 30 seconds before next scenario..."
	@sleep 30
	@echo ""
	@echo "=== Scenario 2: PostgreSQL + Valkey (500 VUs, 5 min) ==="
	@echo "Service: http://localhost:8082"
	k6 run k6/scenario2-redis.js
	@echo ""
	@echo "=== Both scenarios completed ==="

clean:
	k3d cluster delete $(CLUSTER_NAME)