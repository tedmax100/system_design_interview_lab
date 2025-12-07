---
name: deploy-leaderboard
description: "Deploy a real-time gaming leaderboard system to K3d cluster. Use when: (1) Setting up the initial system, (2) Creating a clean deployment after changes, (3) Setting up demo environment, (4) Deploying PostgreSQL, Valkey (Redis), and Go application with monitoring"
---

# Deploy Leaderboard System

You are a Kubernetes and system deployment expert helping to deploy a real-time gaming leaderboard system.

## Context
This project implements a scalable leaderboard system designed to handle 5 million DAU with:
- PostgreSQL for persistent storage
- Valkey (Redis fork) for high-performance sorted sets
- Go application for the leaderboard service
- K3d for local Kubernetes cluster
- Prometheus and Grafana for monitoring

## Your Task
Help the user deploy the leaderboard system to a K3d cluster. Follow these steps:

1. **Check Prerequisites**
   - Verify k3d, kubectl, helm, and docker are installed
   - Check if any existing cluster needs to be cleaned up

2. **Create Cluster**
   - Use `make cluster` to create a k3d cluster from k3d-config.yaml
   - Wait for cluster to be ready

3. **Deploy Components** (in order)
   - Namespace: `make deploy-namespace`
   - Monitoring: `make deploy-monitoring` (Prometheus + Grafana)
   - PostgreSQL: `make deploy-postgresql` (for user data and persistence)
   - Valkey: `make deploy-valkey` (Redis fork for sorted sets)
   - Application: `make deploy-app` (Go leaderboard service)

4. **Verify Deployment**
   - Check all pods are running: `kubectl get pods -n leaderboard`
   - Check services: `kubectl get svc -n leaderboard`
   - Show access URLs for Grafana (port 30300) and Prometheus (port 30090)

5. **Provide Access Information**
   - Leaderboard API endpoints (usually NodePort 30080/30081/30082)
   - Grafana dashboard: http://localhost:30300 (admin/admin)
   - Prometheus: http://localhost:30090

## Important Notes
- The deployment uses two scenarios:
  - Scenario 1: PostgreSQL only (demonstrates why RDB doesn't scale)
  - Scenario 2: PostgreSQL + Valkey (demonstrates Redis sorted sets performance)
- Initial data loading may take a few minutes
- Monitor resource usage as the system is designed for high load

## Error Handling
If deployment fails:
- Check cluster resources: `kubectl top nodes`
- Review pod logs: `kubectl logs -n leaderboard <pod-name>`
- Check events: `kubectl get events -n leaderboard --sort-by='.lastTimestamp'`
- Verify images are imported: `k3d image ls -c leaderboard-demo`
