package orderbook

import (
	"testing"

	"github.com/nathanyu/stock-exchange/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestAddOrder(t *testing.T) {
	ob := NewOrderBook("AAPL")

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

func TestAddMultipleOrders_SamePrice(t *testing.T) {
	ob := NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 500))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10010, 300))

	snap := ob.GetL2Snapshot(5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(800), snap.Asks[0].Quantity) // aggregated
}

func TestBestPriceTracking(t *testing.T) {
	ob := NewOrderBook("AAPL")

	ob.AddOrder(newOrder("b1", domain.SideBuy, 9990, 100))
	ob.AddOrder(newOrder("b2", domain.SideBuy, 10000, 100))
	ob.AddOrder(newOrder("b3", domain.SideBuy, 9980, 100))

	// Best bid = highest buy price
	assert.Equal(t, int64(10000), ob.BuyBook.BestPrice())

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 100))

	// Best ask = lowest sell price
	assert.Equal(t, int64(10010), ob.SellBook.BestPrice())
}

func TestMatchOrder_FullFill(t *testing.T) {
	ob := NewOrderBook("AAPL")

	// Resting sell order
	sell := newOrder("s1", domain.SideSell, 10010, 1000)
	ob.AddOrder(sell)

	// Incoming buy order that crosses the spread
	buy := newOrder("b1", domain.SideBuy, 10010, 1000)
	execs := ob.MatchOrder(buy)

	require.Len(t, execs, 1)
	assert.Equal(t, int64(1000), execs[0].Quantity)
	assert.Equal(t, int64(10010), execs[0].Price) // executes at maker's price
	assert.Equal(t, "s1", execs[0].MakerOrderID)
	assert.Equal(t, "b1", execs[0].TakerOrderID)

	assert.Equal(t, domain.OrderStatusFilled, buy.Status)
	assert.Equal(t, domain.OrderStatusFilled, sell.Status)
	assert.False(t, ob.SellBook.HasOrders())
}

func TestMatchOrder_PartialFill(t *testing.T) {
	ob := NewOrderBook("AAPL")

	sell := newOrder("s1", domain.SideSell, 10010, 1000)
	ob.AddOrder(sell)

	// Smaller buy
	buy := newOrder("b1", domain.SideBuy, 10010, 200)
	execs := ob.MatchOrder(buy)

	require.Len(t, execs, 1)
	assert.Equal(t, int64(200), execs[0].Quantity)
	assert.Equal(t, domain.OrderStatusFilled, buy.Status)
	assert.Equal(t, domain.OrderStatusPartiallyFilled, sell.Status)
	assert.Equal(t, int64(800), sell.RemainingQuantity)

	// Sell book should still have the remaining order
	assert.True(t, ob.SellBook.HasOrders())
	snap := ob.GetL2Snapshot(5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(800), snap.Asks[0].Quantity)
}

func TestMatchOrder_MultipleLevels(t *testing.T) {
	ob := NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10020, 200))

	// Big buy that sweeps both levels
	buy := newOrder("b1", domain.SideBuy, 10020, 300)
	execs := ob.MatchOrder(buy)

	require.Len(t, execs, 2)
	assert.Equal(t, int64(100), execs[0].Quantity) // filled at 10010 first (best ask)
	assert.Equal(t, int64(10010), execs[0].Price)
	assert.Equal(t, int64(200), execs[1].Quantity) // then at 10020
	assert.Equal(t, int64(10020), execs[1].Price)

	assert.Equal(t, domain.OrderStatusFilled, buy.Status)
	assert.False(t, ob.SellBook.HasOrders())
}

func TestMatchOrder_NoMatch(t *testing.T) {
	ob := NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10020, 100))

	// Buy price is below the ask
	buy := newOrder("b1", domain.SideBuy, 10010, 100)
	execs := ob.MatchOrder(buy)

	assert.Empty(t, execs)
	assert.Equal(t, domain.OrderStatusNew, buy.Status)
	assert.Equal(t, int64(100), buy.RemainingQuantity)
}

func TestMatchOrder_FIFO(t *testing.T) {
	ob := NewOrderBook("AAPL")

	// Two sells at same price - s1 arrived first
	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10010, 100))

	buy := newOrder("b1", domain.SideBuy, 10010, 100)
	execs := ob.MatchOrder(buy)

	require.Len(t, execs, 1)
	assert.Equal(t, "s1", execs[0].MakerOrderID) // s1 matched first (FIFO)
}

func TestCancelOrder(t *testing.T) {
	ob := NewOrderBook("AAPL")

	sell := newOrder("s1", domain.SideSell, 10010, 1000)
	ob.AddOrder(sell)

	canceled := ob.CancelOrder("s1")
	require.NotNil(t, canceled)
	assert.Equal(t, domain.OrderStatusCanceled, canceled.Status)
	assert.False(t, ob.SellBook.HasOrders())
	assert.Empty(t, ob.OrderMap)
}

func TestCancelOrder_NotFound(t *testing.T) {
	ob := NewOrderBook("AAPL")
	canceled := ob.CancelOrder("nonexistent")
	assert.Nil(t, canceled)
}

func TestCancelOrder_MiddleOfLevel(t *testing.T) {
	ob := NewOrderBook("AAPL")

	ob.AddOrder(newOrder("s1", domain.SideSell, 10010, 100))
	ob.AddOrder(newOrder("s2", domain.SideSell, 10010, 200))
	ob.AddOrder(newOrder("s3", domain.SideSell, 10010, 300))

	// Cancel the middle order
	canceled := ob.CancelOrder("s2")
	require.NotNil(t, canceled)

	snap := ob.GetL2Snapshot(5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(400), snap.Asks[0].Quantity) // 100 + 300
}

func TestL2Snapshot_Depth(t *testing.T) {
	ob := NewOrderBook("AAPL")

	// Add 5 buy levels
	for i := int64(0); i < 5; i++ {
		ob.AddOrder(newOrder(
			"b"+string(rune('1'+i)),
			domain.SideBuy,
			9990-i*10,
			100,
		))
	}

	// Depth = 3 should only return top 3
	snap := ob.GetL2Snapshot(3)
	assert.Len(t, snap.Bids, 3)
	// Should be sorted descending for bids
	assert.Equal(t, int64(9990), snap.Bids[0].Price)
	assert.Equal(t, int64(9980), snap.Bids[1].Price)
	assert.Equal(t, int64(9970), snap.Bids[2].Price)
}

func TestL2Snapshot_Empty(t *testing.T) {
	ob := NewOrderBook("AAPL")
	snap := ob.GetL2Snapshot(5)
	assert.Empty(t, snap.Bids)
	assert.Empty(t, snap.Asks)
}
