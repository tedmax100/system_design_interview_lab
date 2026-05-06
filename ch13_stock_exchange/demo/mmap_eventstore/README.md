# mmap Event Store demo

可執行的最小範例，用來呈現 `ch13.md` 描述的 **單一生產者、多消費者
（SPMC）mmap 環形緩衝區** 設計（重點請看 1014–1107 與 1141 行）。
三個 Go process 共享 `/dev/shm` 上的同一塊記憶體區域，熱路徑上完全
**沒有 syscall、沒有 lock、沒有複製**。

```
                       /dev/shm/exchange_events  (mmap, MAP_SHARED)
                                  │
   ┌──────────────┐               │
   │  sequencer   │ ──── publish ─┤
   │ (1 writer)   │               │
   └──────────────┘               │
                                  ├──── pull ───→ ┌──────────────┐
                                  │               │   matching   │
                                  │               └──────────────┘
                                  └──── pull ───→ ┌──────────────┐
                                                  │  marketdata  │
                                                  └──────────────┘
```

每個 consumer 自己維護 `readSeq` offset；producer 完全不知道有誰在訂閱。
新增第三個 consumer 只要 `Open()` 一下，**producer 不需要任何改動**。

## 為什麼要用 mmap，不用 Go channel？

Channel 只能在**同一個 process 內**運作。ch13 的設計把系統切成多個獨立
OS process（Matching Engine、Market Data Publisher、Reporter、Warm
Instance — 詳見 ch13.md:1094–1101），這樣某個元件當掉時不會把其他元件
一起拖垮。把 mmap 檔放在 tmpfs（`/dev/shm`）上，讓不相干的 process
共享記憶體——延遲就只是「寫入記憶體」的成本，沒有 kernel 複製、沒有
socket、沒有跨 process 序列化。

>  tmpfs 純活在記憶體的檔案系統實做

## 記憶體佈局（Layout）

先忘掉 mmap、atomic 那些抽象詞。問題其實很單純：「我有一塊 16 MB 的記憶體，
怎麼把它**當成訊息佇列**用？」

### 用「公佈欄」來想像

想像一塊**公佈欄**，要拿它來貼訊息給很多人看：

```
公佈欄 (16 MB):
┌───────────────────────────────────────────────────┐
│  [封面頁]  這是「交易所事件公佈欄」                │
│            總共 65536 格，每格 256 bytes           │
├───────────────────────────────────────────────────┤
│  第 0 格   (空的)                                  │
├───────────────────────────────────────────────────┤
│  第 1 格   (空的)                                  │
├───────────────────────────────────────────────────┤
│  ...                                              │
├───────────────────────────────────────────────────┤
│  第 65535 格 (空的)                                │
└───────────────────────────────────────────────────┘
```

- **「封面頁」就是 Header** — 告訴你這個公佈欄是什麼、有幾格、每格多大
- **「每一格」就是 Slot** — 一格放一筆事件（一張訂單、一筆成交）

整份 layout 就只是 **「封面頁 + 一堆格子」**。

### 為什麼需要 Header（封面頁）？

當一個 process 第一次打開 `/dev/shm/exchange_events`，它怎麼知道：

- 「這檔案是不是給我用的？」 → 看 **magic**（魔數），是 `EXCHGENT` 我才認
- 「裡面有幾格？」 → 看 **slotCount**
- 「每格多大？」 → 看 **slotSize**
- 「目前 producer 寫到哪了？」 → 看 **writeSeq**（最新的 seq）

如果沒有 Header，每個 process 都要把這些參數寫死在程式碼裡。一改參數所有
process 都要重編。**Header 讓檔案「自我描述」**。

```
Header (公佈欄的封面，剛好 64 bytes):
┌──────────────────────────────────────────────┐
│ magic     = "EXCHGENT"  ← "你打開的是這格式" │
│ slotCount = 65536       ← "我有 65536 格"    │
│ slotSize  = 256         ← "每格 256 bytes"   │
│ writeSeq  = 12345       ← "我寫到第 12345 筆"│
│ (padding 32 bytes)                           │
└──────────────────────────────────────────────┘
```

### 為什麼需要 Slot（一格）？

如果**沒有固定格子**，事件直接一筆接一筆塞進去：

```
[event1 (32B)][event2 (47B)][event3 (15B)][event4 (60B)]...
```

要找「第 100 筆事件」就要從頭一路掃 99 筆計算 offset，**O(n)**，超慢。

固定大小的格子讓我們可以**直接算地址**：

```
slot[N] 在哪？
位置 = 64 (Header) + N × 256
```

第 100 筆？`64 + 100 × 256 = 25664` byte。**O(1)，一個乘法搞定**。

每一格的內部結構：

```
Slot (一格 = 256 bytes):
┌──────────────────────────────────────────┐
│ seq        (8 B)  ← "這格放的是第幾筆?"  │
│ eventType  (4 B)  ← "1=NewOrder, 2=Fill" │
│ payloadLen (4 B)  ← "payload 有多長?"    │
│ payload    (240 B) ← 真正的事件內容      │
└──────────────────────────────────────────┘
8 + 4 + 4 + 240 = 256 bytes
```

`seq = 0` 代表這格還沒人寫過（空的）。Producer 寫第一筆時把 seq 設成 1、第二筆設 2…
consumer 看到 seq 不再是 0 就知道有東西可讀。

### 用「4 格的小例子」看真實 bytes

把 slotCount 縮成 **4 格**（方便視覺化），總大小 = 64 + 4 × 256 = 1088 bytes。

**剛建立、什麼都沒寫**：

```
offset    bytes (16 進位)                                說明
─────────────────────────────────────────────────────────────────
0x000     54 4E 45 47 48 43 58 45  ← magic ("EXCHGENT")
0x008     04 00 00 00 00 00 00 00  ← slotCount = 4
0x010     00 01 00 00 00 00 00 00  ← slotSize = 256
0x018     00 00 00 00 00 00 00 00  ← writeSeq = 0 (還沒寫過)
0x020     00 ... 00 (32 bytes)     ← Header padding
─────────────────────────────────────────────────────────────────  ← Header 結束 (64B)
0x040     00 ... (整格 256 B 全 0) ← Slot[0]，空
0x140     00 ... (整格 256 B 全 0) ← Slot[1]，空
0x240     00 ... (整格 256 B 全 0) ← Slot[2]，空
0x340     00 ... (整格 256 B 全 0) ← Slot[3]，空
```

**Producer publish 第一筆**（NewOrder，seq = 1，slot index = `1 & 3` = **1**）：

```
offset    bytes                                          說明
─────────────────────────────────────────────────────────────────
0x000     54 4E 45 47 48 43 58 45  ← magic 不變
0x008     04 00 ...                ← slotCount 不變
0x010     00 01 ...                ← slotSize 不變
0x018     01 00 00 00 00 00 00 00  ← writeSeq = 1   ★ 更新
─────────────────────────────────────────────────────────────────
0x040     00 ...                   ← Slot[0] 還是空
─────────────────────────────────────────────────────────────────
0x140     01 00 00 00 00 00 00 00  ← Slot[1].seq = 1 ★ 寫入！
0x148     01 00 00 00              ← Slot[1].eventType = 1 (NewOrder)
0x14C     29 00 00 00              ← Slot[1].payloadLen = 41
0x150     [41 bytes 的 NewOrder 編碼]  ← Slot[1].payload
```

Consumer 看到 `Slot[1].seq` 從 0 變成 1，就知道：「第 1 筆事件來了，在這格」，
把 payload 解出來就好。

### 連起來看一次

```
┌─────────────────────────────────────────────────────────────┐
│ Header (64B) — 公佈欄的封面                                 │
│  ├── 我是誰：EXCHGENT 格式                                  │
│  ├── 我有幾格：65536                                        │
│  ├── 每格多大：256 B                                        │
│  └── Producer 進度：寫到第 N 筆                             │
├─────────────────────────────────────────────────────────────┤
│ Slot[0] — 第 0 筆事件的「位子」                             │
│  └── seq=? 是不是空？eventType, payloadLen, payload         │
├─────────────────────────────────────────────────────────────┤
│ Slot[1] — 第 1 筆事件的「位子」                             │
├─────────────────────────────────────────────────────────────┤
│ Slot[2] — 第 2 筆事件的「位子」                             │
├─────────────────────────────────────────────────────────────┤
│ ...                                                         │
└─────────────────────────────────────────────────────────────┘
```

> **Header = 公佈欄的封面**（這個公佈欄是什麼）
> **Slot = 公佈欄的一格**（一格放一筆事件）
> **mmap = 把這個公佈欄做在 RAM 裡，多個 process 共看同一份**

預設 slotCount = 65 536，所以檔案大小 = 64 + 65 536 × 256 = **16 MB**。

### 為什麼要這樣設計？

**1. Header 補到 64 B（cache-line aligned）**

x86 / arm64 的 CPU cache 一次抓 64 byte（一條 cache line）。讓 Header 剛好
= 64 B，就保證它**獨佔一條 cache line**，不會跟 Slot[0] 共享。為什麼重要？
**False sharing** — 如果 Header 跟 Slot[0] 在同一條 cache line：

- Producer 更新 `Header.WriteSeq` → 這條 cache line 在 Producer 的 CPU 變
  Modified 狀態
- Consumer 想讀 `Slot[0].Seq` → 這條 cache line 必須從 Producer 的 CPU 用
  cache coherence protocol 同步過來
- **明明讀寫的是不同欄位，卻互相搶同一條 cache line**，效能掉好幾倍

**2. slotCount 必須是 2 的次方**

把 seq 對應到 slot index 的算式：

```
slotIndex = seq & (slotCount - 1)        // 例：seq=12345, slotCount=65536
                                         //     → 12345 & 65535 = 12345
```

只是一個 AND 指令（1 個 CPU cycle）。若 slotCount 不是 2 的次方就得用
`seq % slotCount`，那是除法，20+ cycles。在 µs 級系統裡這個差距很大。

**3. 為什麼 slot 是固定 256 B？**

也是 cache line 對齊（4 條 = 256 B）+ 簡化 indexing。如果 slot 大小可變：

- index 不能直接算（要先掃過前面所有 slot 才知道 offset）
- 不同 slot 跨在不同 cache line 邊界，讀取要多一次 fetch

固定大小換來「乘法直接定位」：`slot[N]` 的地址 = `base + 64 + N × 256`。

**4. Slot 內 `seq` 放最前面**

Consumer 第一個動作就是讀 `seq`（決定這格有沒有資料）。把它放在 offset 0，
讀整個 slot 時 `seq` 跟 payload 在同一條 cache line，一起被 prefetch 進來。

### 環形（ring）怎麼運作？

```
seq:      1     2     3   ...  65535  65536  65537  65538
slotIdx:  1     2     3   ...  65535    0      1      2     ← 繞回去！
```

seq=65537 會寫進 slot[1]，蓋掉原本 seq=1 的內容。**這是設計而非 bug** —
buffer 容量本來就是 65 536 筆，超過就覆蓋舊資料。Consumer 必須跟得上，否則
就會遇到下一節的 **overrun**。

## 並行協定（Concurrency protocol）

這節是整份程式碼的核心。我們要在**沒有 lock** 的前提下，讓多個 process
透過共享記憶體通訊還能保證**正確性**。

### 我們要避免什麼問題

最直觀的「先寫資料、再標記」寫法（**錯的**）：

```go
// Producer
slot.Payload = payload
slot.EventType = eventType
slot.Seq = nextSeq    // 普通寫入，不是 atomic

// Consumer
if slot.Seq == expected {
    use(slot.Payload)   // ← 可能讀到舊的 payload！
}
```

為什麼會壞？因為**「我寫的順序」不等於「別人看到的順序」**。亂序來自三個層級：

| 層級        | 來源                                                                    | 例子                                                            |
| ----------- | ----------------------------------------------------------------------- | --------------------------------------------------------------- |
| 1. Compiler | 編譯器最佳化會重排 store                                                | 看到三個 store 沒有資料依賴，可能先寫 `slot.Seq` 再寫 payload |
| 2. CPU      | store 進 store buffer 後不一定立即 flush                                | x86 是 TSO（store-store 不亂序，但 store-load 會）；arm64 更鬆  |
| 3. Cache    | 別的 CPU 透過 cache coherence 看到的更新順序，可能跟原 CPU 寫的順序不同 | 多 socket 機器更明顯                                            |

結果：consumer 可能看到 `slot.Seq` 已經是新的，但 `slot.Payload` 還是舊的。
在交易所的場景下，這會讓 matching engine 撮合到錯的訂單。

### 解法：release-acquire 一對動作

**Producer**（單一 writer，這是底線）：

```go
// step 1：寫 payload (這幾行之間順序可以隨意)
copy(slot.Payload[:], payload)
slot.EventType = eventType
slot.PayloadLen = uint32(len(payload))

// step 2：release store — 「發布」的瞬間
atomic.StoreUint64(&slot.Seq, nextSeq)
```

`atomic.StoreUint64` 在 Go runtime 是 **release store**。它的契約是：

> **任何在它「之前」的 store**（不管 atomic 還是普通）— 對「之後 acquire
> 載入到這個值」的另一個 thread 都保證可見。

換句話說，consumer **一旦讀到新的 seq，就保證能看到 payload**；不會看到
seq 已更新但 payload 還沒寫完的中間狀態。

**Consumer**：

```go
got := atomic.LoadUint64(&slot.Seq)   // acquire load
if got == expected {
    // 因為剛才是 acquire load，下面這幾行讀取
    // 都保證能看到 producer 寫進去的最新值
    payload := slot.Payload[:slot.PayloadLen]
    handle(payload)
}
```

### 一個具體 timeline

```
時間 →
Producer (CPU 0)                  │ Consumer (CPU 1)
                                  │
  copy(slot.Payload, "ABC")       │
  slot.EventType = 1              │   atomic.LoadUint64(&slot.Seq) = 0
  slot.PayloadLen = 3             │   → ReadEmpty，busy-poll 再讀一次
   ↑ 這三個 store 順序、可見時機  │
     都不保證                     │
                                  │
  atomic.StoreUint64(&Seq, 42) ───┼──→ release barrier：保證上面三個
                                  │     store 已經對 CPU 1 可見
                                  │
                                  │   atomic.LoadUint64(&slot.Seq) = 42
                                  │   ↑ acquire barrier：之後讀的
                                  │     Payload / EventType / PayloadLen
                                  │     一定看得到最新值
                                  │
                                  │   payload := slot.Payload[:3] // 安全 ✓
```

關鍵是 `atomic.Store(&Seq, 42)` 跟 `atomic.Load(&Seq)` 這對動作在記憶體裡
**畫了一條線**：線之前 producer 寫的東西，線之後 consumer 讀得到。

### 為什麼不用 mutex？

Mutex 的 lock/unlock 也算 release/acquire，邏輯上能解，但代價：

| 方式                               | 一筆事件的成本                   |
| ---------------------------------- | -------------------------------- |
| `sync.Mutex` Lock+Unlock         | ~25–50 ns（無爭用），有爭用更糟 |
| `atomic.Store` + `atomic.Load` | ~1–2 ns                         |

更糟的是 mutex 跨 process 還要用 robust mutex 處理「持鎖 process 死掉」的
情境，複雜度暴增。Atomic 方案沒這問題 —— consumer 死掉 producer 完全
不在乎。

### 為什麼也不用 CAS？

CAS（compare-and-swap）解決的是 **多個 writer 互相競爭** 的問題。我們是
**單 writer 設計**，每個 seq 對應的 slot 在那一輪只會被它自己寫一次（直到
ring 繞回來），根本沒有競爭，所以 plain `atomic.Store` 就夠了。

### 為什麼 single-writer rule 是底線

如果有兩個 writer 同時呼叫 `Publish(42, ...)`：

- Writer A 把 payload 寫成 "ABC"，然後 `atomic.Store(&Seq, 42)`
- Writer B 把 payload 寫成 "XYZ"，然後 `atomic.Store(&Seq, 42)`

兩人寫的 payload 字串可能交錯（A 寫到一半 B 蓋上去）。最終 slot 裡可能是
"AYC" 之類的垃圾，但 `Seq` 還是 42 看起來合法。Consumer 解碼時就出事。

ch13.md:1141 因此規定：**每個 Event Store 只能有一個 Sequencer**。
`make run` 啟動前用 `pgrep` 檢查就是這個原因 —— 防止你不小心開兩個 sequencer。

### TryRead 三種結果再看一次

有了上面的背景，再回頭看就清楚多了：

| `slot.Seq` vs `expected` | 物理意義                                             | 該做什麼                                  |
| ---------------------------- | ---------------------------------------------------- | ----------------------------------------- |
| `==`                       | 這格 slot 寫的就是我要的事件                         | 讀 payload，readSeq++                     |
| `<`                        | Producer 還沒寫到這個 seq（slot 裡是舊事件，或全 0） | busy-poll 再讀                            |
| `>`                        | Producer 已經繞了 ring 一整圈把這格覆蓋了            | overrun，事件遺失，要 resync 或當錯誤處理 |

`>` 的情況（overrun）只有在 **producer 比 consumer 快 ≥ slotCount 筆**
的時候才會發生。65 536 slot + 7M ev/s 表示 consumer 落後 ≥ 9.4 ms 就會被覆蓋。

## 編譯（Build）

```
make build      # 產生 bin/sequencer、bin/matching、bin/marketdata
make test       # 跑 eventstore 單元測試
```

## 執行（Run）

一個 terminal、一個指令：

```
make run        # 同時啟動三個 process（輸出帶前綴）；Ctrl-C 全部關閉
```

每行輸出會自動加上 `[seq]` / `[mch]` / `[mkt]` 前綴，一眼就能看出是誰
講的話。按 Ctrl-C 會把整個 process group 一起殺掉，並順便清掉
`/dev/shm/exchange_events`。

如果還是不小心留下殘留 process：

```
make ps         # 列出目前還在跑的 demo process
make stop       # 強制殺掉並移除 shm 檔
```

`make run` 啟動前會檢查有無前一輪殘留，**有的話會直接拒絕啟動**——這是為了
避免兩個 sequencer 同時對同一塊 mmap 寫入（違反 single-writer rule，
ch13.md:1141），它們互相覆寫會直接 SIGSEGV。

如果真的想把每個元件分開放在不同 terminal（例如要掛 debugger），舊的
單一 process target 也還在：

```
# Terminal 1
make sequencer

# Terminal 2
make matching

# Terminal 3
make marketdata
```

## 預期輸出

**Paced 模式**（`-rate 50000`，預設）：

```
sequencer:  published seq=8438 (1601 ev/s)         ← 受 ticker 解析度限制
matching:   processed=8263 (+1532/s) overruns=0 avgLat=1.4µs maxLat=21µs
marketdata: processed=8260 (+1530/s) buyVol=204238 sellVol=213392
```

5 萬/秒的目標被 `time.Ticker`（在 cooperative scheduling 下大約只有 1ms
解析度）卡住——這是**時鐘 pacing** 的限制，不是 ring buffer 本身的限制。
真正重要的數字是延遲：mmap 路徑單筆 **約 1–2 µs**。

**Burst 模式**（`./bin/sequencer -burst`）：

```
sequencer:  published seq=7076864 (7076044 ev/s)
matching:   overrun: lost 65536 events, resyncing from seq=...
```

筆電上單執行緒 ~7M events/s。在 65 536 slot 的 buffer 下 consumer 跟不上，
所以會有 overrun。Demo 會偵測並重新同步——實際生產系統會：(1) 把 buffer
配大到能容納最壞情況的延遲；或 (2) 加 backpressure，讓 producer 等最慢
的 consumer 跟上（這就是 LMAX Disruptor 的做法）。

## 與 ch13.md 概念對照

| 書中概念                                      | Demo 中對應位置                                                        |
| --------------------------------------------- | ---------------------------------------------------------------------- |
| Event Store（mmap，ch13.md:1014–1016）       | `eventstore.Ring` over `/dev/shm`                                  |
| 單一 Sequencer（ch13.md:1141）                | `cmd/sequencer` 是唯一 writer                                        |
| 注入 Sequence ID（ch13.md:1157）              | `publishOne` 內的 `nextSeq++`                                      |
| Application Loop / busy poll（ch13.md:1004）  | `cmd/matching -busy`                                                 |
| 多個 consumer 各自維護 offset（ch13.md:1094） | matching、marketdata 各自記 `readSeq`                                |
| SBE 編碼 payload（ch13.md:1067）              | `eventstore/event.go`（手寫 binary，不是真的 SBE）                   |
| CPU pinning（ch13.md:1004–1010）             | **未實作** — 要用 `taskset(1)` 或 cgo 設定 affinity           |
| OrderFilledEvent 分流（ch13.md:1080）         | `matching.handleEvent` 內有骨架但沒實際發出——要做就需要第二個 ring |

## 這個 demo 刻意**沒有**做的事

- **CPU pinning / 專核 busy polling**：書中所謂的 sub-µs 延遲是建立在
  專核 + busy poll 之上的。要更貼近書本的數字，跑 `taskset -c 2 ./bin/matching`
  把 consumer 釘在隔離的 core 上比較。
- **Backpressure**：producer 永遠不會 block；慢的 consumer 一定會被
  overrun。實際系統會把 buffer 配很大（16M slot = 4GB，tmpfs 撐得起），
  或讓 producer 等最慢的 consumer offset。
- **Crash recovery / 從頭 replay**：`Open` 雖然支援 `-from head` 可以從
  buffer 裡還在的最早 seq 開始讀，但這個 buffer 是**環形**的，不是書中
  描述的「永久不可變 Event Log」。真正的 Event Store 還會額外 append-only
  寫一份 log file。
- **真正的 SBE 編碼**：這裡是手寫 little-endian。
- **Hot/Warm 熱備援**（ch13.md:1161）：本 demo 只跑單機。
- **Raft / leader election**（ch13.md:1191）：超出範圍。

## 檔案結構

```
eventstore/ringbuffer.go    SPMC ring buffer + mmap 底層實作
eventstore/event.go         NewOrder / OrderFilled 二進位編解碼
eventstore/ringbuffer_test.go
cmd/sequencer/main.go       單一 producer
cmd/matching/main.go        Consumer 1 — 量測延遲
cmd/marketdata/main.go      Consumer 2 — 聚合買賣量
scripts/run.sh              一鍵啟動三個 process（make run 會呼叫它）
Makefile
```
