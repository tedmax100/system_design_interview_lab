# Leaderboard Version 1: PostgreSQL Only

這個版本使用純關聯式資料庫（PostgreSQL）來實作排行榜系統，用來展示在大規模資料下，SQL 查詢排名的性能問題。

## 架構概述

```
Client -> Leaderboard Service -> PostgreSQL
                |
                v
           Prometheus <- Grafana
```

## 技術棧

- **Language**: Go 1.25
- **Database**: PostgreSQL 17
- **Container**: Docker + k3d (Kubernetes)
- **Monitoring**: Prometheus + Grafana
- **Load Testing**: k6

## 性能問題演示

Version 1 實作了以下三個 API，並展示了 SQL 在排行榜場景下的性能瓶頸：

### 1. POST /v1/scores - 更新分數 ✅ 較快
使用 INSERT ... ON CONFLICT 語法，性能尚可。

### 2. GET /v1/scores - 取得 Top 10 ⚠️ 需要全表掃描
```sql
SELECT user_id, score, RANK() OVER (ORDER BY score DESC) as rank
FROM monthly_leaderboard
WHERE month = '2025-10'
ORDER BY score DESC
LIMIT 10
```
**問題**：即使只取 10 筆，仍需對整張表排序，時間複雜度 O(N log N)。

### 3. GET /v1/scores/{user_id} - 查詢用戶排名 ❌ 極慢
```sql
SELECT (SELECT COUNT(*) FROM monthly_leaderboard lb2
        WHERE lb2.score >= lb1.score) AS rank
FROM monthly_leaderboard lb1
WHERE lb1.user_id = :user_id
```
**問題**：每次查詢都需要 COUNT(*) 掃描全表，時間複雜度 O(N²)。

## 快速開始

### 前置要求

- Docker Desktop
- kubectl
- helm
- k6
- Go 1.23+ (for building)

### 部署步驟

```bash
# 1. 給予執行權限
chmod +x deploy.sh

# 2. 執行部署腳本
./deploy.sh

# 3. 等待部署完成（約 3-5 分鐘）
```

### 存取服務

```bash
# Port-forward Leaderboard API
kubectl port-forward -n leaderboard svc/leaderboard-service-rdb 8080:80

# 在新的終端視窗測試 API
curl http://localhost:8080/health
```

### Grafana Dashboard

1. 打開瀏覽器訪問: http://localhost:30300
2. 登入帳號密碼: `admin` / `admin`
3. 選擇 Dashboard > "Leaderboard Performance - RDB Only"

### Prometheus

訪問: http://localhost:30090

## 執行負載測試

### 步驟 1: 初始化測試資料

這會創建 1000 個用戶，每個用戶有 1-100 分的隨機分數：

```bash
# 確保已經 port-forward 服務
kubectl port-forward -n leaderboard svc/leaderboard-service-rdb 8080:80

# 在新的終端執行
k6 run k6/init-data.js
```

### 步驟 2: 執行負載測試

```bash
k6 run k6/scenario1-rdb.js
```

測試腳本會模擬：
- 70% 的請求是 **更新分數** (POST /v1/scores)
- 15% 的請求是 **查詢 Top 10** (GET /v1/scores)
- 15% 的請求是 **查詢用戶排名** (GET /v1/scores/{user_id})

負載會從 0 逐步提升到 150 VUs (模擬 2500 QPS)。

### 步驟 3: 觀察 Grafana

在 Grafana Dashboard 中觀察：

1. **HTTP Request Duration (Percentiles)** - 可以看到隨著負載增加，延遲顯著上升
2. **GET /v1/scores/{user_id} - P95 Latency** - 這個 API 會特別慢（通常 > 500ms）
3. **Request Rate** - 觀察 QPS

## 預期結果

基於 PostgreSQL 的實作會展現以下問題：

| 指標 | 目標 (SLO) | 實際表現 (RDB) |
|------|-----------|---------------|
| POST /v1/scores P95 | < 50ms | ✅ ~30-50ms |
| GET /v1/scores P95 | < 50ms | ⚠️ ~100-200ms |
| GET /v1/scores/{user_id} P95 | < 100ms | ❌ ~500-2000ms |

**關鍵發現**：
- 當資料量達到數十萬筆時，查詢用戶排名會變得極慢
- 即使加了索引，排名計算仍需要掃描大量資料
- 這就是為什麼需要 Redis Sorted Set 的原因

## 查看應用日誌

```bash
kubectl logs -f -l app=leaderboard,scenario=rdb-only -n leaderboard
```

## 查看 PostgreSQL 執行計畫

```bash
# 進入 PostgreSQL Pod
kubectl exec -it -n leaderboard postgresql-0 -- psql -U postgres -d leaderboard

# 查看查詢執行計畫
EXPLAIN ANALYZE
SELECT user_id, score,
  (SELECT COUNT(*) FROM monthly_leaderboard lb2
   WHERE lb2.score >= lb1.score) AS rank
FROM monthly_leaderboard lb1
WHERE lb1.user_id = 'player_1_100';
```

你會看到 `Seq Scan`（全表掃描）的執行計畫。

## 清理資源

```bash
# 刪除整個 k3d cluster
k3d cluster delete leaderboard-demo

# 或只刪除 namespace
kubectl delete namespace leaderboard
```

## 下一步: Version 2

Version 2 將使用 Redis Sorted Set 來優化排行榜查詢，展示性能的巨大提升：

- GET /v1/scores: O(log N + 10) - 從毫秒級降到次毫秒級
- GET /v1/scores/{user_id}: O(log N) - 從秒級降到毫秒級

## 故障排除

### 1. Pod 無法啟動

```bash
kubectl get pods -n leaderboard
kubectl describe pod <pod-name> -n leaderboard
kubectl logs <pod-name> -n leaderboard
```

### 2. Database 連線失敗

```bash
# 檢查 PostgreSQL 狀態
kubectl get statefulset -n leaderboard
kubectl logs postgresql-0 -n leaderboard
```

### 3. Port-forward 失敗

確保沒有其他程式佔用 8080 port：
```bash
lsof -i :8080
```

## 參考資料

- [PostgreSQL Window Functions](https://www.postgresql.org/docs/current/tutorial-window.html)
- [k6 Load Testing](https://k6.io/docs/)
- [Prometheus Query Examples](https://prometheus.io/docs/prometheus/latest/querying/examples/)
