---
name: scale-leaderboard
description: "Scale the leaderboard system from 5M to 500M+ users. Use when: (1) Planning for 10x or 100x growth, (2) Implementing Redis sharding strategies (fixed partition or hash partition), (3) Optimizing for higher loads and QPS, (4) Cost optimization at scale, (5) Implementing scatter-gather pattern for distributed queries"
---

# Scale Leaderboard System

You are a distributed systems architect specializing in horizontal scaling, sharding strategies, and high-availability systems.

## Context
The current leaderboard is designed for 5 million DAU. This skill helps you understand and implement strategies to scale to 100x (500 million DAU) or beyond.

## Scaling Dimensions

### 1. Vertical Scaling (Scale Up)
Increase resources on existing nodes:

#### Redis/Valkey Memory
```yaml
# k8s/valkey/deployment.yaml
resources:
  requests:
    memory: "8Gi"  # Increase from default
    cpu: "2"
  limits:
    memory: "16Gi"
    cpu: "4"
```

Calculate memory needs:
- 26 bytes per leaderboard entry
- 500M users × 26 bytes = 13GB
- Skip list overhead: 2x = 26GB
- Total: ~32GB recommended

#### Application Instances
```bash
# Scale horizontally (preferred over vertical)
kubectl scale deployment leaderboard-app -n leaderboard --replicas=5

# Or use HPA (Horizontal Pod Autoscaler)
kubectl autoscale deployment leaderboard-app \
  -n leaderboard \
  --cpu-percent=70 \
  --min=3 \
  --max=10
```

### 2. Horizontal Scaling - Redis Sharding

#### Strategy A: Fixed Partition by Score Range
Best for evenly distributed scores.

```
Shard 1: scores 1-100
Shard 2: scores 101-200
Shard 3: scores 201-300
...
Shard N: scores 901-1000
```

**Implementation considerations:**
```go
// Route to shard based on score
func getShardByScore(score int) int {
    return score / 100  // 100-point ranges
}

// For top 10, query highest shard first
// For user rank, need to aggregate across shards
```

**Pros:**
- Simple to implement
- Easy to query top N (just query top shards)

**Cons:**
- Requires even score distribution
- Handling users moving between shards is complex
- Need to maintain score→shard mapping cache

#### Strategy B: Hash Partition (Redis Cluster)
Industry standard for Redis scaling.

```
Slot = CRC16(key) % 16384
Shard 1: slots 0-5500
Shard 2: slots 5501-11000
Shard 3: slots 11001-16383
```

**Configuration:**
```yaml
# k8s/valkey/statefulset-cluster.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: valkey-cluster
spec:
  serviceName: valkey-cluster
  replicas: 6  # 3 masters + 3 replicas
  selector:
    matchLabels:
      app: valkey-cluster
```

**Initialization:**
```bash
# Create cluster
redis-cli --cluster create \
  valkey-0.valkey-cluster:6379 \
  valkey-1.valkey-cluster:6379 \
  valkey-2.valkey-cluster:6379 \
  valkey-3.valkey-cluster:6379 \
  valkey-4.valkey-cluster:6379 \
  valkey-5.valkey-cluster:6379 \
  --cluster-replicas 1
```

**Pros:**
- Automatic load distribution
- Built-in failover with replicas
- Industry standard, well-tested

**Cons:**
- Cannot do cross-shard operations easily
- Need scatter-gather for global top N
- More complex to operate

### 3. Scatter-Gather Pattern

For getting global top 10 from sharded system:

```go
func GetGlobalTop10(shards []RedisClient) []Player {
    // Step 1: Scatter - query each shard for its top 10
    results := make(chan []Player, len(shards))

    for _, shard := range shards {
        go func(s RedisClient) {
            results <- s.ZRevRange("leaderboard", 0, 9)
        }(shard)
    }

    // Step 2: Gather - collect all results
    allPlayers := []Player{}
    for i := 0; i < len(shards); i++ {
        allPlayers = append(allPlayers, <-results...)
    }

    // Step 3: Sort and take top 10
    sort.Slice(allPlayers, func(i, j int) bool {
        return allPlayers[i].Score > allPlayers[j].Score
    })

    return allPlayers[:10]
}
```

**Performance characteristics:**
- Latency: Limited by slowest shard
- Parallelization: Queries run concurrently
- Network: (N shards) × (top K) records transferred
- Computation: Sort at most N×K records

### 4. Database Scaling

#### PostgreSQL Scaling
Current setup uses single PostgreSQL instance. For scale:

**Read Replicas:**
```yaml
# k8s/postgresql-values.yaml
replication:
  enabled: true
  readReplicas: 3

# Route read queries to replicas
primary:
  service: postgresql-primary
readReplicas:
  service: postgresql-read
```

**Connection Pooling:**
```go
// src/internal/config/config.go
type DatabaseConfig struct {
    MaxOpenConns    int // 100-200 for high load
    MaxIdleConns    int // 50-100
    ConnMaxLifetime time.Duration // 5-10 minutes
}
```

**Partitioning:**
```sql
-- Partition user table by ID range
CREATE TABLE users_0 PARTITION OF users FOR VALUES FROM (0) TO (1000000);
CREATE TABLE users_1 PARTITION OF users FOR VALUES FROM (1000000) TO (2000000);
-- ... etc
```

### 5. Caching Layer

Add Redis cache for user profiles (hot data):

```go
// Cache top 100 players' profiles
func GetPlayerProfile(userID string) (*Profile, error) {
    // Try cache first
    if cached := redis.Get("profile:" + userID); cached != nil {
        return cached, nil
    }

    // Cache miss - fetch from PostgreSQL
    profile := db.Query("SELECT * FROM users WHERE id = ?", userID)

    // Cache for 5 minutes
    redis.Set("profile:" + userID, profile, 5*time.Minute)

    return profile, nil
}
```

### 6. Load Balancing

#### Application Layer
```yaml
# k8s/app/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: leaderboard-app
spec:
  type: LoadBalancer  # or NodePort
  sessionAffinity: None  # Round-robin
  selector:
    app: leaderboard-app
  ports:
  - port: 80
    targetPort: 8080
```

#### Redis Layer
Use Redis Sentinel for automatic failover:
```yaml
# k8s/valkey/sentinel.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: valkey-sentinel-config
data:
  sentinel.conf: |
    sentinel monitor mymaster valkey-0.valkey 6379 2
    sentinel down-after-milliseconds mymaster 5000
    sentinel parallel-syncs mymaster 1
    sentinel failover-timeout mymaster 10000
```

## Scaling Decision Tree

```
Current load: 2,500 QPS (5M DAU)
Target load: 250,000 QPS (500M DAU)

1. Can single Redis handle it?
   - Memory: 32GB+ needed
   - QPS: Redis can handle 100K+ QPS
   - Decision: Yes, if using powerful instance
   - Recommendation: Still shard for resilience

2. How many shards needed?
   - Memory: 32GB ÷ 8GB per instance = 4 shards minimum
   - QPS: 250K ÷ 50K per shard = 5 shards
   - Recommendation: Start with 6 shards (3 masters + 3 replicas)

3. Application instances?
   - Each instance handles ~5K QPS
   - 250K ÷ 5K = 50 instances
   - Recommendation: Use HPA, start with 10-20

4. Database?
   - PostgreSQL: Add read replicas (3-5)
   - Connection pooling: 200 connections per instance
   - Partitioning: Consider user_id hash partitioning
```

## Performance Testing at Scale

```bash
# Test with higher load
k6 run --vus 1000 --duration 5m k6/stress-test.js

# Monitor during test
watch kubectl top pods -n leaderboard

# Check for bottlenecks
kubectl exec -it <app-pod> -n leaderboard -- \
  wget -O- http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
```

## Cost Optimization

Balance performance vs. cost:

1. **Use spot instances** for non-critical workloads
2. **Auto-scale down** during off-peak hours
3. **Cache aggressively** to reduce database load
4. **Use Redis persistence** wisely (RDB vs AOF)
5. **Monitor and right-size** resources regularly

## Monitoring at Scale

Add metrics for scaled system:
```go
// Shard-specific metrics
shardLatency := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "redis_shard_latency_seconds",
        Help: "Latency per Redis shard",
    },
    []string{"shard"},
)

// Scatter-gather metrics
scatterGatherLatency := prometheus.NewHistogram(
    prometheus.HistogramOpts{
        Name: "scatter_gather_latency_seconds",
        Help: "End-to-end scatter-gather latency",
    },
)
```

## Your Task

When helping with scaling:

1. **Assess current capacity**
   - What's the current load?
   - Where are the bottlenecks?
   - What are the resource limits?

2. **Calculate requirements**
   - Target DAU/QPS
   - Memory needs
   - CPU requirements
   - Network bandwidth

3. **Propose scaling strategy**
   - Vertical vs horizontal
   - Sharding approach
   - Caching strategy
   - Database scaling

4. **Implement incrementally**
   - Start with 2x capacity
   - Test and measure
   - Scale further as needed

5. **Monitor and optimize**
   - Track key metrics
   - Identify bottlenecks
   - Adjust based on real data
