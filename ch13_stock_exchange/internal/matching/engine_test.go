package matching

import (
	"testing"

	"github.com/nathanyu/stock-exchange/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newOrder(id string, symbol string, side domain.Side, price, qty int64) *domain.Order {
	return &domain.Order{
		OrderID:           id,
		Symbol:            symbol,
		Side:              side,
		Price:             price,
		Quantity:          qty,
		RemainingQuantity: qty,
		Status:            domain.OrderStatusNew,
		UserID:            "user1",
	}
}

func TestEngine_NewOrder_NoMatch(t *testing.T) {
	engine := NewEngine()

	order := newOrder("o1", "AAPL", domain.SideSell, 10010, 1000)
	event := &domain.OrderEvent{Action: domain.OrderActionNew, Order: order}
	result := engine.HandleOrder(event)

	require.NotNil(t, result)
	assert.Empty(t, result.Executions)
	assert.Equal(t, order, result.TakerOrder)

	// Order should be resting in the book
	snap := engine.GetL2Snapshot("AAPL", 5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(1000), snap.Asks[0].Quantity)
}

func TestEngine_NewOrder_Match(t *testing.T) {
	engine := NewEngine()

	// Place resting sell
	sell := newOrder("s1", "AAPL", domain.SideSell, 10010, 1000)
	engine.HandleOrder(&domain.OrderEvent{Action: domain.OrderActionNew, Order: sell})

	// Place crossing buy
	buy := newOrder("b1", "AAPL", domain.SideBuy, 10010, 200)
	result := engine.HandleOrder(&domain.OrderEvent{Action: domain.OrderActionNew, Order: buy})

	require.Len(t, result.Executions, 1)
	assert.Equal(t, int64(200), result.Executions[0].Quantity)
	assert.Equal(t, int64(10010), result.Executions[0].Price)
	assert.Equal(t, domain.OrderStatusFilled, buy.Status)

	// Sell should have 800 remaining
	snap := engine.GetL2Snapshot("AAPL", 5)
	require.Len(t, snap.Asks, 1)
	assert.Equal(t, int64(800), snap.Asks[0].Quantity)
}

func TestEngine_CancelOrder(t *testing.T) {
	engine := NewEngine()

	sell := newOrder("s1", "AAPL", domain.SideSell, 10010, 1000)
	engine.HandleOrder(&domain.OrderEvent{Action: domain.OrderActionNew, Order: sell})

	// Cancel
	cancel := &domain.Order{OrderID: "s1", Symbol: "AAPL"}
	result := engine.HandleOrder(&domain.OrderEvent{Action: domain.OrderActionCancel, Order: cancel})

	require.NotNil(t, result)
	assert.Equal(t, domain.OrderStatusCanceled, result.TakerOrder.Status)

	snap := engine.GetL2Snapshot("AAPL", 5)
	assert.Empty(t, snap.Asks)
}

func TestEngine_MultipleSymbols(t *testing.T) {
	engine := NewEngine()

	engine.HandleOrder(&domain.OrderEvent{
		Action: domain.OrderActionNew,
		Order:  newOrder("a1", "AAPL", domain.SideSell, 10010, 100),
	})
	engine.HandleOrder(&domain.OrderEvent{
		Action: domain.OrderActionNew,
		Order:  newOrder("g1", "GOOG", domain.SideSell, 20000, 50),
	})

	aaplSnap := engine.GetL2Snapshot("AAPL", 5)
	googSnap := engine.GetL2Snapshot("GOOG", 5)

	require.Len(t, aaplSnap.Asks, 1)
	require.Len(t, googSnap.Asks, 1)
	assert.Equal(t, int64(10010), aaplSnap.Asks[0].Price)
	assert.Equal(t, int64(20000), googSnap.Asks[0].Price)
}

func TestEngine_Determinism(t *testing.T) {
	// Given the same sequence of orders, we should get the same executions
	orders := []*domain.OrderEvent{
		{Action: domain.OrderActionNew, Order: newOrder("s1", "AAPL", domain.SideSell, 10010, 100)},
		{Action: domain.OrderActionNew, Order: newOrder("s2", "AAPL", domain.SideSell, 10010, 200)},
		{Action: domain.OrderActionNew, Order: newOrder("b1", "AAPL", domain.SideBuy, 10010, 150)},
	}

	// Run twice and compare
	run := func() []*domain.Execution {
		e := NewEngine()
		var allExecs []*domain.Execution
		for _, evt := range orders {
			// Deep copy orders for each run
			o := *evt.Order
			o.RemainingQuantity = o.Quantity
			o.FilledQuantity = 0
			o.Status = domain.OrderStatusNew
			result := e.HandleOrder(&domain.OrderEvent{Action: evt.Action, Order: &o})
			allExecs = append(allExecs, result.Executions...)
		}
		return allExecs
	}

	execs1 := run()
	execs2 := run()

	require.Equal(t, len(execs1), len(execs2))
	for i := range execs1 {
		assert.Equal(t, execs1[i].Quantity, execs2[i].Quantity)
		assert.Equal(t, execs1[i].Price, execs2[i].Price)
		assert.Equal(t, execs1[i].MakerOrderID, execs2[i].MakerOrderID)
	}
}

func TestEngine_GetL2Snapshot_NonexistentSymbol(t *testing.T) {
	engine := NewEngine()
	snap := engine.GetL2Snapshot("UNKNOWN", 5)
	assert.Empty(t, snap.Bids)
	assert.Empty(t, snap.Asks)
}
