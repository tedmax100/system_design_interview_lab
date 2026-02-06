package orderbook

import (
	"container/list"
	"fmt"
	"sort"

	"github.com/nathanyu/stock-exchange/internal/domain"
)

// orderEntry maps an order to its linked list element for O(1) cancel.
type orderEntry struct {
	order   *domain.Order
	element *list.Element
	level   *bookLevel
}

// bookLevel is a price level in one side of the book.
// It holds a doubly-linked list of orders at this price (FIFO).
type bookLevel struct {
	Price       int64
	TotalVolume int64
	Orders      *list.List // of *domain.Order
}

// Book represents one side (buy or sell) of an order book.
type Book struct {
	Side      domain.Side
	LimitMap  map[int64]*bookLevel // price -> level
	bestPrice int64                // best bid (highest buy) or best ask (lowest sell)
	hasOrders bool
}

// NewBook creates a new order book side.
func NewBook(side domain.Side) *Book {
	return &Book{
		Side:     side,
		LimitMap: make(map[int64]*bookLevel),
	}
}

// BestPrice returns the best price on this side, or 0 if empty.
func (b *Book) BestPrice() int64 {
	if !b.hasOrders {
		return 0
	}
	return b.bestPrice
}

// HasOrders returns whether this side has any resting orders.
func (b *Book) HasOrders() bool {
	return b.hasOrders
}

// addOrder appends an order to the tail of the price level's linked list.
func (b *Book) addOrder(order *domain.Order) *list.Element {
	level, exists := b.LimitMap[order.Price]
	if !exists {
		level = &bookLevel{
			Price:  order.Price,
			Orders: list.New(),
		}
		b.LimitMap[order.Price] = level
	}

	level.TotalVolume += order.RemainingQuantity
	elem := level.Orders.PushBack(order)

	b.refreshBestPrice()
	return elem
}

// removeOrder removes an order from its price level.
func (b *Book) removeOrder(entry *orderEntry) {
	level := entry.level
	level.Orders.Remove(entry.element)
	level.TotalVolume -= entry.order.RemainingQuantity

	if level.Orders.Len() == 0 {
		delete(b.LimitMap, level.Price)
	}

	b.refreshBestPrice()
}

// refreshBestPrice recalculates the best price.
func (b *Book) refreshBestPrice() {
	if len(b.LimitMap) == 0 {
		b.hasOrders = false
		b.bestPrice = 0
		return
	}

	b.hasOrders = true
	if b.Side == domain.SideBuy {
		// Best bid = highest price
		best := int64(0)
		for price := range b.LimitMap {
			if price > best {
				best = price
			}
		}
		b.bestPrice = best
	} else {
		// Best ask = lowest price
		best := int64(1<<62 - 1)
		for price := range b.LimitMap {
			if price < best {
				best = price
			}
		}
		b.bestPrice = best
	}
}

// OrderBook holds the full two-sided order book for a single symbol.
type OrderBook struct {
	Symbol   string
	BuyBook  *Book
	SellBook *Book
	OrderMap map[string]*orderEntry // orderID -> entry for O(1) lookup/cancel
}

// NewOrderBook creates a new order book for a symbol.
func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		Symbol:   symbol,
		BuyBook:  NewBook(domain.SideBuy),
		SellBook: NewBook(domain.SideSell),
		OrderMap: make(map[string]*orderEntry),
	}
}

// AddOrder adds a resting order to the appropriate side of the book.
func (ob *OrderBook) AddOrder(order *domain.Order) {
	var book *Book
	if order.Side == domain.SideBuy {
		book = ob.BuyBook
	} else {
		book = ob.SellBook
	}

	elem := book.addOrder(order)
	level := book.LimitMap[order.Price]
	ob.OrderMap[order.OrderID] = &orderEntry{
		order:   order,
		element: elem,
		level:   level,
	}
}

// CancelOrder removes an order from the book by ID. Returns the order if found, nil otherwise.
func (ob *OrderBook) CancelOrder(orderID string) *domain.Order {
	entry, exists := ob.OrderMap[orderID]
	if !exists {
		return nil
	}

	var book *Book
	if entry.order.Side == domain.SideBuy {
		book = ob.BuyBook
	} else {
		book = ob.SellBook
	}

	book.removeOrder(entry)
	delete(ob.OrderMap, orderID)

	entry.order.Status = domain.OrderStatusCanceled
	return entry.order
}

// MatchOrder attempts to match an incoming order against the opposite side.
// Returns a list of executions and whether the taker order has remaining quantity.
func (ob *OrderBook) MatchOrder(taker *domain.Order) []*domain.Execution {
	var oppositeBook *Book
	if taker.Side == domain.SideBuy {
		oppositeBook = ob.SellBook
	} else {
		oppositeBook = ob.BuyBook
	}

	var executions []*domain.Execution
	execSeq := 0

	for taker.RemainingQuantity > 0 && oppositeBook.HasOrders() {
		bestPrice := oppositeBook.BestPrice()

		// Check price match
		if taker.Side == domain.SideBuy && taker.Price < bestPrice {
			break // buy price too low
		}
		if taker.Side == domain.SideSell && taker.Price > bestPrice {
			break // sell price too high
		}

		level := oppositeBook.LimitMap[bestPrice]

		// FIFO: consume from head of the linked list at this price level
		for taker.RemainingQuantity > 0 && level.Orders.Len() > 0 {
			front := level.Orders.Front()
			maker := front.Value.(*domain.Order)

			matchQty := min(taker.RemainingQuantity, maker.RemainingQuantity)

			// Update quantities
			taker.FilledQuantity += matchQty
			taker.RemainingQuantity -= matchQty
			maker.FilledQuantity += matchQty
			maker.RemainingQuantity -= matchQty

			// Update level volume
			level.TotalVolume -= matchQty

			// Update statuses
			if maker.RemainingQuantity == 0 {
				maker.Status = domain.OrderStatusFilled
				level.Orders.Remove(front)
				delete(ob.OrderMap, maker.OrderID)
			} else {
				maker.Status = domain.OrderStatusPartiallyFilled
			}

			if taker.RemainingQuantity == 0 {
				taker.Status = domain.OrderStatusFilled
			} else {
				taker.Status = domain.OrderStatusPartiallyFilled
			}

			execSeq++
			exec := &domain.Execution{
				ExecID:       fmt.Sprintf("%s-exec-%d", taker.OrderID, execSeq),
				OrderID:      taker.OrderID,
				Symbol:       taker.Symbol,
				Side:         taker.Side,
				Price:        maker.Price, // execute at maker's (resting) price
				Quantity:     matchQty,
				MakerOrderID: maker.OrderID,
				TakerOrderID: taker.OrderID,
			}
			executions = append(executions, exec)
		}

		// Clean up empty price level
		if level.Orders.Len() == 0 {
			delete(oppositeBook.LimitMap, bestPrice)
			oppositeBook.refreshBestPrice()
		}
	}

	return executions
}

// GetL2Snapshot returns an aggregated L2 order book snapshot.
func (ob *OrderBook) GetL2Snapshot(depth int) *domain.L2OrderBook {
	snapshot := &domain.L2OrderBook{
		Symbol: ob.Symbol,
		Bids:   aggregateLevels(ob.BuyBook, depth, true),
		Asks:   aggregateLevels(ob.SellBook, depth, false),
	}
	return snapshot
}

// aggregateLevels collects price levels sorted by price.
// For bids: descending (highest first). For asks: ascending (lowest first).
func aggregateLevels(book *Book, depth int, descending bool) []domain.PriceLevel {
	prices := make([]int64, 0, len(book.LimitMap))
	for price := range book.LimitMap {
		prices = append(prices, price)
	}

	if descending {
		sort.Slice(prices, func(i, j int) bool { return prices[i] > prices[j] })
	} else {
		sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
	}

	if depth > 0 && len(prices) > depth {
		prices = prices[:depth]
	}

	levels := make([]domain.PriceLevel, len(prices))
	for i, price := range prices {
		level := book.LimitMap[price]
		levels[i] = domain.PriceLevel{
			Price:    price,
			Quantity: level.TotalVolume,
		}
	}
	return levels
}
