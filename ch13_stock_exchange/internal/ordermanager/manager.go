package ordermanager

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nathanyu/stock-exchange/internal/domain"
)

// Wallet tracks a user's cash balance and stock holdings.
type Wallet struct {
	CashBalance int64            // in cents
	Holdings    map[string]int64 // symbol -> quantity
	// Withheld amounts for pending buy orders
	WithheldCash map[string]int64 // orderID -> withheld cents
	// Withheld shares for pending sell orders
	WithheldShares map[string]withheldShare // orderID -> withheld share info
}

type withheldShare struct {
	Symbol   string
	Quantity int64
}

// Manager handles order validation, risk checks, and wallet management.
// It receives orders from the API, validates them, and forwards them to the sequencer.
// It also receives execution events to update wallet balances and order states.
type Manager struct {
	mu sync.RWMutex

	wallets map[string]*Wallet        // userID -> wallet
	orders  map[string]*domain.Order  // orderID -> order

	// Risk check: per-user per-symbol daily volume limit
	dailyVolume map[string]int64 // "userID:symbol" -> volume today
	maxDailyVolume int64

	// Channel to send validated orders to the sequencer
	OrderOut chan *domain.OrderEvent

	// Channel to receive execution events from the sequencer
	ExecutionIn chan *domain.ExecutionEvent

	done chan struct{}
}

// NewManager creates a new order manager.
func NewManager(maxDailyVolume int64, bufferSize int) *Manager {
	return &Manager{
		wallets:        make(map[string]*Wallet),
		orders:         make(map[string]*domain.Order),
		dailyVolume:    make(map[string]int64),
		maxDailyVolume: maxDailyVolume,
		OrderOut:       make(chan *domain.OrderEvent, bufferSize),
		ExecutionIn:    make(chan *domain.ExecutionEvent, bufferSize),
		done:           make(chan struct{}),
	}
}

// Start begins the execution listener goroutine.
func (m *Manager) Start() {
	go m.listenExecutions()
}

// Stop shuts down the manager.
func (m *Manager) Stop() {
	close(m.done)
}

// InitWallet initializes a user's wallet with starting balances.
func (m *Manager) InitWallet(userID string, cashBalance int64, holdings map[string]int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h := make(map[string]int64)
	for k, v := range holdings {
		h[k] = v
	}

	m.wallets[userID] = &Wallet{
		CashBalance:    cashBalance,
		Holdings:       h,
		WithheldCash:   make(map[string]int64),
		WithheldShares: make(map[string]withheldShare),
	}
}

// GetWallet returns a copy of a user's wallet.
func (m *Manager) GetWallet(userID string) *Wallet {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w, exists := m.wallets[userID]
	if !exists {
		return nil
	}

	// Return a copy
	holdings := make(map[string]int64)
	for k, v := range w.Holdings {
		holdings[k] = v
	}
	return &Wallet{
		CashBalance: w.CashBalance,
		Holdings:    holdings,
	}
}

// GetAllWallets returns a copy of all wallets.
func (m *Manager) GetAllWallets() map[string]*Wallet {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Wallet)
	for userID, w := range m.wallets {
		holdings := make(map[string]int64)
		for k, v := range w.Holdings {
			holdings[k] = v
		}
		result[userID] = &Wallet{
			CashBalance: w.CashBalance,
			Holdings:    holdings,
		}
	}
	return result
}

// PlaceOrder validates and submits a new order.
func (m *Manager) PlaceOrder(userID, symbol string, side domain.Side, price, quantity int64) (*domain.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wallet, exists := m.wallets[userID]
	if !exists {
		return nil, fmt.Errorf("user %s not found", userID)
	}

	// Risk check: daily volume limit
	volKey := userID + ":" + symbol
	if m.dailyVolume[volKey]+quantity > m.maxDailyVolume {
		return nil, fmt.Errorf("daily volume limit exceeded for %s on %s", userID, symbol)
	}

	// Wallet check
	if side == domain.SideBuy {
		// Withhold cash: price * quantity (in cents)
		cost := price * quantity
		available := wallet.CashBalance - m.totalWithheldCash(wallet)
		if available < cost {
			return nil, fmt.Errorf("insufficient funds: need %d, available %d", cost, available)
		}
	} else {
		// Withhold shares
		available := wallet.Holdings[symbol] - m.totalWithheldShares(wallet, symbol)
		if available < quantity {
			return nil, fmt.Errorf("insufficient shares: need %d %s, available %d", quantity, symbol, available)
		}
	}

	order := &domain.Order{
		OrderID:           uuid.New().String(),
		Symbol:            symbol,
		Side:              side,
		Price:             price,
		Quantity:          quantity,
		RemainingQuantity: quantity,
		Status:            domain.OrderStatusNew,
		UserID:            userID,
		CreatedAt:         time.Now(),
	}

	// Withhold funds/shares
	if side == domain.SideBuy {
		wallet.WithheldCash[order.OrderID] = price * quantity
	} else {
		wallet.WithheldShares[order.OrderID] = withheldShare{
			Symbol:   symbol,
			Quantity: quantity,
		}
	}

	// Track daily volume
	m.dailyVolume[volKey] += quantity

	// Store order
	m.orders[order.OrderID] = order

	// Send to sequencer (non-blocking)
	select {
	case m.OrderOut <- &domain.OrderEvent{Action: domain.OrderActionNew, Order: order}:
	default:
		log.Println("[ordermanager] WARN: order output channel full")
	}

	return order, nil
}

// CancelOrder submits a cancel request.
func (m *Manager) CancelOrder(orderID string) (*domain.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, exists := m.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order %s not found", orderID)
	}

	if order.Status == domain.OrderStatusFilled || order.Status == domain.OrderStatusCanceled {
		return nil, fmt.Errorf("order %s is already %s", orderID, order.Status)
	}

	// Send cancel to sequencer
	select {
	case m.OrderOut <- &domain.OrderEvent{Action: domain.OrderActionCancel, Order: order}:
	default:
		log.Println("[ordermanager] WARN: order output channel full")
	}

	return order, nil
}

// GetOrder returns an order by ID.
func (m *Manager) GetOrder(orderID string) *domain.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.orders[orderID]
}

// listenExecutions processes execution events from the matching engine.
func (m *Manager) listenExecutions() {
	log.Println("[ordermanager] execution listener started")
	for {
		select {
		case event := <-m.ExecutionIn:
			m.processExecutionEvent(event)
		case <-m.done:
			log.Println("[ordermanager] execution listener stopped")
			return
		}
	}
}

// processExecutionEvent updates order states and wallet balances based on executions.
func (m *Manager) processExecutionEvent(event *domain.ExecutionEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.TakerOrder != nil {
		// Update stored order with latest state from matching engine
		if stored, exists := m.orders[event.TakerOrder.OrderID]; exists {
			stored.Status = event.TakerOrder.Status
			stored.FilledQuantity = event.TakerOrder.FilledQuantity
			stored.RemainingQuantity = event.TakerOrder.RemainingQuantity
			stored.SequenceID = event.TakerOrder.SequenceID
		}

		// Release withheld funds on cancel
		if event.TakerOrder.Status == domain.OrderStatusCanceled {
			m.releaseWithheld(event.TakerOrder)
		}
	}

	for _, exec := range event.Executions {
		m.settleExecution(exec)
	}
}

// settleExecution adjusts wallet balances for a trade.
func (m *Manager) settleExecution(exec *domain.Execution) {
	// Look up orders to find users
	takerOrder := m.orders[exec.TakerOrderID]
	makerOrder := m.orders[exec.MakerOrderID]
	if takerOrder == nil || makerOrder == nil {
		return
	}

	var buyer, seller *domain.Order
	if takerOrder.Side == domain.SideBuy {
		buyer = takerOrder
		seller = makerOrder
	} else {
		buyer = makerOrder
		seller = takerOrder
	}

	buyerWallet := m.wallets[buyer.UserID]
	sellerWallet := m.wallets[seller.UserID]
	if buyerWallet == nil || sellerWallet == nil {
		return
	}

	cost := exec.Price * exec.Quantity

	// Buyer: deduct cash, receive shares
	buyerWallet.CashBalance -= cost
	buyerWallet.Holdings[exec.Symbol] += exec.Quantity
	// Reduce withheld cash for the buyer's order
	if withheld, ok := buyerWallet.WithheldCash[buyer.OrderID]; ok {
		buyerWallet.WithheldCash[buyer.OrderID] = withheld - cost
		if buyerWallet.WithheldCash[buyer.OrderID] <= 0 {
			delete(buyerWallet.WithheldCash, buyer.OrderID)
		}
	}

	// Seller: deduct shares, receive cash
	sellerWallet.CashBalance += cost
	sellerWallet.Holdings[exec.Symbol] -= exec.Quantity
	// Reduce withheld shares for the seller's order
	if ws, ok := sellerWallet.WithheldShares[seller.OrderID]; ok {
		ws.Quantity -= exec.Quantity
		if ws.Quantity <= 0 {
			delete(sellerWallet.WithheldShares, seller.OrderID)
		} else {
			sellerWallet.WithheldShares[seller.OrderID] = ws
		}
	}

	// Update maker order state in our map
	if stored, exists := m.orders[makerOrder.OrderID]; exists {
		stored.Status = makerOrder.Status
		stored.FilledQuantity = makerOrder.FilledQuantity
		stored.RemainingQuantity = makerOrder.RemainingQuantity
	}
}

// releaseWithheld releases withheld funds/shares when an order is canceled.
func (m *Manager) releaseWithheld(order *domain.Order) {
	wallet := m.wallets[order.UserID]
	if wallet == nil {
		return
	}

	delete(wallet.WithheldCash, order.OrderID)
	delete(wallet.WithheldShares, order.OrderID)
}

func (m *Manager) totalWithheldCash(w *Wallet) int64 {
	var total int64
	for _, v := range w.WithheldCash {
		total += v
	}
	return total
}

func (m *Manager) totalWithheldShares(w *Wallet, symbol string) int64 {
	var total int64
	for _, ws := range w.WithheldShares {
		if ws.Symbol == symbol {
			total += ws.Quantity
		}
	}
	return total
}
