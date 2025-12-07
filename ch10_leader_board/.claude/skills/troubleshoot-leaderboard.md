---
name: troubleshoot-leaderboard
description: "Debug and fix issues in the deployed leaderboard system. Use when: (1) Pods are failing or crashing (CrashLoopBackOff, ImagePullBackOff), (2) Performance issues or high latency, (3) Database or Redis connection problems, (4) Deployment errors, (5) API returning errors, (6) Resource exhaustion"
---

# Troubleshoot Leaderboard System

You are a DevOps and SRE expert specializing in debugging distributed systems, with expertise in Kubernetes, Redis, PostgreSQL, and Go applications.

## Context
This leaderboard system runs on K3d with multiple components that can fail. You need to diagnose issues quickly and provide actionable solutions.

## Common Issues and Solutions

### 1. Cluster Issues

#### Cluster won't start
```bash
# Check if ports are already in use
sudo lsof -i :6443 -i :8080 -i :30080-30300

# Delete existing cluster and recreate
k3d cluster delete leaderboard-demo
make cluster
```

#### Cluster is slow/unresponsive
```bash
# Check node resources
kubectl top nodes

# Check if nodes are ready
kubectl get nodes

# Restart the cluster
k3d cluster stop leaderboard-demo
k3d cluster start leaderboard-demo
```

### 2. Pod Issues

#### Pods stuck in Pending
```bash
# Check pod status
kubectl get pods -n leaderboard

# Describe pod to see events
kubectl describe pod <pod-name> -n leaderboard

# Common causes:
# - Insufficient resources
# - Image pull errors
# - Volume mount issues
```

#### Pods in CrashLoopBackOff
```bash
# Check logs
kubectl logs <pod-name> -n leaderboard --tail=100

# Check previous crash logs
kubectl logs <pod-name> -n leaderboard --previous

# Common causes:
# - Application panic/error
# - Database connection failure
# - Configuration error
```

#### Pods in ImagePullBackOff
```bash
# Check if image exists in k3d
k3d image ls -c leaderboard-demo

# Reimport image
docker build -t leaderboard-app:latest -f docker/Dockerfile.app .
k3d image import leaderboard-app:latest -c leaderboard-demo

# Restart deployment
kubectl rollout restart deployment/leaderboard-app -n leaderboard
```

### 3. Database Issues

#### PostgreSQL won't start
```bash
# Check PostgreSQL pod
kubectl get pods -n leaderboard -l app.kubernetes.io/name=postgresql

# Check logs
kubectl logs -n leaderboard -l app.kubernetes.io/name=postgresql

# Check PVC
kubectl get pvc -n leaderboard

# Common fixes:
# - Delete and recreate: helm uninstall postgresql -n leaderboard
# - Check storage provisioner
```

#### Cannot connect to PostgreSQL
```bash
# Test connection from within cluster
kubectl run -it --rm psql-test --image=postgres:15 -n leaderboard -- \
  psql -h postgresql -U postgres -d leaderboard

# Check service
kubectl get svc postgresql -n leaderboard

# Check credentials
kubectl get secret postgresql -n leaderboard -o yaml
```

#### PostgreSQL performance issues
```bash
# Check active connections
kubectl exec -it postgresql-0 -n leaderboard -- \
  psql -U postgres -c "SELECT count(*) FROM pg_stat_activity;"

# Check slow queries
kubectl exec -it postgresql-0 -n leaderboard -- \
  psql -U postgres -c "SELECT query, calls, total_time FROM pg_stat_statements ORDER BY total_time DESC LIMIT 10;"

# Add indexes if needed
# Review queries in src/internal/repository/postgres.go
```

### 4. Valkey/Redis Issues

#### Valkey won't start
```bash
# Check pod status
kubectl get pods -n leaderboard -l app=valkey

# Check logs
kubectl logs -n leaderboard -l app=valkey

# Common issues:
# - Port conflicts
# - Memory limits too low
# - Configuration errors
```

#### Cannot connect to Valkey
```bash
# Test connection
kubectl run -it --rm redis-test --image=redis:7 -n leaderboard -- \
  redis-cli -h valkey-service ping

# Should return: PONG

# Check service
kubectl get svc valkey-service -n leaderboard
```

#### Valkey memory issues
```bash
# Check memory usage
kubectl exec -it <valkey-pod> -n leaderboard -- redis-cli INFO memory

# Check sorted set size
kubectl exec -it <valkey-pod> -n leaderboard -- \
  redis-cli ZCARD leaderboard_<month>

# Clear old data if needed
kubectl exec -it <valkey-pod> -n leaderboard -- \
  redis-cli DEL old_leaderboard_key
```

### 5. Application Issues

#### App won't start
```bash
# Check logs
kubectl logs -n leaderboard deployment/leaderboard-app

# Common errors to look for:
# - "failed to connect to database"
# - "failed to connect to redis"
# - "port already in use"
# - Configuration errors

# Check environment variables
kubectl describe pod <app-pod> -n leaderboard | grep -A 20 Environment
```

#### High latency
```bash
# Check Prometheus metrics
# Go to http://localhost:30090
# Query: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Check database connection pool
# Look for connection pool exhaustion in logs

# Check if Valkey is being used
# Scenario 1 uses only PostgreSQL (slow)
# Scenario 2 uses Valkey (fast)
```

#### API returns errors
```bash
# Test endpoints
curl -X POST http://localhost:30080/v1/scores \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test123", "points": 10}'

curl http://localhost:30080/v1/scores

# Check app logs for errors
kubectl logs -n leaderboard deployment/leaderboard-app --tail=50 -f
```

### 6. Performance Issues

#### Low throughput in load tests
```bash
# Check resource limits
kubectl describe pod <app-pod> -n leaderboard | grep -A 5 Limits

# Increase resources if needed
# Edit k8s/app/deployment.yaml

# Check connection pools
# Review src/internal/config/config.go

# Scale horizontally
kubectl scale deployment leaderboard-app -n leaderboard --replicas=3
```

#### High memory usage
```bash
# Check memory metrics
kubectl top pods -n leaderboard

# Check for memory leaks in Go app
# Look for goroutine leaks
kubectl exec -it <app-pod> -n leaderboard -- \
  wget -O- http://localhost:6060/debug/pprof/goroutine?debug=1

# Restart if needed
kubectl rollout restart deployment/leaderboard-app -n leaderboard
```

### 7. Monitoring Issues

#### Grafana not accessible
```bash
# Check Grafana pod
kubectl get pods -n leaderboard -l app.kubernetes.io/name=grafana

# Check NodePort service
kubectl get svc -n leaderboard | grep grafana

# Port forward if NodePort doesn't work
kubectl port-forward -n leaderboard svc/prometheus-grafana 3000:80
```

#### Prometheus not scraping metrics
```bash
# Check ServiceMonitor
kubectl get servicemonitor -n leaderboard

# Check Prometheus targets
# Go to http://localhost:30090/targets

# Verify app exposes metrics
curl http://<app-pod-ip>:8080/metrics
```

## Diagnostic Commands Cheat Sheet

```bash
# Quick health check
kubectl get all -n leaderboard

# Check events
kubectl get events -n leaderboard --sort-by='.lastTimestamp' | tail -20

# Check resource usage
kubectl top pods -n leaderboard
kubectl top nodes

# Restart everything
kubectl rollout restart deployment -n leaderboard

# Clean slate
make clean
make all

# View all logs
kubectl logs -n leaderboard --all-containers=true --tail=100
```

## Debugging Workflow

1. **Identify the symptom**
   - API errors? Check app logs
   - Slow performance? Check metrics
   - Can't deploy? Check K8s events

2. **Narrow down the component**
   - Use `kubectl get pods` to find failing pods
   - Use `kubectl logs` to see what's wrong
   - Use `kubectl describe` to see events

3. **Check dependencies**
   - Is PostgreSQL healthy?
   - Is Valkey healthy?
   - Are services exposing correct ports?

4. **Test connections**
   - Can app reach database?
   - Can you reach app from outside cluster?
   - Are services properly configured?

5. **Fix and verify**
   - Apply fix
   - Wait for pods to be ready
   - Test the functionality
   - Monitor for regression

## When to ask for help

Provide these details:
- Output of `kubectl get pods -n leaderboard`
- Relevant logs from failing components
- Error messages from the application
- What you were trying to do when the issue occurred
- Steps already taken to troubleshoot
