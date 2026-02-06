package ordermanager

import (
	"testing"

	"github.com/nathanyu/stock-exchange/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager() *Manager {
	m := NewManager(1_000_000, 100)
	m.InitWallet("user1", 10_000_000, map[string]int64{"AAPL": 5000})
	m.InitWallet("user2", 10_000_000, map[string]int64{"AAPL": 5000})
	return m
}

func TestPlaceOrder_Buy(t *testing.T) {
	m := newTestManager()

	order, err := m.PlaceOrder("user1", "AAPL", domain.SideBuy, 10010, 100)
	require.NoError(t, err)
	require.NotNil(t, order)

	assert.NotEmpty(t, order.OrderID)
	assert.Equal(t, "AAPL", order.Symbol)
	assert.Equal(t, domain.SideBuy, order.Side)
	assert.Equal(t, int64(10010), order.Price)
	assert.Equal(t, int64(100), order.Quantity)
	assert.Equal(t, domain.OrderStatusNew, order.Status)

	// Should have sent to OrderOut channel
	event := <-m.OrderOut
	assert.Equal(t, domain.OrderActionNew, event.Action)
	assert.Equal(t, order.OrderID, event.Order.OrderID)
}

func TestPlaceOrder_Sell(t *testing.T) {
	m := newTestManager()

	order, err := m.PlaceOrder("user1", "AAPL", domain.SideSell, 10010, 100)
	require.NoError(t, err)
	require.NotNil(t, order)

	event := <-m.OrderOut
	assert.Equal(t, domain.OrderActionNew, event.Action)
}

func TestPlaceOrder_InsufficientFunds(t *testing.T) {
	m := newTestManager()

	// Try to buy more than cash allows
	// Cash = 10,000,000 cents. Price 10010 * qty 1001 = 10,020,010 > 10,000,000
	_, err := m.PlaceOrder("user1", "AAPL", domain.SideBuy, 10010, 1001)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient funds")
}

func TestPlaceOrder_InsufficientShares(t *testing.T) {
	m := newTestManager()

	// User has 5000 AAPL shares
	_, err := m.PlaceOrder("user1", "AAPL", domain.SideSell, 10010, 5001)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient shares")
}

func TestPlaceOrder_DailyVolumeLimit(t *testing.T) {
	m := NewManager(100, 100) // Very low daily limit
	m.InitWallet("user1", 10_000_000, map[string]int64{"AAPL": 5000})

	_, err := m.PlaceOrder("user1", "AAPL", domain.SideSell, 10010, 101)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "daily volume limit")
}

func TestPlaceOrder_UserNotFound(t *testing.T) {
	m := newTestManager()

	_, err := m.PlaceOrder("unknown", "AAPL", domain.SideBuy, 10010, 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCancelOrder(t *testing.T) {
	m := newTestManager()

	order, err := m.PlaceOrder("user1", "AAPL", domain.SideSell, 10010, 100)
	require.NoError(t, err)
	<-m.OrderOut // drain

	canceled, err := m.CancelOrder(order.OrderID)
	require.NoError(t, err)
	assert.Equal(t, order.OrderID, canceled.OrderID)

	// Should have sent cancel event
	event := <-m.OrderOut
	assert.Equal(t, domain.OrderActionCancel, event.Action)
}

func TestCancelOrder_NotFound(t *testing.T) {
	m := newTestManager()

	_, err := m.CancelOrder("nonexistent")
	assert.Error(t, err)
}

func TestWithheldFunds(t *testing.T) {
	m := newTestManager()

	// Place first buy that withholds funds
	_, err := m.PlaceOrder("user1", "AAPL", domain.SideBuy, 10010, 500)
	require.NoError(t, err)
	<-m.OrderOut

	// Second buy should see reduced available funds
	// Total cash: 10,000,000. First order withheld: 10010*500 = 5,005,000
	// Available: 4,995,000. Second order: 10010*500 = 5,005,000 > 4,995,000
	_, err = m.PlaceOrder("user1", "AAPL", domain.SideBuy, 10010, 500)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient funds")
}

func TestGetWallet(t *testing.T) {
	m := newTestManager()

	wallet := m.GetWallet("user1")
	require.NotNil(t, wallet)
	assert.Equal(t, int64(10_000_000), wallet.CashBalance)
	assert.Equal(t, int64(5000), wallet.Holdings["AAPL"])

	// Nonexistent user
	assert.Nil(t, m.GetWallet("nobody"))
}

func TestGetAllWallets(t *testing.T) {
	m := newTestManager()

	wallets := m.GetAllWallets()
	assert.Len(t, wallets, 2)
	assert.Contains(t, wallets, "user1")
	assert.Contains(t, wallets, "user2")
}
