# 快速啟動指南

## 一鍵部署

```bash
cd version_1
chmod +x deploy.sh test-api.sh
./deploy.sh
```

⏱️ **等待約 8-10 分鐘完成部署**

部署過程包含：
1. 創建 k3d cluster
2. 部署 PostgreSQL 17
3. **預填充 50,000 筆測試資料** ⬅️ 這步驟需要較長時間
4. 部署 Leaderboard Service
5. 安裝 Prometheus + Grafana

> 💡 PostgreSQL 會在初始化時自動建立 50,000 個用戶及對應的排行榜資料，這樣可以立即展示性能問題。

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

資料庫已經有 **50,000 個用戶**的基礎資料了！可以直接開始測試：

```bash
# Terminal 2: 直接執行負載測試
k6 run k6/scenario1-rdb.js
```

在 Grafana 觀察：
1. 打開 Dashboard: "Leaderboard Performance - RDB Only"
2. 觀察 P95 延遲指標，特別是 GET /v1/scores/{user_id}

### (可選) 增加更多測試資料

如果想要測試 **100,000 用戶**的極端場景：

```bash
# Terminal 2: 增加額外 50,000 用戶 (player_50001 ~ player_100000)
k6 run k6/init-data.js  # 需要約 15-20 分鐘

# 再次執行負載測試，觀察性能惡化
k6 run k6/scenario1-rdb.js
```

## 預期效果

### 基於 50,000 用戶：
- ✅ POST /v1/scores - 快速（~30-50ms）
- ⚠️ GET /v1/scores - 中等（~200-500ms）
- ❌ GET /v1/scores/{user_id} - **很慢（~500-2000ms）**

### 基於 100,000 用戶：
- ✅ POST /v1/scores - 快速（~30-50ms）
- ⚠️ GET /v1/scores - 慢（~500-1000ms）
- ❌ GET /v1/scores/{user_id} - **極慢（~2000-5000ms）**

**這證明了使用 SQL 來做排行榜的性能問題會隨著資料量增長而惡化！**

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
