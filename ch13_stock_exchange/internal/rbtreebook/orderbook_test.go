package rbtreebook_test

import (
	"fmt"
	"testing"

	"github.com/nathanyu/stock-exchange/internal/domain"
	"github.com/nathanyu/stock-exchange/internal/orderbook"
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

// ── RB Tree unit tests ────────────────────────────────────────────────────────

func TestRBTree_AddSingleSellOrder(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

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

func TestRBTree_AddMultipleOrders_SamePrice(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 500))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10010, 300))

	snap := ob.GetL2Snapshot(5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(800), snap.Asks[0].Quantity)
}

func TestRBTree_BestPriceTracking(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	ob.AddOrder(newOrder("b1", domain.SideBuy, 9990, 100))
	ob.AddOrder(newOrder("b2", domain.SideBuy, 10000, 100))
	ob.AddOrder(newOrder("b3", domain.SideBuy, 9980, 100))

	assert.Equal(t, int64(10000), ob.BuyBook.BestPrice(), "best bid should be highest buy price")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 100))

	assert.Equal(t, int64(10010), ob.SellBook.BestPrice(), "best ask should be lowest sell price")
}

func TestRBTree_BestPriceUpdatesAfterCancel(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	ob.AddOrder(newOrder("b1", domain.SideBuy, 10000, 100))
	ob.AddOrder(newOrder("b2", domain.SideBuy, 9990, 100))
	assert.Equal(t, int64(10000), ob.BuyBook.BestPrice())

	ob.CancelOrder("b1")
	// After canceling the best bid, the next level becomes best
	assert.Equal(t, int64(9990), ob.BuyBook.BestPrice())
}

func TestRBTree_MatchOrder_FullFill(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

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

func TestRBTree_MatchOrder_PartialFill(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	sell := newOrder("s1", domain.SideSell, 10010, 1000)
	ob.AddOrder(sell)

	buy := newOrder("b1", domain.SideBuy, 10010, 200)
	execs := ob.MatchOrder(buy)

	require.Len(t, execs, 1)
	assert.Equal(t, int64(200), execs[0].Quantity)
	assert.Equal(t, domain.OrderStatusFilled, buy.Status)
	assert.Equal(t, domain.OrderStatusPartiallyFilled, sell.Status)
	assert.Equal(t, int64(800), sell.RemainingQuantity)
	assert.True(t, ob.SellBook.HasOrders())

	snap := ob.GetL2Snapshot(5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(800), snap.Asks[0].Quantity)
}

func TestRBTree_MatchOrder_MultiplePriceLevels(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 200))

	buy := newOrder("b1", domain.SideBuy, 10020, 300)
	execs := ob.MatchOrder(buy)

	require.Len(t, execs, 2)
	assert.Equal(t, int64(100), execs[0].Quantity)
	assert.Equal(t, int64(10010), execs[0].Price)
	assert.Equal(t, int64(200), execs[1].Quantity)
	assert.Equal(t, int64(10020), execs[1].Price)
	assert.Equal(t, domain.OrderStatusFilled, buy.Status)
	assert.False(t, ob.SellBook.HasOrders())
}

func TestRBTree_MatchOrder_NoMatch(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10020, 100))

	buy := newOrder("b1", domain.SideBuy, 10010, 100)
	execs := ob.MatchOrder(buy)

	assert.Empty(t, execs)
	assert.Equal(t, domain.OrderStatusNew, buy.Status)
	assert.Equal(t, int64(100), buy.RemainingQuantity)
}

func TestRBTree_MatchOrder_FIFO(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	// s1 arrives before s2 at the same price — s1 must match first
	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10010, 100))

	buy := newOrder("b1", domain.SideBuy, 10010, 100)
	execs := ob.MatchOrder(buy)

	require.Len(t, execs, 1)
	assert.Equal(t, "s1", execs[0].MakerOrderID, "FIFO: s1 should match before s2")
}

func TestRBTree_CancelOrder(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	sell := newOrder("s1", domain.SideSell, 10010, 1000)
	ob.AddOrder(sell)

	canceled := ob.CancelOrder("s1")
	require.NotNil(t, canceled)
	assert.Equal(t, domain.OrderStatusCanceled, canceled.Status)
	assert.False(t, ob.SellBook.HasOrders())
	assert.Empty(t, ob.OrderMap)
}

func TestRBTree_CancelOrder_NotFound(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")
	assert.Nil(t, ob.CancelOrder("nonexistent"))
}

func TestRBTree_CancelOrder_MiddleOfLevel(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10010, 200))
	ob.AddOrder(newOrder("s3", domain.SideSell, 10010, 300))

	canceled := ob.CancelOrder("s2") // middle order
	require.NotNil(t, canceled)

	snap := ob.GetL2Snapshot(5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(400), snap.Asks[0].Quantity) // 100 + 300
}

func TestRBTree_L2Snapshot_Depth(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	for i := range int64(5) {
		ob.AddOrder(newOrder(
			fmt.Sprintf("b%d", i),
			domain.SideBuy,
			9990-i*10,
			100,
		))
	}

	snap := ob.GetL2Snapshot(3)
	assert.Len(t, snap.Bids, 3)
	assert.Equal(t, int64(9990), snap.Bids[0].Price, "highest bid first")
	assert.Equal(t, int64(9980), snap.Bids[1].Price)
	assert.Equal(t, int64(9970), snap.Bids[2].Price)
}

func TestRBTree_L2Snapshot_AsksSortedAscending(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	ob.AddOrder(newOrder("s3", domain.SideSell, 10030, 100))
	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 100))

	snap := ob.GetL2Snapshot(0) // depth 0 = all levels
	require.Len(t, snap.Asks, 3)
	assert.Equal(t, int64(10010), snap.Asks[0].Price, "lowest ask first")
	assert.Equal(t, int64(10020), snap.Asks[1].Price)
	assert.Equal(t, int64(10030), snap.Asks[2].Price)
}

func TestRBTree_L2Snapshot_Empty(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")
	snap := ob.GetL2Snapshot(5)
	assert.Empty(t, snap.Bids)
	assert.Empty(t, snap.Asks)
}

// TestRBTree_RebalanceCorrectness inserts many orders in a pattern that forces
// multiple rotations and recolourings, then verifies the book state is correct.
func TestRBTree_RebalanceCorrectness(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	// Insert in ascending order — worst case for naive BST, forces left rotations
	prices := []int64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	for i, p := range prices {
		ob.AddOrder(newOrder(fmt.Sprintf("s%d", i), domain.SideSell, p, 10))
	}

	assert.Equal(t, int64(100), ob.SellBook.BestPrice(), "min after ascending insert")

	// Insert in descending order — forces right rotations
	ob2 := rbtreebook.NewRBOrderBook("AAPL")
	for i, p := range []int64{1000, 900, 800, 700, 600, 500, 400, 300, 200, 100} {
		ob2.AddOrder(newOrder(fmt.Sprintf("s%d", i), domain.SideSell, p, 10))
	}
	assert.Equal(t, int64(100), ob2.SellBook.BestPrice(), "min after descending insert")

	// Interleaved — forces Case 1 (uncle red) recolourings
	ob3 := rbtreebook.NewRBOrderBook("AAPL")
	for i, p := range []int64{500, 300, 700, 200, 400, 600, 800, 100, 250, 350} {
		ob3.AddOrder(newOrder(fmt.Sprintf("s%d", i), domain.SideSell, p, 10))
	}
	snap := ob3.GetL2Snapshot(0)
	require.Len(t, snap.Asks, 10)
	// Verify ascending sort (proves in-order traversal is correct after rebalancing)
	for i := 1; i < len(snap.Asks); i++ {
		assert.Less(t, snap.Asks[i-1].Price, snap.Asks[i].Price,
			"L2 asks must be sorted ascending after rebalancing")
	}
}

// TestRBTree_DeleteRebalanceCorrectness verifies the book stays consistent
// through many inserts and deletes that exercise all four delete fix-up cases.
func TestRBTree_DeleteRebalanceCorrectness(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")
	ids := make([]string, 0, 20)

	for i := range int64(20) {
		id := fmt.Sprintf("s%d", i)
		ids = append(ids, id)
		ob.AddOrder(newOrder(id, domain.SideSell, (i+1)*100, 10))
	}

	// Delete every other order — exercises both Case 2 (double-black push-up)
	// and Cases 3/4 (rotations in deleteFixup)
	for i := 0; i < 20; i += 2 {
		canceled := ob.CancelOrder(ids[i])
		require.NotNil(t, canceled, "cancel should succeed for id=%s", ids[i])
	}

	snap := ob.GetL2Snapshot(0)
	require.Len(t, snap.Asks, 10, "10 levels should remain after deleting 10")
	for i := 1; i < len(snap.Asks); i++ {
		assert.Less(t, snap.Asks[i-1].Price, snap.Asks[i].Price,
			"remaining levels must still be sorted after deletes")
	}
}

// ── RB Tree-specific invariant tests ─────────────────────────────────────────
// (White-box: these tests would break on a HashMap+List implementation)

// TestRBTree_BestPriceIsLogN verifies that BestPrice is always current after
// operations — it calls Min/Max on the tree, not a stale cached value.
func TestRBTree_BestPriceIsAlwaysCurrent(t *testing.T) {
	ob := rbtreebook.NewRBOrderBook("AAPL")

	prices := []int64{500, 400, 600, 300, 700}
	for i, p := range prices {
		ob.AddOrder(newOrder(fmt.Sprintf("s%d", i), domain.SideSell, p, 10))
	}

	// Verify BestPrice tracks the minimum sell price correctly as we cancel levels
	expected := []int64{300, 400, 500, 600, 700}
	for i, exp := range expected {
		assert.Equal(t, exp, ob.SellBook.BestPrice(), "step %d", i)
		ob.CancelOrder(fmt.Sprintf("s%d", findIDByPrice(prices, exp)))
	}
	assert.False(t, ob.SellBook.HasOrders())
}

func findIDByPrice(prices []int64, target int64) int {
	for i, p := range prices {
		if p == target {
			return i
		}
	}
	return -1
}

// ════════════════════════════════════════════════════════════════════════════
// Benchmarks — RB Tree vs HashMap+List
//
// Run with: go test ./internal/rbtreebook/ -bench=. -benchmem -benchtime=3s
//
// Each "RB" benchmark is paired with an equivalent "HM" (HashMap) benchmark
// so the numbers can be compared directly in the same run.
// ════════════════════════════════════════════════════════════════════════════

// ── AddOrder ──────────────────────────────────────────────────────────────────

// BenchmarkAddOrder_RB measures O(log n) insert into an RB tree.
func BenchmarkAddOrder_RB(b *testing.B) {
	ob := rbtreebook.NewRBOrderBook("AAPL")
	i := 0
	for b.Loop() {
		price := int64(10000 + i%100) // 100 distinct price levels → tree height ≈ log2(100) ≈ 7
		ob.AddOrder(&domain.Order{
			OrderID:           fmt.Sprintf("o%d", i),
			Symbol:            "AAPL",
			Side:              domain.SideSell,
			Price:             price,
			Quantity:          100,
			RemainingQuantity: 100,
			Status:            domain.OrderStatusNew,
		})
		i++
	}
}

// BenchmarkAddOrder_HM measures O(1) insert + O(n) refreshBestPrice in HashMap.
func BenchmarkAddOrder_HM(b *testing.B) {
	ob := orderbook.NewOrderBook("AAPL")
	i := 0
	for b.Loop() {
		price := int64(10000 + i%100)
		ob.AddOrder(&domain.Order{
			OrderID:           fmt.Sprintf("o%d", i),
			Symbol:            "AAPL",
			Side:              domain.SideSell,
			Price:             price,
			Quantity:          100,
			RemainingQuantity: 100,
			Status:            domain.OrderStatusNew,
		})
		i++
	}
}

// ── BestPrice ─────────────────────────────────────────────────────────────────

// BenchmarkBestPrice_RB measures O(log n) spine traversal.
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

// BenchmarkBestPrice_HM measures O(1) cached read (set after O(n) refresh).
func BenchmarkBestPrice_HM(b *testing.B) {
	ob := orderbook.NewOrderBook("AAPL")
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

// ── CancelOrder (exercises RB delete fix-up) ──────────────────────────────────

// BenchmarkCancelOrder_RB measures O(log n) RB delete.
func BenchmarkCancelOrder_RB(b *testing.B) {
	const n = 1000
	for b.Loop() {
		b.StopTimer()
		ob := rbtreebook.NewRBOrderBook("AAPL")
		for j := range n {
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
		b.StartTimer()
		for j := range n {
			ob.CancelOrder(fmt.Sprintf("s%d", j))
		}
	}
}

// BenchmarkCancelOrder_HM measures O(1) list remove + O(n) refreshBestPrice.
func BenchmarkCancelOrder_HM(b *testing.B) {
	const n = 1000
	for b.Loop() {
		b.StopTimer()
		ob := orderbook.NewOrderBook("AAPL")
		for j := range n {
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
		b.StartTimer()
		for j := range n {
			ob.CancelOrder(fmt.Sprintf("s%d", j))
		}
	}
}

// ── MatchOrder (hot path, multi-level sweep) ──────────────────────────────────

// BenchmarkMatchOrder_RB measures the full match loop with O(log n) per level.
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

// BenchmarkMatchOrder_HM measures the same scenario with the HashMap implementation.
func BenchmarkMatchOrder_HM(b *testing.B) {
	const levels = 50
	for b.Loop() {
		b.StopTimer()
		ob := orderbook.NewOrderBook("AAPL")
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

// ── GetL2Snapshot (in-order vs sort.Slice) ────────────────────────────────────

// BenchmarkGetL2Snapshot_RB measures O(n) in-order traversal.
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

// BenchmarkGetL2Snapshot_HM measures O(n log n) sort.Slice.
func BenchmarkGetL2Snapshot_HM(b *testing.B) {
	ob := orderbook.NewOrderBook("AAPL")
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

// ── Latency jitter simulation ─────────────────────────────────────────────────
//
// BenchmarkWorstCaseInsert_RB inserts keys in strictly ascending order.
// This is the worst-case insertion pattern for a naive BST (linear chain),
// but for an RB tree it triggers maximum left rotations during fix-up.
// The goal is to observe whether the RB tree's worst-case still has bounded
// and predictable latency (no degeneration to O(n)).

// BenchmarkWorstCaseInsert_RB inserts keys in ascending order (triggers left rotations).
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
				Price:             int64(j + 1), // strictly ascending
				Quantity:          10,
				RemainingQuantity: 10,
				Status:            domain.OrderStatusNew,
			})
		}
	}
}

// BenchmarkWorstCaseInsert_HM inserts the same ascending sequence into HashMap.
func BenchmarkWorstCaseInsert_HM(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		ob := orderbook.NewOrderBook("AAPL")
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
