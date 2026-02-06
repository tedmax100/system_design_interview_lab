package matching

import (
	"time"

	"github.com/nathanyu/stock-exchange/internal/domain"
	"github.com/nathanyu/stock-exchange/internal/orderbook"
)

// Engine is the matching engine. It maintains per-symbol order books and
// dispatches incoming orders for matching.
type Engine struct {
	books map[string]*orderbook.OrderBook // symbol -> order book
}

// NewEngine creates a new matching engine.
func NewEngine() *Engine {
	return &Engine{
		books: make(map[string]*orderbook.OrderBook),
	}
}

// getOrCreateBook returns the order book for a symbol, creating it if needed.
func (e *Engine) getOrCreateBook(symbol string) *orderbook.OrderBook {
	book, exists := e.books[symbol]
	if !exists {
		book = orderbook.NewOrderBook(symbol)
		e.books[symbol] = book
	}
	return book
}

// HandleOrder processes an order event (new or cancel) and returns any resulting executions.
func (e *Engine) HandleOrder(event *domain.OrderEvent) *domain.ExecutionEvent {
	switch event.Action {
	case domain.OrderActionNew:
		return e.handleNew(event.Order)
	case domain.OrderActionCancel:
		return e.handleCancel(event.Order)
	default:
		return nil
	}
}

// handleNew processes a new order: match against opposite side, then rest remainder.
func (e *Engine) handleNew(order *domain.Order) *domain.ExecutionEvent {
	book := e.getOrCreateBook(order.Symbol)
	now := time.Now()

	// Attempt to match
	executions := book.MatchOrder(order)

	// Stamp timestamps on executions
	for _, exec := range executions {
		exec.Timestamp = now
	}

	// Collect affected maker orders
	makerOrders := make([]*domain.Order, 0, len(executions))
	seen := make(map[string]bool)
	for _, exec := range executions {
		if !seen[exec.MakerOrderID] {
			seen[exec.MakerOrderID] = true
			// Look up maker order from the book's order map if still there,
			// or we already have it from the execution
		}
	}

	// If order has remaining quantity, add it as a resting order
	if order.RemainingQuantity > 0 {
		if order.Status == domain.OrderStatusNew {
			order.Status = domain.OrderStatusNew
		}
		book.AddOrder(order)
	}

	if len(executions) == 0 {
		return &domain.ExecutionEvent{
			TakerOrder: order,
		}
	}

	return &domain.ExecutionEvent{
		Executions:  executions,
		TakerOrder:  order,
		MakerOrders: makerOrders,
	}
}

// handleCancel cancels an existing order.
func (e *Engine) handleCancel(order *domain.Order) *domain.ExecutionEvent {
	book := e.getOrCreateBook(order.Symbol)
	canceled := book.CancelOrder(order.OrderID)
	if canceled != nil {
		return &domain.ExecutionEvent{
			TakerOrder: canceled,
		}
	}
	return &domain.ExecutionEvent{
		TakerOrder: order,
	}
}

// GetOrderBook returns the order book for a symbol (nil if it doesn't exist).
func (e *Engine) GetOrderBook(symbol string) *orderbook.OrderBook {
	return e.books[symbol]
}

// GetL2Snapshot returns an L2 snapshot for a symbol.
func (e *Engine) GetL2Snapshot(symbol string, depth int) *domain.L2OrderBook {
	book := e.books[symbol]
	if book == nil {
		return &domain.L2OrderBook{
			Symbol: symbol,
			Bids:   []domain.PriceLevel{},
			Asks:   []domain.PriceLevel{},
		}
	}
	return book.GetL2Snapshot(depth)
}
