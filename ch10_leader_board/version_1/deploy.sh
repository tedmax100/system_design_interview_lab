#!/bin/bash

set -e

echo "========================================="
echo "Deploying Leaderboard Version 1 (RDB Only)"
echo "========================================="

CLUSTER_NAME="leaderboard-demo"
NAMESPACE="leaderboard"

# Check if k3d cluster exists
if ! k3d cluster list | grep -q "$CLUSTER_NAME"; then
    echo "Creating k3d cluster..."
    k3d cluster create $CLUSTER_NAME \
        --servers 1 \
        --agents 3 \
        --port "8080:80@loadbalancer" \
        --port "8443:443@loadbalancer" \
        --port "30000-32767:30000-32767@server:0" \
        --k3s-arg "--disable=traefik@server:0"

    echo "Waiting for cluster to be ready..."
    kubectl wait --for=condition=ready node --all --timeout=60s
else
    echo "Cluster already exists, using existing cluster"
fi

# Create namespace
echo "Creating namespace..."
kubectl apply -f ../k8s/namespace.yaml

# Deploy PostgreSQL
echo "Deploying PostgreSQL 17..."
kubectl apply -f ../k8s/postgresql/

echo "Waiting for PostgreSQL to be ready..."
kubectl wait --for=condition=ready pod -l app=postgresql -n $NAMESPACE --timeout=180s

# Build and deploy application (RDB-only version)
echo "Building application..."
cd ../src
go mod tidy
cd ..

echo "Building Docker image..."
docker build -t leaderboard-app:latest -f docker/Dockerfile.app .

echo "Importing image to k3d..."
k3d image import leaderboard-app:latest -c $CLUSTER_NAME

echo "Deploying application..."
kubectl apply -f k8s/app/deployment.yaml
kubectl apply -f k8s/app/servicemonitor.yaml

echo "Waiting for application to be ready..."
kubectl wait --for=condition=ready pod -l app=leaderboard,scenario=rdb-only -n $NAMESPACE --timeout=120s

# Deploy monitoring
echo "Setting up Prometheus and Grafana..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
    --namespace $NAMESPACE \
    --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
    --set grafana.adminPassword=admin \
    --set grafana.service.type=NodePort \
    --set grafana.service.nodePort=30300 \
    --set prometheus.service.type=NodePort \
    --set prometheus.service.nodePort=30090 \
    --wait

# Deploy Grafana dashboard
echo "Deploying Grafana dashboard..."
kubectl apply -f k8s/monitoring/grafana-dashboard.yaml

echo ""
echo "========================================="
echo "Deployment Complete!"
echo "========================================="
echo ""
echo "Service endpoints:"
echo "  Leaderboard API: kubectl port-forward -n $NAMESPACE svc/leaderboard-service-rdb 8080:80"
echo "  Prometheus: http://localhost:30090"
echo "  Grafana: http://localhost:30300 (admin/admin)"
echo ""
echo "To run load tests:"
echo "  1. Port-forward the service: kubectl port-forward -n $NAMESPACE svc/leaderboard-service-rdb 8080:80"
echo "  2. Initialize test data: k6 run k6/init-data.js"
echo "  3. Run load test: k6 run k6/scenario1-rdb.js"
echo ""
echo "To view logs:"
echo "  kubectl logs -f -l app=leaderboard,scenario=rdb-only -n $NAMESPACE"
echo ""
