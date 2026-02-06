# Data Structures Deep-Dive

## Order Book (HashMap + Doubly-Linked List)

The order book is the most performance-critical data structure in a stock exchange. Our implementation follows the design described in pages 395-397 (Figure 13.14).

### Structure

```
OrderBook
├── BuyBook (bids)
│   └── LimitMap: map[price] → PriceLevel
│       └── PriceLevel
│           ├── Price: int64
│           ├── TotalVolume: int64
│           └── Orders: doubly-linked list of *Order (FIFO)
├── SellBook (asks)
│   └── (same structure as BuyBook)
└── OrderMap: map[orderID] → orderEntry
    └── orderEntry
        ├── *Order
        ├── *list.Element  (pointer into the linked list)
        └── *bookLevel     (which price level this order belongs to)
```

### Why This Design?

| Operation | Time Complexity | How |
|---|---|---|
| **Add order** | O(1) | HashMap lookup for price level → append to linked list tail |
| **Cancel order** | O(1) | OrderMap gives direct pointer to list.Element → remove from linked list |
| **Match (per fill)** | O(1) | Best price tracking → front of linked list at best price level |
| **L2 snapshot** | O(n log n) | Sort price levels (n = number of distinct prices, typically small) |

### FIFO Matching

At each price level, orders are maintained in a doubly-linked list. New orders append to the **tail**. Matching consumes from the **head**. This guarantees First-In-First-Out (FIFO) price-time priority.

```
Price Level $100.10:
  HEAD → [Order A, qty 100] → [Order B, qty 200] → [Order C, qty 300] → TAIL
         ↑ matched first        matched second        matched last
```

### Best Price Tracking

- **Best bid** = highest buy price (buyers want to pay as much as possible to get filled)
- **Best ask** = lowest sell price (sellers want to sell at the lowest competitive price)

The spread = best ask - best bid. When a new order's price crosses the spread, matching occurs.

---

## Candlestick Ring Buffer

Market data candlesticks use a **ring buffer** (circular array) to maintain a fixed-size window of recent OHLCV data.

### Structure

```
RingBuffer (capacity = 100)
┌────┬────┬────┬────┬────┬─────┬────┬────┐
│ C0 │ C1 │ C2 │ .. │C98 │ C99 │ C0'│ .. │
└────┴────┴────┴────┴────┴─────┴────┴────┘
                              ↑
                            head (next write position)
```

### Properties

| Property | Value |
|---|---|
| **Capacity** | 100 candles |
| **Push** | O(1) — write at head, advance head |
| **Memory** | Fixed — no allocations after initialization |
| **Overflow** | Oldest candle is overwritten (natural expiry) |

### Candlestick Building

For each symbol, we maintain a "building" candle that accumulates trades during the current interval:

```
Execution arrives (price=10010, qty=100):
  - If first trade in interval: Open=10010, High=10010, Low=10010, Close=10010, Volume=100
  - Otherwise: update High/Low if needed, Close=10010, Volume += 100

On interval tick (every 1 minute):
  - Push current candle to ring buffer
  - Reset building state for next interval
```

### OHLCV Fields

- **Open**: Price of the first trade in the interval
- **High**: Highest trade price in the interval
- **Low**: Lowest trade price in the interval
- **Close**: Price of the last trade in the interval
- **Volume**: Total quantity of shares traded in the interval
