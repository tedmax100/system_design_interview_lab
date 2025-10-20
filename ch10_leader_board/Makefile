# Makefile
.PHONY: all setup cluster deploy test clean

CLUSTER_NAME := leaderboard-demo
NAMESPACE := leaderboard

all: setup cluster deploy

setup:
	@echo "Installing required tools..."
	@command -v k3d >/dev/null 2>&1 || (echo "Installing k3d..." && curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash)
	@command -v kubectl >/dev/null 2>&1 || (echo "Please install kubectl" && exit 1)
	@command -v helm >/dev/null 2>&1 || (echo "Installing helm..." && curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash)
	@command -v k6 >/dev/null 2>&1 || (echo "Installing k6..." && sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69 && echo "deb https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list && sudo apt-get update && sudo apt-get install k6)

cluster:
	@echo "Creating k3d cluster..."
	k3d cluster create $(CLUSTER_NAME) \
		--servers 1 \
		--agents 3 \
		--port "8080:80@loadbalancer" \
		--port "8443:443@loadbalancer" \
		--port "30000-32767:30000-32767@server:0" \
		--volume "$(PWD)/data:/data" \
		--k3s-arg "--disable=traefik@server:0"
	
	@echo "Waiting for cluster to be ready..."
	kubectl wait --for=condition=ready node --all --timeout=60s

deploy: deploy-namespace deploy-monitoring deploy-postgresql deploy-valkey deploy-app

deploy-namespace:
	kubectl apply -f k8s/namespace.yaml

deploy-monitoring:
	@echo "Deploying Prometheus and Grafana..."
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm repo add grafana https://grafana.github.io/helm-charts
	helm repo update
	
	helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
		--namespace $(NAMESPACE) \
		--set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
		--set grafana.adminPassword=admin \
		--set grafana.service.type=NodePort \
		--set grafana.service.nodePort=30300 \
		--set prometheus.service.type=NodePort \
		--set prometheus.service.nodePort=30090

deploy-postgresql:
	@echo "Deploying PostgreSQL 17..."
	kubectl apply -f k8s/postgresql/

deploy-valkey:
	@echo "Deploying Valkey..."
	kubectl apply -f k8s/valkey/

deploy-app:
	@echo "Building and deploying application..."
	docker build -t leaderboard-app:latest -f docker/Dockerfile.app .
	k3d image import leaderboard-app:latest -c $(CLUSTER_NAME)
	kubectl apply -f k8s/app/

test-scenario1:
	@echo "Running Scenario 1: RDB Only"
	kubectl port-forward -n $(NAMESPACE) svc/leaderboard-service 8080:80 &
	sleep 5
	k6 run k6/scenario1-rdb.js

test-scenario2:
	@echo "Running Scenario 2: With Valkey"
	kubectl port-forward -n $(NAMESPACE) svc/leaderboard-service 8080:80 &
	sleep 5
	k6 run k6/scenario2-redis.js

clean:
	k3d cluster delete $(CLUSTER_NAME)