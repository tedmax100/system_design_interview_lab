package domain

import "time"

// Side represents the order side (buy or sell).
type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

// OrderStatus represents the lifecycle state of an order.
type OrderStatus string

const (
	OrderStatusNew            OrderStatus = "new"
	OrderStatusPartiallyFilled OrderStatus = "partially_filled"
	OrderStatusFilled         OrderStatus = "filled"
	OrderStatusCanceled       OrderStatus = "canceled"
)

// OrderType represents the type of order. Only limit orders for this lab.
type OrderType string

const (
	OrderTypeLimit OrderType = "limit"
)

// Order represents a limit order in the exchange.
// Prices are in cents (int64) to avoid floating-point issues.
type Order struct {
	OrderID           string      `json:"order_id"`
	Symbol            string      `json:"symbol"`
	Side              Side        `json:"side"`
	Price             int64       `json:"price"`     // in cents, e.g. 10010 = $100.10
	Quantity          int64       `json:"quantity"`
	FilledQuantity    int64       `json:"filled_quantity"`
	RemainingQuantity int64       `json:"remaining_quantity"`
	Status            OrderStatus `json:"status"`
	UserID            string      `json:"user_id"`
	CreatedAt         time.Time   `json:"created_at"`
	SequenceID        uint64      `json:"sequence_id"`
}

// Execution represents a trade execution between two orders.
type Execution struct {
	ExecID       string    `json:"exec_id"`
	OrderID      string    `json:"order_id"`
	Symbol       string    `json:"symbol"`
	Side         Side      `json:"side"`
	Price        int64     `json:"price"`
	Quantity     int64     `json:"quantity"`
	MakerOrderID string    `json:"maker_order_id"`
	TakerOrderID string    `json:"taker_order_id"`
	Timestamp    time.Time `json:"timestamp"`
	SequenceID   uint64    `json:"sequence_id"`
}

// Candlestick represents OHLCV data for a time interval.
type Candlestick struct {
	Symbol    string    `json:"symbol"`
	Open      int64     `json:"open"`
	High      int64     `json:"high"`
	Low       int64     `json:"low"`
	Close     int64     `json:"close"`
	Volume    int64     `json:"volume"`
	Timestamp time.Time `json:"timestamp"`
	Interval  string    `json:"interval"` // e.g. "1m", "5m"
}

// L2OrderBook represents an aggregated L2 order book snapshot.
type L2OrderBook struct {
	Symbol string       `json:"symbol"`
	Bids   []PriceLevel `json:"bids"`
	Asks   []PriceLevel `json:"asks"`
}

// PriceLevel represents an aggregated price level in the L2 order book.
type PriceLevel struct {
	Price    int64 `json:"price"`
	Quantity int64 `json:"quantity"`
}

// OrderAction is the action type sent through the sequencer.
type OrderAction string

const (
	OrderActionNew    OrderAction = "new"
	OrderActionCancel OrderAction = "cancel"
)

// OrderEvent wraps an order with its action for the sequencer pipeline.
type OrderEvent struct {
	Action OrderAction
	Order  *Order
}

// ExecutionEvent wraps executions with the updated orders for downstream processing.
type ExecutionEvent struct {
	Executions []*Execution
	TakerOrder *Order
	// MakerOrders that were fully or partially filled
	MakerOrders []*Order
}
