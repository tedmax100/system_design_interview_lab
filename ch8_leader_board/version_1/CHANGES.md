# Version 1 更新說明

## 🎯 主要改進

### 1. PostgreSQL 預填充 50,000 筆資料

**位置**: `k8s/postgresql/deployment.yaml`

**改動內容**:
```sql
-- 自動建立 50,000 個用戶
INSERT INTO users (user_id, username)
SELECT 'player_' || i, 'Player ' || i
FROM generate_series(1, 50000) AS i;

-- 建立排行榜記錄（分數 1-1000 隨機）
INSERT INTO monthly_leaderboard (user_id, score, month)
SELECT
  'player_' || i,
  floor(random() * 1000 + 1)::int,
  TO_CHAR(CURRENT_DATE, 'YYYY-MM')
FROM generate_series(1, 50000) AS i;

-- 建立約 25M 筆歷史記錄
INSERT INTO score_history (user_id, match_id, points)
SELECT
  ml.user_id,
  ml.user_id || '_match_' || s,
  1
FROM monthly_leaderboard ml
CROSS JOIN LATERAL generate_series(1, ml.score) AS s;
```

**效果**:
- ✅ 部署完成後立即有 50,000 用戶資料
- ✅ 可以直接執行負載測試，無需等待資料初始化
- ✅ 足夠的資料量展示 PostgreSQL 性能問題

### 2. 調整 PostgreSQL 資源配置

**改動**:
```yaml
resources:
  requests:
    memory: "512Mi"   # 原: 256Mi
    cpu: "500m"       # 原: 250m
  limits:
    memory: "1Gi"     # 原: 512Mi
    cpu: "1000m"      # 原: 500m
```

**原因**:
- 需要處理 50,000 用戶 + 25M 歷史記錄
- 初始化時需要更多記憶體和 CPU

### 3. 修改 k6/init-data.js

**位置**: `k6/init-data.js`

**改動內容**:
- 從 `1,000 用戶` 改為 `50,000 用戶`
- 用戶 ID 範圍: `player_50001` ~ `player_100000` (避免與預填充資料衝突)
- 使用 `shared-iterations` executor，50 個並發
- 新增進度顯示

**用途**:
- 可選擇性將資料量從 50K 增加到 100K
- 演示性能隨資料量增長而惡化

### 4. 修改 k6/scenario1-rdb.js

**位置**: `k6/scenario1-rdb.js`

**改動內容**:
```javascript
// 原: user_${Math.floor(Math.random() * 10000)}
// 新: player_${Math.floor(Math.random() * 50000) + 1}
function randomUser() {
  const userId = `player_${Math.floor(Math.random() * 50000) + 1}`;
  return userId;
}
```

**效果**:
- 負載測試會從 50,000 個現有用戶中隨機選擇
- 更真實的測試場景

### 5. 更新部署腳本 (deploy.sh)

**位置**: `version_1/deploy.sh`

**新增內容**:
```bash
# 延長 PostgreSQL 初始化等待時間
kubectl wait --timeout=300s  # 原: 180s

# 新增初始化進度提示
echo "PostgreSQL is initializing..."
echo "This will take 5-8 minutes because:"
echo "  1. Creating database schema"
echo "  2. Inserting 50,000 users"
echo "  3. Creating leaderboard records"
echo "  4. Generating ~25M match history records"

# 新增驗證步驟
for i in {1..10}; do
  if kubectl exec postgresql-0 -- psql -c "SELECT COUNT(*) FROM users;" > /dev/null 2>&1; then
    echo "✅ Database initialization complete!"
    break
  fi
  sleep 30
done
```

### 6. 更新文檔

**新增文件**:
- ✅ `version_1/DATA_SCALE.md` - 資料規模與性能分析
- ✅ `version_1/CHANGES.md` - 本文件

**更新文件**:
- ✅ `version_1/QUICKSTART.md` - 更新部署時間和測試流程
- ✅ `version_1/README.md` - 更新預期性能結果

## 📊 預期性能差異

### 改進前（1,000 用戶）

| API | P95 延遲 | 問題程度 |
|-----|---------|---------|
| POST /v1/scores | ~30ms | ✅ 正常 |
| GET /v1/scores | ~50ms | ✅ 正常 |
| GET /v1/scores/{user_id} | ~50ms | ⚠️ 問題不明顯 |

**問題**: 資料量太小，看不出 PostgreSQL 的性能瓶頸

### 改進後（50,000 用戶）

| API | P95 延遲 | 問題程度 |
|-----|---------|---------|
| POST /v1/scores | ~50ms | ✅ 正常 |
| GET /v1/scores | ~200-500ms | ❌ **超標 4-10x** |
| GET /v1/scores/{user_id} | ~500-2000ms | ❌ **超標 5-20x** |

**效果**: 清楚展示 COUNT(*) 和 RANK() OVER 的性能問題

### 極端場景（100,000 用戶）

使用 `k6 run k6/init-data.js` 增加資料後：

| API | P95 延遲 | 惡化程度 |
|-----|---------|---------|
| POST /v1/scores | ~50ms | 1x (不變) |
| GET /v1/scores | ~500-1000ms | **2x 惡化** |
| GET /v1/scores/{user_id} | ~2000-5000ms | **2.5x 惡化** |

## 🎓 教學價值提升

### 改進前的問題

1. ❌ 部署後還要等 5 分鐘初始化資料
2. ❌ 1,000 用戶看不出明顯性能問題
3. ❌ 需要手動調整測試腳本
4. ❌ 沒有清楚的資料規模說明

### 改進後的優勢

1. ✅ **立即可測**: 部署完就有 50,000 筆資料
2. ✅ **問題明顯**: P95 延遲超標 5-20 倍
3. ✅ **可擴展**: 可選擇性增加到 100K 展示惡化
4. ✅ **文檔完整**: DATA_SCALE.md 詳細解釋性能分析

## 🚀 使用流程

### 快速測試（推薦）

```bash
# 1. 部署（8-10 分鐘）
cd version_1
./deploy.sh

# 2. Port-forward
kubectl port-forward -n leaderboard svc/leaderboard-service-rdb 8080:80

# 3. 直接測試（資料已準備好）
k6 run k6/scenario1-rdb.js

# 4. 觀察 Grafana
open http://localhost:30300
```

### 深度測試（可選）

```bash
# 1-3 同上

# 4. 增加到 100K 用戶
k6 run k6/init-data.js  # 約 15-20 分鐘

# 5. 再次測試，觀察性能惡化
k6 run k6/scenario1-rdb.js

# 6. 對比前後差異
```

## 📝 相關文件

- [DATA_SCALE.md](./DATA_SCALE.md) - 資料規模與性能詳細分析
- [QUICKSTART.md](./QUICKSTART.md) - 快速啟動指南
- [README.md](./README.md) - 完整使用文檔
- [STRUCTURE.md](./STRUCTURE.md) - 專案結構說明

## 🔍 驗證方法

### 1. 檢查資料是否正確初始化

```bash
kubectl exec -n leaderboard postgresql-0 -- psql -U postgres -d leaderboard -c "SELECT COUNT(*) FROM users;"
# 應該返回: 50000

kubectl exec -n leaderboard postgresql-0 -- psql -U postgres -d leaderboard -c "SELECT COUNT(*) FROM monthly_leaderboard;"
# 應該返回: 50000

kubectl exec -n leaderboard postgresql-0 -- psql -U postgres -d leaderboard -c "SELECT COUNT(*) FROM score_history;"
# 應該返回: ~25,000,000
```

### 2. 檢查資料分布

```bash
kubectl exec -n leaderboard postgresql-0 -- psql -U postgres -d leaderboard -c "
SELECT
  MIN(score) as min_score,
  MAX(score) as max_score,
  AVG(score)::int as avg_score,
  COUNT(*) as total_users
FROM monthly_leaderboard;
"
```

預期輸出：
```
 min_score | max_score | avg_score | total_users
-----------+-----------+-----------+-------------
         1 |      1000 |       500 |       50000
```

### 3. 檢查 Top 10

```bash
kubectl exec -n leaderboard postgresql-0 -- psql -U postgres -d leaderboard -c "
SELECT user_id, score
FROM monthly_leaderboard
ORDER BY score DESC
LIMIT 10;
"
```

## 🎯 總結

這次改進將 Version 1 的教學效果提升到了最佳狀態：

1. ✅ **部署即可測** - 不需要額外等待資料初始化
2. ✅ **問題明顯** - 50K 用戶足以展示 PostgreSQL 瓶頸
3. ✅ **時間合理** - 部署 8-10 分鐘可接受
4. ✅ **可擴展性** - 支援增加到 100K 展示惡化趨勢
5. ✅ **文檔完整** - 詳細的性能分析和使用說明

現在可以自信地向任何人展示「為什麼 PostgreSQL 不適合排行榜」！
