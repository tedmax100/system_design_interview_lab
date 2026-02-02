# 系統設計：數位錢包 (Digital Wallet)

設計一個能夠處理高併發轉帳請求的數位錢包後端系統。

## 1. 理解問題與確立設計範圍 (Step 1: Understand the Problem and Establish Design Scope)
功能需求
• 支援兩個數位錢包之間的餘額轉帳 (Balance Transfer) 操作。
• 不需要考慮儲值 (Deposit) 或提款 (Withdrawal) 操作，專注於轉帳。
非功能需求
• 極高吞吐量 (High Throughput)： 需支援 1,000,000 TPS (每秒一百萬次交易)。
• 高可靠性 (Reliability)： 至少 99.99% 的可用性。
• 事務保證 (Transactional Guarantees)： 必須確保資料正確性，支援 ACID 特性。
• 可重現性 (Reproducibility)： 能夠透過重播 (Replay) 資料從頭重建歷史餘額，以進行對帳與審計。
粗略估算 (Back-of-the-envelope Estimation)
• 典型的關聯式資料庫節點約可處理 1,000 TPS。
• 為了達到 1,000,000 TPS，理論上需要 1,000 個節點。
• 考慮到每筆轉帳涉及兩個操作（扣款與存款），實際負載加倍，系統可能需要約 2,000 個節點。

--------------------------------------------------------------------------------
## 2. 提出高階設計方案 (Step 2: Propose High-level Design)
API 設計
採用 RESTful API 慣例。
POST /v1/wallet/balance_transfer
請求參數 (Request Parameters): | 欄位 | 描述 | 類型 | | :--- | :--- | :--- | | from_account | 扣款帳戶 (Debit account) | string | | to_account | 收款帳戶 (Credit account) | string | | amount | 金額 (字串類型以避免精度丟失) | string | | currency | 貨幣類型 (ISO 4217) | string | | transaction_id | 用於去重 (Deduplication) 的 ID | uuid |
方案演進
方案一：基於記憶體的解決方案 (Simple In-memory Solution)
• 使用 Redis 叢集來儲存 <User, Balance> 映射。
• 利用雜湊演算法 (Hashing) 將用戶帳戶分散到不同的 Redis 節點。
• 缺點： 缺乏持久性 (Durability)，一旦節點崩潰資料即丟失，無法滿足金融級別的可靠性要求。
方案二：基於資料庫的分佈式事務 (Database-based Distributed Transaction)
將 Redis 替換為事務型關聯式資料庫，並將資料分片 (Sharding)。跨分片轉帳需要分佈式事務支援。
1. 兩階段提交 (Two-Phase Commit, 2PC) - 低階方案
• 流程： 協調者 (Coordinator) 先要求資料庫「準備 (Prepare)」，收到所有確認後再要求「提交 (Commit)」。
• 缺點：
    ◦ 性能差： 鎖定 (Locks) 資源時間過長，無法支撐百萬級 TPS。
    ◦ 單點故障 (SPOF)： 協調者崩潰可能導致事務卡住。
2. Try-Confirm/Cancel (TC/C) - 高階方案
• 概念： 透過補償事務 (Compensating Transaction) 在應用層實現事務。
• 流程：
    ◦ Try 階段： 預留資源（所有本地事務已提交或取消）。
    ◦ Confirm 階段： 若全部成功，執行確認操作。
    ◦ Cancel 階段： 若有失敗，執行反向操作 (Undo) 進行補償。
• 關鍵組件：
    ◦ 階段狀態表 (Phase Status Table)： 記錄事務 ID、狀態、階段等，用於協調者崩潰後的恢復。
• 挑戰：
    ◦ 不平衡狀態 (Unbalanced State)： 轉帳過程中（A 扣款後、B 存款前），總餘額暫時減少，應用層可見。
    ◦ 有效操作順序： Try 階段必須是「扣款帳戶執行扣款，收款帳戶無操作 (NOP)」，否則可能導致無法回滾。
    ◦ 亂序執行 (Out-of-order Execution)： 需處理網路延遲導致的操作順序顛倒問題。
3. Saga - 線性化分佈式事務
• 概念： 將長事務拆解為一系列有序的本地事務。
• 模式： 本設計採用編排模式 (Orchestration)，由中央協調者控制流程。
• TC/C vs Saga： Saga 操作是線性的 (Linear)，TC/C 可並行。若對延遲不敏感且依賴微服務架構，Saga 是好選擇。

--------------------------------------------------------------------------------
## 3. 深入設計 (Step 3: Design Deep Dive) - 事件溯源 (Event Sourcing)
為了滿足可審計性 (Auditability) 和 可重現性 (Reproducibility)，系統引入事件溯源。
核心概念
• 不只存狀態，存事件： 傳統資料庫存餘額 (State)，事件溯源存導致餘額變化的事件 (Events)。
• 不可變性： 事件列表是不可變的 (Immutable)，透過重播事件可重建任何時間點的狀態。
架構組件
1. 狀態機 (State Machine)：
    ◦ 負責驗證命令 (Command) 並生成事件 (Event)。
    ◦ 應用事件以更新狀態。
    ◦ 必須是確定性的 (Deterministic)： 不能包含隨機性，確保重播結果一致。
2. CQRS (命令查詢職責分離)：
    ◦ 寫入路徑 (Write Path)： 狀態機處理命令，生成並儲存事件。
    ◦ 讀取路徑 (Read Path)： 唯讀狀態機訂閱事件流，建立視圖 (View) 供外部查詢餘額。
3. 基於檔案的儲存 (File-based Storage)：
    ◦ 為了效能，使用本地磁碟的 mmap (記憶體映射檔案) 與追加寫入 (Append-only) 來儲存命令與事件，而非遠端資料庫。
    ◦ 使用 RocksDB 儲存本地狀態以加速讀取。
4. 快照 (Snapshot)：
    ◦ 定期儲存狀態快照，避免每次重啟都從頭重播所有事件，加速恢復。
高可靠性設計 (Reliability)
• Raft 共識演算法 (Consensus)：
    ◦ 單靠本地檔案有單點故障風險。
    ◦ 使用 Raft 將事件列表複製到多個節點 (Leader/Follower)，保證資料不丟失且順序一致。
最終架構流程 (Final Design Workflow)
採用 TCC/Saga 協調者 搭配 分區 Raft 群組。
轉帳流程 (Happy Path):
1. 用戶發送請求至 Saga 協調者。
2. 協調者在 階段狀態表 建立記錄。
3. 第一階段 (扣款 A)：
    ◦ 協調者發送命令至 分區 1 (帳戶 A) 的 Raft Leader。
    ◦ 分區 1 透過 Raft 同步事件，狀態機執行扣款。
    ◦ CQRS 讀取路徑更新狀態，回傳成功給協調者。
4. 第二階段 (存款 C)：
    ◦ 協調者更新階段狀態表。
    ◦ 協調者發送命令至 分區 2 (帳戶 C) 的 Raft Leader。
    ◦ 分區 2 執行相同流程 (Raft -> State Machine -> CQRS)。
5. 完成： 協調者更新階段狀態表為完成，並回應與戶。

--------------------------------------------------------------------------------
## 4. 總結 (Step 4: Wrap Up)
• 本章設計了一個能處理 100 萬 TPS 的數位錢包服務。
• 從簡單的 Redis 方案演進到 分佈式事務 (TC/C) 方案，解決了原子性問題。
• 最終採用 事件溯源 (Event Sourcing) 結合 Raft 與 CQRS，解決了審計、可重現性及高可用性問題。