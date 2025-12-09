# Version 1 專案結構說明

## 完整檔案清單

```
ch10_leader_board/
├── version_1/
│   ├── README.md           # 詳細說明文檔
│   ├── QUICKSTART.md       # 快速啟動指南
│   ├── STRUCTURE.md        # 本文件
│   ├── deploy.sh           # 自動化部署腳本
│   └── test-api.sh         # API 測試腳本
│
├── src/                    # Go 應用程式原始碼
│   ├── go.mod              # Go 模組定義
│   ├── go.sum              # Go 依賴鎖定
│   ├── cmd/
│   │   └── main.go         # 應用程式入口點
│   └── internal/
│       ├── config/
│       │   └── config.go   # 配置管理
│       ├── handler/
│       │   └── handler.go  # HTTP handlers
│       ├── middleware/
│       │   └── metrics.go  # Prometheus metrics
│       └── repository/
│           └── postgres.go # PostgreSQL 資料存取層
│
├── k8s/                    # Kubernetes 配置檔
│   ├── namespace.yaml      # 命名空間定義
│   ├── postgresql/
│   │   └── deployment.yaml # PostgreSQL StatefulSet + Service
│   ├── app/
│   │   ├── deployment.yaml      # 應用程式 Deployment + Service
│   │   └── servicemonitor.yaml  # Prometheus ServiceMonitor
│   └── monitoring/
│       └── grafana-dashboard.yaml # Grafana Dashboard
│
├── k6/                     # 負載測試腳本
│   ├── init-data.js        # 測試資料初始化
│   └── scenario1-rdb.js    # Version 1 負載測試場景
│
├── docker/
│   └── Dockerfile.app      # 應用程式 Docker 映像
│
└── article.md              # 系統設計文章

```

## 核心元件說明

### 1. Go 應用程式 (src/)

#### main.go
- 應用程式入口點
- 設置 HTTP 路由
- 連接 PostgreSQL
- 暴露 /metrics 端點給 Prometheus

#### repository/postgres.go
- 實作三個核心功能：
  - `UpdateScore()`: 更新用戶分數（含冪等性檢查）
  - `GetTopN()`: 取得前 N 名（展示 RANK() OVER 的性能問題）
  - `GetUserRank()`: 查詢用戶排名（展示 COUNT(*) 的性能問題）

#### handler/handler.go
- HTTP API handlers：
  - `POST /v1/scores`: 更新分數
  - `GET /v1/scores`: 取得 Top 10
  - `GET /v1/scores/{user_id}`: 查詢用戶排名

#### middleware/metrics.go
- 收集 Prometheus metrics：
  - `http_requests_total`: 請求總數
  - `http_request_duration_seconds`: 請求延遲
  - 自訂 buckets 用於更好的可視化

### 2. Kubernetes 配置 (k8s/)

#### postgresql/deployment.yaml
- PostgreSQL 17 StatefulSet
- 包含初始化 SQL：
  - users 表
  - score_history 表（含 match_id 用於冪等性）
  - monthly_leaderboard 表（核心排行榜表）
  - Materialized View（top10_current_month）- 雖然有，但仍然慢

#### app/deployment.yaml
- 兩個 Deployment：
  - `leaderboard-app-rdb`: PostgreSQL only (Version 1)
  - `leaderboard-app-redis`: With Redis (Version 2)
- 對應的 Service

#### app/servicemonitor.yaml
- Prometheus ServiceMonitor
- 讓 Prometheus 自動抓取應用程式的 /metrics

#### monitoring/grafana-dashboard.yaml
- ConfigMap 包含 Grafana Dashboard JSON
- 視覺化關鍵指標：
  - P50/P95/P99 延遲
  - Request Rate
  - 各 API 的 P95 延遲 Gauge

### 3. 負載測試 (k6/)

#### init-data.js
- 創建 1000 個用戶
- 每個用戶有 1-100 分的隨機分數
- 總共約 5 萬筆資料

#### scenario1-rdb.js
- 模擬真實流量模式：
  - 70% 更新分數
  - 15% 查詢 Top 10
  - 15% 查詢用戶排名
- 逐步提升負載到 150 VUs
- 設定 SLO 閾值：P95 < 50ms

### 4. 部署腳本 (version_1/)

#### deploy.sh
- 自動化部署流程：
  1. 創建 k3d cluster
  2. 部署 PostgreSQL
  3. 構建並部署應用程式
  4. 安裝 Prometheus + Grafana
  5. 部署 Dashboard

#### test-api.sh
- 簡單的 API 功能測試
- 驗證三個 API 端點正常運作

## 關鍵設計決策

### 為何選擇這些技術？

1. **PostgreSQL 17**: 最新版本，展示即使最新的 RDB 也有排行榜性能問題
2. **Go 1.25**: 高性能、容易部署的單一二進制檔
3. **k3d**: 輕量級 Kubernetes，適合本地開發和演示
4. **k6**: 現代化負載測試工具，支援 JavaScript 腳本
5. **Prometheus + Grafana**: 業界標準的監控解決方案

### 為何這樣設計資料庫 Schema？

```sql
-- monthly_leaderboard 表
CREATE TABLE monthly_leaderboard (
  user_id VARCHAR(50) NOT NULL,
  score INTEGER NOT NULL DEFAULT 0,
  month VARCHAR(7) NOT NULL,
  PRIMARY KEY (user_id, month)
);

CREATE INDEX idx_monthly_score ON monthly_leaderboard(month, score DESC);
```

- **複合主鍵** (user_id, month): 支援多賽季
- **索引** (month, score DESC): 嘗試優化排序查詢
- **問題**: 即使有索引，RANK() 和 COUNT(*) 仍需全表掃描

### 為何保留 Materialized View？

雖然 Materialized View 可以快取 Top 10，但：
- 每次更新都需要 REFRESH（觸發器）
- 在高併發寫入時會成為瓶頸
- 目的是展示「即使優化，RDB 仍有限制」

## 性能基準預期

基於 5 萬筆資料（1000 users × 平均 50 wins）：

| API | 預期 P95 延遲 | 說明 |
|-----|--------------|------|
| POST /v1/scores | ~30-50ms | ✅ INSERT ... ON CONFLICT 很快 |
| GET /v1/scores | ~100-200ms | ⚠️ RANK() OVER 需要排序全表 |
| GET /v1/scores/{user_id} | ~500-2000ms | ❌ COUNT(*) 需要掃描全表 |

## 如何驗證性能問題？

1. 部署系統並初始化資料
2. 在 PostgreSQL 中執行 EXPLAIN ANALYZE
3. 觀察執行計畫中的 Seq Scan
4. 執行 k6 負載測試
5. 在 Grafana 看到延遲飆升

## 下一步

Version 2 將使用 Redis Sorted Set 替換核心查詢邏輯：
- `ZINCRBY`: 更新分數 O(log N)
- `ZREVRANGE`: 取得 Top 10 O(log N + 10)
- `ZREVRANK`: 查詢排名 O(log N)

預期性能提升：
- GET /v1/scores: 200ms → 5ms (40x 改善)
- GET /v1/scores/{user_id}: 2000ms → 10ms (200x 改善)
