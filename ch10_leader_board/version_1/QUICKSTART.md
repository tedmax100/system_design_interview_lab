# 快速啟動指南

## 一鍵部署

```bash
cd version_1
chmod +x deploy.sh test-api.sh
./deploy.sh
```

等待約 3-5 分鐘完成部署。

## 測試 API

在新的終端視窗執行：

```bash
# Terminal 1: Port-forward
kubectl port-forward -n leaderboard svc/leaderboard-service-rdb 8080:80
```

```bash
# Terminal 2: 測試 API
cd version_1
./test-api.sh
```

## 訪問監控介面

- **Grafana**: http://localhost:30300 (admin/admin)
- **Prometheus**: http://localhost:30090

## 執行負載測試

```bash
# Terminal 2: 初始化測試資料
k6 run k6/init-data.js

# 執行負載測試
k6 run k6/scenario1-rdb.js
```

在 Grafana 觀察：
1. 打開 Dashboard: "Leaderboard Performance - RDB Only"
2. 觀察 P95 延遲指標，特別是 GET /v1/scores/{user_id}

## 預期效果

你會看到：
- ✅ POST /v1/scores - 快速（~30-50ms）
- ⚠️ GET /v1/scores - 中等（~100-200ms）
- ❌ GET /v1/scores/{user_id} - 很慢（~500-2000ms）

這證明了使用 SQL 來做排行榜的性能問題！

## 清理

```bash
k3d cluster delete leaderboard-demo
```

## 故障排除

如果遇到問題：

```bash
# 檢查 Pod 狀態
kubectl get pods -n leaderboard

# 檢查日誌
kubectl logs -l app=leaderboard,scenario=rdb-only -n leaderboard

# 檢查 PostgreSQL
kubectl logs postgresql-0 -n leaderboard
```
