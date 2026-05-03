package hmbook_test

import (
	"fmt"
	"testing"

	"github.com/nathanyu/stock-exchange/internal/domain"
	"github.com/nathanyu/stock-exchange/internal/hmbook"
	"github.com/nathanyu/stock-exchange/internal/rbtreebook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newOrder(id string, side domain.Side, price, qty int64) *domain.Order {
	return &domain.Order{
		OrderID:           id,
		Symbol:            "AAPL",
		Side:              side,
		Price:             price,
		Quantity:          qty,
		RemainingQuantity: qty,
		Status:            domain.OrderStatusNew,
		UserID:            "user1",
	}
}

// ── Unit Tests ────────────────────────────────────────────────────────────────

func TestHM_AddSingleSellOrder(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	sell := newOrder("s1", domain.SideSell, 10010, 1000)
	ob.AddOrder(sell)

	assert.True(t, ob.SellBook.HasOrders())
	assert.Equal(t, int64(10010), ob.SellBook.BestPrice())
	assert.Len(t, ob.OrderMap, 1)

	snap := ob.GetL2Snapshot(5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(10010), snap.Asks[0].Price)
	assert.Equal(t, int64(1000), snap.Asks[0].Quantity)
}

func TestHM_AddMultipleOrders_SamePrice(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 500))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10010, 300))

	snap := ob.GetL2Snapshot(5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(800), snap.Asks[0].Quantity)
}

func TestHM_BestPriceTracking(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("b1", domain.SideBuy, 9990, 100))
	ob.AddOrder(newOrder("b2", domain.SideBuy, 10000, 100))
	ob.AddOrder(newOrder("b3", domain.SideBuy, 9980, 100))

	assert.Equal(t, int64(10000), ob.BuyBook.BestPrice(), "best bid = highest buy price")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 100))

	assert.Equal(t, int64(10010), ob.SellBook.BestPrice(), "best ask = lowest sell price")
}

// TestHM_CacheUpdate_AddBetterBid verifies the O(1) cache update on add.
func TestHM_CacheUpdate_AddBetterBid(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("b1", domain.SideBuy, 10000, 100))
	assert.Equal(t, int64(10000), ob.BuyBook.BestPrice())

	// Adding a lower bid should NOT change the best bid
	ob.AddOrder(newOrder("b2", domain.SideBuy, 9990, 100))
	assert.Equal(t, int64(10000), ob.BuyBook.BestPrice(), "lower bid must not change cache")

	// Adding a higher bid SHOULD update the cache
	ob.AddOrder(newOrder("b3", domain.SideBuy, 10010, 100))
	assert.Equal(t, int64(10010), ob.BuyBook.BestPrice(), "higher bid must update cache")
}

// TestHM_CacheUpdate_AddBetterAsk verifies the O(1) cache update on add for sell side.
func TestHM_CacheUpdate_AddBetterAsk(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10020, 100))
	assert.Equal(t, int64(10020), ob.SellBook.BestPrice())

	// Adding a higher ask should NOT change the best ask
	ob.AddOrder(newOrder("s2", domain.SideSell, 10030, 100))
	assert.Equal(t, int64(10020), ob.SellBook.BestPrice(), "higher ask must not change cache")

	// Adding a lower ask SHOULD update the cache
	ob.AddOrder(newOrder("s3", domain.SideSell, 10010, 100))
	assert.Equal(t, int64(10010), ob.SellBook.BestPrice(), "lower ask must update cache")
}

// TestHM_CancelBestPrice_TriggersRescan verifies that cancelling the best-price level
// correctly triggers refreshBestPrice and yields the next-best price.
func TestHM_CancelBestPrice_TriggersRescan(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100)) // best ask
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 100))
	ob.AddOrder(newOrder("s3", domain.SideSell, 10030, 100))

	assert.Equal(t, int64(10010), ob.SellBook.BestPrice())

	// Cancel the best ask — triggers O(n) rescan
	ob.CancelOrder("s1")
	assert.Equal(t, int64(10020), ob.SellBook.BestPrice(), "after cancelling best ask, next level becomes best")
}

// TestHM_CancelNonBestPrice_NorescanNeeded verifies that cancelling a non-best-price
// level does NOT change the cached best price (no rescan).
func TestHM_CancelNonBestPrice_NorescanNeeded(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100)) // best ask
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 100)) // non-best

	// Cancel the non-best ask — best price should NOT change
	ob.CancelOrder("s2")
	assert.Equal(t, int64(10010), ob.SellBook.BestPrice(), "best ask unchanged after non-best cancel")
}

func TestHM_MatchOrder_FullFill(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	sell := newOrder("s1", domain.SideSell, 10010, 1000)
	ob.AddOrder(sell)

	buy := newOrder("b1", domain.SideBuy, 10010, 1000)
	execs := ob.MatchOrder(buy)

	require.Len(t, execs, 1)
	assert.Equal(t, int64(1000), execs[0].Quantity)
	assert.Equal(t, int64(10010), execs[0].Price)
	assert.Equal(t, "s1", execs[0].MakerOrderID)
	assert.Equal(t, "b1", execs[0].TakerOrderID)
	assert.Equal(t, domain.OrderStatusFilled, buy.Status)
	assert.Equal(t, domain.OrderStatusFilled, sell.Status)
	assert.False(t, ob.SellBook.HasOrders())
}

func TestHM_MatchOrder_PartialFill(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	sell := newOrder("s1", domain.SideSell, 10010, 1000)
	ob.AddOrder(sell)

	buy := newOrder("b1", domain.SideBuy, 10010, 200)
	execs := ob.MatchOrder(buy)

	require.Len(t, execs, 1)
	assert.Equal(t, int64(200), execs[0].Quantity)
	assert.Equal(t, domain.OrderStatusFilled, buy.Status)
	assert.Equal(t, domain.OrderStatusPartiallyFilled, sell.Status)
	assert.Equal(t, int64(800), sell.RemainingQuantity)

	snap := ob.GetL2Snapshot(5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(800), snap.Asks[0].Quantity)
}

func TestHM_MatchOrder_MultiplePriceLevels(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 200))

	buy := newOrder("b1", domain.SideBuy, 10020, 300)
	execs := ob.MatchOrder(buy)

	require.Len(t, execs, 2)
	assert.Equal(t, int64(10010), execs[0].Price)
	assert.Equal(t, int64(10020), execs[1].Price)
	assert.Equal(t, domain.OrderStatusFilled, buy.Status)
	assert.False(t, ob.SellBook.HasOrders())
}

// TestHM_MatchOrder_BestPriceCacheAfterSweep verifies the best-price cache
// is correctly updated after each price level is consumed during matching.
func TestHM_MatchOrder_BestPriceCacheAfterSweep(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100)) // best
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 100))
	ob.AddOrder(newOrder("s3", domain.SideSell, 10030, 100))

	assert.Equal(t, int64(10010), ob.SellBook.BestPrice())

	// Match only the first level
	buy := newOrder("b1", domain.SideBuy, 10010, 100)
	ob.MatchOrder(buy)

	// Cache must have advanced to the next level
	assert.Equal(t, int64(10020), ob.SellBook.BestPrice(),
		"best price cache must advance to 10020 after 10010 level is consumed")
}

func TestHM_MatchOrder_NoMatch(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10020, 100))

	buy := newOrder("b1", domain.SideBuy, 10010, 100)
	execs := ob.MatchOrder(buy)

	assert.Empty(t, execs)
	assert.Equal(t, domain.OrderStatusNew, buy.Status)
}

func TestHM_MatchOrder_FIFO(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10010, 100))

	buy := newOrder("b1", domain.SideBuy, 10010, 100)
	execs := ob.MatchOrder(buy)

	require.Len(t, execs, 1)
	assert.Equal(t, "s1", execs[0].MakerOrderID, "FIFO: s1 must match before s2")
}

func TestHM_CancelOrder(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	sell := newOrder("s1", domain.SideSell, 10010, 1000)
	ob.AddOrder(sell)

	canceled := ob.CancelOrder("s1")
	require.NotNil(t, canceled)
	assert.Equal(t, domain.OrderStatusCanceled, canceled.Status)
	assert.False(t, ob.SellBook.HasOrders())
	assert.Empty(t, ob.OrderMap)
}

func TestHM_CancelOrder_NotFound(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")
	assert.Nil(t, ob.CancelOrder("nonexistent"))
}

func TestHM_CancelOrder_MiddleOfLevel(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10010, 200))
	ob.AddOrder(newOrder("s3", domain.SideSell, 10010, 300))

	canceled := ob.CancelOrder("s2")
	require.NotNil(t, canceled)

	snap := ob.GetL2Snapshot(5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(400), snap.Asks[0].Quantity) // 100 + 300
}

func TestHM_L2Snapshot_DepthLimit(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	for i := range int64(5) {
		ob.AddOrder(newOrder(fmt.Sprintf("b%d", i), domain.SideBuy, 9990-i*10, 100))
	}

	snap := ob.GetL2Snapshot(3)
	assert.Len(t, snap.Bids, 3)
	assert.Equal(t, int64(9990), snap.Bids[0].Price, "highest bid first")
	assert.Equal(t, int64(9980), snap.Bids[1].Price)
	assert.Equal(t, int64(9970), snap.Bids[2].Price)
}

func TestHM_L2Snapshot_AsksSortedAscending(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s3", domain.SideSell, 10030, 100))
	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 100))

	snap := ob.GetL2Snapshot(0)
	require.Len(t, snap.Asks, 3)
	assert.Equal(t, int64(10010), snap.Asks[0].Price, "lowest ask first")
	assert.Equal(t, int64(10020), snap.Asks[1].Price)
	assert.Equal(t, int64(10030), snap.Asks[2].Price)
}

func TestHM_L2Snapshot_Empty(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")
	snap := ob.GetL2Snapshot(5)
	assert.Empty(t, snap.Bids)
	assert.Empty(t, snap.Asks)
}

// TestHM_AllLevelsEmpty_HasOrdersFalse verifies that HasOrders returns false
// after all resting orders are cancelled.
func TestHM_AllLevelsEmpty_HasOrdersFalse(t *testing.T) {
	ob := hmbook.NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 100))

	ob.CancelOrder("s1")
	ob.CancelOrder("s2")

	assert.False(t, ob.SellBook.HasOrders())
	assert.Equal(t, int64(0), ob.SellBook.BestPrice())
}

// ════════════════════════════════════════════════════════════════════════════
// Benchmarks — Optimized HashMap+List (hmbook) vs Red-Black Tree (rbtreebook)
//
// Run with:
//   go test ./internal/hmbook/ -bench=. -benchmem -benchtime=3s
//
// Pair naming convention:
//   BenchmarkXxx_HM  → hmbook  (optimized HashMap+List, O(1) best-price cache)
//   BenchmarkXxx_RB  → rbtreebook (Red-Black Tree, O(log n) for everything)
// ════════════════════════════════════════════════════════════════════════════

// ── AddOrder ──────────────────────────────────────────────────────────────────

// BenchmarkAddOrder_HM: O(1) HashMap insert + O(1) cache compare.
func BenchmarkAddOrder_HM(b *testing.B) {
	ob := hmbook.NewOrderBook("AAPL")
	i := 0
	for b.Loop() {
		ob.AddOrder(&domain.Order{
			OrderID:           fmt.Sprintf("o%d", i),
			Symbol:            "AAPL",
			Side:              domain.SideSell,
			Price:             int64(10000 + i%100),
			Quantity:          100,
			RemainingQuantity: 100,
			Status:            domain.OrderStatusNew,
		})
		i++
	}
}

// BenchmarkAddOrder_RB: O(log n) RB tree search/insert.
func BenchmarkAddOrder_RB(b *testing.B) {
	ob := rbtreebook.NewRBOrderBook("AAPL")
	i := 0
	for b.Loop() {
		ob.AddOrder(&domain.Order{
			OrderID:           fmt.Sprintf("o%d", i),
			Symbol:            "AAPL",
			Side:              domain.SideSell,
			Price:             int64(10000 + i%100),
			Quantity:          100,
			RemainingQuantity: 100,
			Status:            domain.OrderStatusNew,
		})
		i++
	}
}

// ── BestPrice ─────────────────────────────────────────────────────────────────

// BenchmarkBestPrice_HM: O(1) cache read.
func BenchmarkBestPrice_HM(b *testing.B) {
	ob := hmbook.NewOrderBook("AAPL")
	for i := range 100 {
		ob.AddOrder(&domain.Order{
			OrderID:           fmt.Sprintf("s%d", i),
			Symbol:            "AAPL",
			Side:              domain.SideSell,
			Price:             int64(10000 + i),
			Quantity:          100,
			RemainingQuantity: 100,
			Status:            domain.OrderStatusNew,
		})
	}
	for b.Loop() {
		_ = ob.SellBook.BestPrice()
	}
}

// BenchmarkBestPrice_RB: O(log n) left-spine traversal.
func BenchmarkBestPrice_RB(b *testing.B) {
	ob := rbtreebook.NewRBOrderBook("AAPL")
	for i := range 100 {
		ob.AddOrder(&domain.Order{
			OrderID:           fmt.Sprintf("s%d", i),
			Symbol:            "AAPL",
			Side:              domain.SideSell,
			Price:             int64(10000 + i),
			Quantity:          100,
			RemainingQuantity: 100,
			Status:            domain.OrderStatusNew,
		})
	}
	for b.Loop() {
		_ = ob.SellBook.BestPrice()
	}
}

// ── CancelOrder ───────────────────────────────────────────────────────────────

// BenchmarkCancelOrder_HM: O(1) for non-best-price cancels; O(n) only when best level empties.
// Scenario: cancel orders from random (non-best) price levels to show the O(1) fast path.
func BenchmarkCancelOrder_HM_FastPath(b *testing.B) {
	const n = 1000
	for b.Loop() {
		b.StopTimer()
		ob := hmbook.NewOrderBook("AAPL")
		// Insert: price 1 is best ask (lowest); prices 2..n are non-best
		for j := range n {
			ob.AddOrder(&domain.Order{
				OrderID:           fmt.Sprintf("s%d", j),
				Symbol:            "AAPL",
				Side:              domain.SideSell,
				Price:             int64(j + 1),
				Quantity:          10,
				RemainingQuantity: 10,
				Status:            domain.OrderStatusNew,
			})
		}
		b.StartTimer()
		// Cancel from highest price downwards — never touches the best price (s0)
		for j := n - 1; j >= 1; j-- {
			ob.CancelOrder(fmt.Sprintf("s%d", j))
		}
	}
}

// BenchmarkCancelOrder_RB: O(log n) RB delete for every cancel.
func BenchmarkCancelOrder_RB_FastPath(b *testing.B) {
	const n = 1000
	for b.Loop() {
		b.StopTimer()
		ob := rbtreebook.NewRBOrderBook("AAPL")
		for j := range n {
			ob.AddOrder(&domain.Order{
				OrderID:           fmt.Sprintf("s%d", j),
				Symbol:            "AAPL",
				Side:              domain.SideSell,
				Price:             int64(j + 1),
				Quantity:          10,
				RemainingQuantity: 10,
				Status:            domain.OrderStatusNew,
			})
		}
		b.StartTimer()
		for j := n - 1; j >= 1; j-- {
			ob.CancelOrder(fmt.Sprintf("s%d", j))
		}
	}
}

// BenchmarkCancelOrder_HM_SlowPath: cancel the BEST price level every time,
// forcing refreshBestPrice O(n) on each cancel.  Shows worst-case HashMap behaviour.
func BenchmarkCancelOrder_HM_SlowPath(b *testing.B) {
	const n = 1000
	for b.Loop() {
		b.StopTimer()
		ob := hmbook.NewOrderBook("AAPL")
		ids := make([]string, n)
		for j := range n {
			id := fmt.Sprintf("s%d", j)
			ids[j] = id
			ob.AddOrder(&domain.Order{
				OrderID:           id,
				Symbol:            "AAPL",
				Side:              domain.SideSell,
				Price:             int64(j + 1),
				Quantity:          10,
				RemainingQuantity: 10,
				Status:            domain.OrderStatusNew,
			})
		}
		b.StartTimer()
		// Cancel from lowest price (best ask) upwards — triggers rescan every time
		for j := range n {
			ob.CancelOrder(ids[j])
		}
	}
}

// BenchmarkCancelOrder_RB_SlowPath: same pattern with RB tree (O(log n) always).
func BenchmarkCancelOrder_RB_SlowPath(b *testing.B) {
	const n = 1000
	for b.Loop() {
		b.StopTimer()
		ob := rbtreebook.NewRBOrderBook("AAPL")
		ids := make([]string, n)
		for j := range n {
			id := fmt.Sprintf("s%d", j)
			ids[j] = id
			ob.AddOrder(&domain.Order{
				OrderID:           id,
				Symbol:            "AAPL",
				Side:              domain.SideSell,
				Price:             int64(j + 1),
				Quantity:          10,
				RemainingQuantity: 10,
				Status:            domain.OrderStatusNew,
			})
		}
		b.StartTimer()
		for j := range n {
			ob.CancelOrder(ids[j])
		}
	}
}

// ── MatchOrder ────────────────────────────────────────────────────────────────

// BenchmarkMatchOrder_HM: O(1) per match step (cache read + HashMap lookup).
func BenchmarkMatchOrder_HM(b *testing.B) {
	const levels = 50
	for b.Loop() {
		b.StopTimer()
		ob := hmbook.NewOrderBook("AAPL")
		for j := range levels {
			ob.AddOrder(&domain.Order{
				OrderID:           fmt.Sprintf("s%d", j),
				Symbol:            "AAPL",
				Side:              domain.SideSell,
				Price:             int64(10000 + j),
				Quantity:          10,
				RemainingQuantity: 10,
				Status:            domain.OrderStatusNew,
			})
		}
		taker := &domain.Order{
			OrderID:           "buy1",
			Symbol:            "AAPL",
			Side:              domain.SideBuy,
			Price:             int64(10000 + levels - 1),
			Quantity:          int64(levels * 10),
			RemainingQuantity: int64(levels * 10),
			Status:            domain.OrderStatusNew,
		}
		b.StartTimer()
		ob.MatchOrder(taker)
	}
}

// BenchmarkMatchOrder_RB: O(log n) per match step (spine traversal).
func BenchmarkMatchOrder_RB(b *testing.B) {
	const levels = 50
	for b.Loop() {
		b.StopTimer()
		ob := rbtreebook.NewRBOrderBook("AAPL")
		for j := range levels {
			ob.AddOrder(&domain.Order{
				OrderID:           fmt.Sprintf("s%d", j),
				Symbol:            "AAPL",
				Side:              domain.SideSell,
				Price:             int64(10000 + j),
				Quantity:          10,
				RemainingQuantity: 10,
				Status:            domain.OrderStatusNew,
			})
		}
		taker := &domain.Order{
			OrderID:           "buy1",
			Symbol:            "AAPL",
			Side:              domain.SideBuy,
			Price:             int64(10000 + levels - 1),
			Quantity:          int64(levels * 10),
			RemainingQuantity: int64(levels * 10),
			Status:            domain.OrderStatusNew,
		}
		b.StartTimer()
		ob.MatchOrder(taker)
	}
}

// ── GetL2Snapshot ─────────────────────────────────────────────────────────────

// BenchmarkGetL2Snapshot_HM: O(n log n) — must sort HashMap keys.
func BenchmarkGetL2Snapshot_HM(b *testing.B) {
	ob := hmbook.NewOrderBook("AAPL")
	for i := range 200 {
		ob.AddOrder(&domain.Order{
			OrderID:           fmt.Sprintf("s%d", i),
			Symbol:            "AAPL",
			Side:              domain.SideSell,
			Price:             int64(10000 + i),
			Quantity:          100,
			RemainingQuantity: 100,
			Status:            domain.OrderStatusNew,
		})
	}
	for b.Loop() {
		_ = ob.GetL2Snapshot(10)
	}
}

// BenchmarkGetL2Snapshot_RB: O(n) — in-order traversal, naturally sorted.
func BenchmarkGetL2Snapshot_RB(b *testing.B) {
	ob := rbtreebook.NewRBOrderBook("AAPL")
	for i := range 200 {
		ob.AddOrder(&domain.Order{
			OrderID:           fmt.Sprintf("s%d", i),
			Symbol:            "AAPL",
			Side:              domain.SideSell,
			Price:             int64(10000 + i),
			Quantity:          100,
			RemainingQuantity: 100,
			Status:            domain.OrderStatusNew,
		})
	}
	for b.Loop() {
		_ = ob.GetL2Snapshot(10)
	}
}

// ── Worst-case insert pattern (ascending keys) ────────────────────────────────

// BenchmarkWorstCaseInsert_HM: O(1) regardless of insertion order.
func BenchmarkWorstCaseInsert_HM(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		ob := hmbook.NewOrderBook("AAPL")
		b.StartTimer()
		for j := range 1000 {
			ob.AddOrder(&domain.Order{
				OrderID:           fmt.Sprintf("s%d", j),
				Symbol:            "AAPL",
				Side:              domain.SideSell,
				Price:             int64(j + 1), // ascending: cache updates every insert
				Quantity:          10,
				RemainingQuantity: 10,
				Status:            domain.OrderStatusNew,
			})
		}
	}
}

// BenchmarkWorstCaseInsert_RB: O(log n) with maximum left-rotations (ascending input).
func BenchmarkWorstCaseInsert_RB(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		ob := rbtreebook.NewRBOrderBook("AAPL")
		b.StartTimer()
		for j := range 1000 {
			ob.AddOrder(&domain.Order{
				OrderID:           fmt.Sprintf("s%d", j),
				Symbol:            "AAPL",
				Side:              domain.SideSell,
				Price:             int64(j + 1),
				Quantity:          10,
				RemainingQuantity: 10,
				Status:            domain.OrderStatusNew,
			})
		}
	}
}
