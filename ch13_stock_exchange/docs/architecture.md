# Architecture Overview

## Single-Server Low-Latency Design

This lab implements the **single-server stock exchange architecture** from Chapter 13 (Figure 13.15). All critical-path components run in-memory within a single process, communicating via Go channels (simulating ring buffers/mmap).

## Component Diagram

```
                    ┌─────────────────────────────────────────────┐
                    │              Single Server Process           │
                    │                                             │
   HTTP Request     │  ┌───────────┐    ┌──────────────┐         │
  ──────────────────┼─►│  API      │───►│  Order       │         │
                    │  │  Handler  │    │  Manager     │         │
                    │  └───────────┘    │  (risk check │         │
                    │                   │   + wallet)  │         │
                    │                   └──────┬───────┘         │
                    │                          │                  │
                    │                   [chan OrderEvent]          │
                    │                          │                  │
                    │                   ┌──────▼───────┐         │
                    │                   │  Sequencer   │         │
                    │                   │  (seq IDs)   │         │
                    │                   └──────┬───────┘         │
                    │                          │                  │
                    │                   ┌──────▼───────┐         │
                    │                   │  Matching    │         │
                    │                   │  Engine      │         │
                    │                   └──────┬───────┘         │
                    │                          │                  │
                    │                  [chan ExecutionEvent]       │
                    │                     ┌────┴────┐             │
                    │              ┌──────▼──┐  ┌───▼──────────┐ │
                    │              │  Order  │  │  Market Data │ │
                    │              │  Manager│  │  Publisher   │ │
                    │              │ (settle)│  │ (candles/L2) │ │
                    │              └─────────┘  └──────────────┘ │
                    └─────────────────────────────────────────────┘
```

## Data Flow

### Trading Flow (Critical Path)
1. Client sends order via REST API
2. **Order Manager** validates: risk check (daily volume limit), wallet check (sufficient funds/shares)
3. **Sequencer** stamps monotonically increasing sequence ID
4. **Matching Engine** executes FIFO matching against the order book
5. Execution events fan out to Order Manager (settlement) and Market Data Publisher

### Market Data Flow
1. **Market Data Publisher** receives execution events
2. Updates per-symbol candlestick (OHLCV) using ring buffer
3. Candlestick rotation happens every 1 minute
4. L2 order book snapshots served directly from the matching engine

### Channel Pipeline
```
API Handler → [chan Order] → Order Manager → [chan Order] → Sequencer → Matching Engine
                                                                ↓
Market Data Publisher ← [chan Execution] ← Fan-out ← Sequencer
Order Manager         ← [chan Execution] ←
```

## Key Design Decisions

| Decision | Rationale |
|---|---|
| **int64 prices (cents)** | Avoid floating-point precision issues |
| **Go channels** | Simulate ring buffers/mmap from the book's design |
| **Single matching goroutine** | Deterministic FIFO matching, no locking needed |
| **In-memory wallets** | Lab simplicity; production would use persistent store |
| **HashMap + doubly-linked list** | O(1) add/cancel, FIFO matching at each price level |
| **Ring buffer for candles** | Fixed memory, O(1) push, natural expiry of old data |
