# 資料規模與性能分析

## 📊 資料規模設計

Version 1 採用 **50,000 用戶**作為預設測試規模，這個數字是經過精心選擇的。

### 為什麼選擇 50,000？

| 資料量 | 初始化時間 | GET user_rank P95 | 適用場景 |
|--------|-----------|------------------|---------|
| 1,000 | ~30s | ~50ms | ❌ 太小，看不出問題 |
| **50,000** | **~5-8min** | **~500-2000ms** | ✅ **甜蜜點：明顯展示問題** |
| 100,000 | ~15-20min | ~2000-5000ms | ⚠️ 極端場景 |
| 500,000 | ~2-3hr | ~10-30s | ⚠️ 太慢，不適合 demo |

## 🏗️ 資料庫預填充策略

### PostgreSQL 初始化時會自動創建：

```sql
-- 1. 50,000 個用戶
INSERT INTO users (user_id, username)
SELECT 'player_' || i, 'Player ' || i
FROM generate_series(1, 50000) AS i;

-- 2. 每個用戶的排行榜記錄（分數 1-1000 隨機）
INSERT INTO monthly_leaderboard (user_id, score, month)
SELECT
  'player_' || i,
  floor(random() * 1000 + 1)::int,
  TO_CHAR(CURRENT_DATE, 'YYYY-MM')
FROM generate_series(1, 50000) AS i;

-- 3. 對應的歷史記錄（每個用戶的 match 數 = 分數）
-- 這會創建約 25,000,000 筆歷史記錄（平均每人 500 分）
INSERT INTO score_history (user_id, match_id, points)
SELECT
  ml.user_id,
  ml.user_id || '_match_' || s,
  1
FROM monthly_leaderboard ml
CROSS JOIN LATERAL generate_series(1, ml.score) AS s;
```

### 總資料量估算

| 表名 | 筆數 | 單筆大小 | 總大小 (估) |
|------|------|---------|-----------|
| users | 50,000 | ~100 bytes | ~5 MB |
| monthly_leaderboard | 50,000 | ~50 bytes | ~2.5 MB |
| score_history | ~25,000,000 | ~80 bytes | **~2 GB** |
| **Total** | | | **~2 GB** |

> 💡 `score_history` 是最大的表，因為它記錄每一場比賽的歷史。

## 🔬 性能惡化分析

### SQL 查詢的時間複雜度

#### 1. GET /v1/scores (Top 10)

```sql
SELECT user_id, score, RANK() OVER (ORDER BY score DESC) as rank
FROM monthly_leaderboard
WHERE month = '2025-10'
ORDER BY score DESC
LIMIT 10;
```

**時間複雜度**: O(N log N)
- 需要對整張表排序
- 即使只取 10 筆，仍需先計算所有人的排名

| 用戶數 | 排序操作 | 預期延遲 |
|--------|---------|---------|
| 1,000 | ~10,000 次比較 | ~10ms |
| 50,000 | ~800,000 次比較 | **~200-500ms** |
| 100,000 | ~1,700,000 次比較 | ~500-1000ms |

#### 2. GET /v1/scores/{user_id} (查詢排名)

```sql
SELECT (SELECT COUNT(*) FROM monthly_leaderboard lb2
        WHERE lb2.score >= lb1.score) AS rank
FROM monthly_leaderboard lb1
WHERE lb1.user_id = 'player_12345';
```

**時間複雜度**: O(N) 甚至 O(N²)
- 需要 COUNT(*) 掃描全表
- 每次查詢都要比較所有其他用戶的分數

| 用戶數 | COUNT(*) 掃描 | 預期延遲 |
|--------|--------------|---------|
| 1,000 | ~1,000 rows | ~5ms |
| 50,000 | ~50,000 rows | **~500-2000ms** |
| 100,000 | ~100,000 rows | **~2000-5000ms** |

## 📈 實際測試結果預期

### Scenario 1: 基礎資料（50,000 用戶）

部署完成後直接測試：

```bash
k6 run k6/scenario1-rdb.js
```

**預期結果**：

| API | Target (SLO) | 實際 P95 | 結果 |
|-----|-------------|---------|------|
| POST /v1/scores | < 50ms | ~30-50ms | ✅ 達標 |
| GET /v1/scores | < 50ms | ~200-500ms | ❌ **超標 4-10x** |
| GET /v1/scores/{user_id} | < 100ms | ~500-2000ms | ❌ **超標 5-20x** |

### Scenario 2: 加倍資料（100,000 用戶）

執行 `k6 run k6/init-data.js` 增加額外 50,000 用戶後：

**預期結果**：

| API | 50K 用戶 | 100K 用戶 | 惡化倍數 |
|-----|---------|----------|---------|
| POST /v1/scores | ~50ms | ~50ms | 1x (不受影響) |
| GET /v1/scores | ~500ms | ~1000ms | **2x 惡化** |
| GET /v1/scores/{user_id} | ~2000ms | ~5000ms | **2.5x 惡化** |

## 🎯 關鍵觀察點

### 在 Grafana Dashboard 中觀察

1. **HTTP Request Duration (Percentiles)** 圖表
   - 注意 P95 和 P99 的延遲
   - 觀察延遲隨時間的變化

2. **GET /v1/scores/{user_id} - P95 Latency** Gauge
   - 這個會顯示紅色（超過 0.5s）
   - 這是性能問題最嚴重的 API

3. **Response Status Distribution**
   - 檢查是否有 timeout 或 500 錯誤

### 在 PostgreSQL 中驗證

```bash
# 進入 PostgreSQL
kubectl exec -it -n leaderboard postgresql-0 -- psql -U postgres -d leaderboard

# 查看執行計畫
EXPLAIN ANALYZE
SELECT user_id, score,
  (SELECT COUNT(*) FROM monthly_leaderboard lb2
   WHERE lb2.score >= lb1.score) AS rank
FROM monthly_leaderboard lb1
WHERE lb1.user_id = 'player_25000';
```

你會看到：
- `Seq Scan on monthly_leaderboard` - **全表掃描**
- `Execution Time: 1500-3000 ms` - **非常慢**

### 查看表的實際大小

```sql
-- 查看各表大小
SELECT
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

預期輸出：
```
 schemaname |      tablename       |  size
------------+----------------------+---------
 public     | score_history        | 2048 MB
 public     | monthly_leaderboard  | 3 MB
 public     | users                | 5 MB
```

## 🔄 漸進式測試建議

### Step 1: 驗證基礎功能（部署後立即測試）

```bash
# 1. 測試 API 功能
./test-api.sh

# 2. 執行輕量負載測試（只跑 1 分鐘）
k6 run --duration 1m k6/scenario1-rdb.js
```

**目的**：確認系統正常運作

### Step 2: 觀察性能問題（完整負載測試）

```bash
# 執行完整 7 分鐘的負載測試
k6 run k6/scenario1-rdb.js
```

**目的**：在 Grafana 中清楚看到性能惡化

### Step 3: (可選) 極端場景測試

```bash
# 增加到 100,000 用戶
k6 run k6/init-data.js  # 約 15-20 分鐘

# 再次測試，對比差異
k6 run k6/scenario1-rdb.js
```

**目的**：展示性能隨資料量線性或指數級惡化

## 💡 為什麼不用更大的資料量？

| 資料量 | 優點 | 缺點 | 適用性 |
|--------|------|------|--------|
| 1,000 | 快速部署 | 看不出問題 | ❌ 不推薦 |
| **50,000** | **問題明顯** | **部署時間可接受** | ✅ **推薦** |
| 500,000 | 問題極端明顯 | 部署太慢 (2-3hr) | ❌ 不實際 |
| 2,500,000 (真實 MAU) | 完全真實場景 | 本地環境無法運行 | ❌ 只能雲端測試 |

## 🎓 教學價值

這個資料規模的設計達到了以下教學目標：

1. ✅ **立即可測**：部署完 8-10 分鐘即可開始測試
2. ✅ **問題明顯**：P95 延遲超標 5-20 倍，在 Grafana 中清晰可見
3. ✅ **可擴展性**：可以選擇性增加到 100K 展示惡化趨勢
4. ✅ **真實感**：50,000 用戶已經是中型應用的規模
5. ✅ **對比鮮明**：與 Version 2 (Redis) 的性能差異會非常明顯

## 📚 相關文件

- [QUICKSTART.md](./QUICKSTART.md) - 快速啟動指南
- [README.md](./README.md) - 完整文檔
- [STRUCTURE.md](./STRUCTURE.md) - 專案結構說明
