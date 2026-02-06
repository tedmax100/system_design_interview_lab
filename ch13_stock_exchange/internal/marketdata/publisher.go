package marketdata

import (
	"log"
	"sync"
	"time"

	"github.com/nathanyu/stock-exchange/internal/domain"
)

const (
	ringBufferCapacity = 100
	defaultInterval    = "1m"
)

// candleState tracks the current (building) candlestick for a symbol.
type candleState struct {
	current  *domain.Candlestick
	hasData  bool
	interval time.Duration
}

// RingBuffer is a fixed-size circular buffer of candlesticks.
type RingBuffer struct {
	data  [ringBufferCapacity]*domain.Candlestick
	head  int // next write position
	count int
}

// Push adds a candlestick to the ring buffer.
func (rb *RingBuffer) Push(c *domain.Candlestick) {
	rb.data[rb.head] = c
	rb.head = (rb.head + 1) % ringBufferCapacity
	if rb.count < ringBufferCapacity {
		rb.count++
	}
}

// GetAll returns all candlesticks in chronological order.
func (rb *RingBuffer) GetAll() []*domain.Candlestick {
	if rb.count == 0 {
		return nil
	}

	result := make([]*domain.Candlestick, rb.count)
	start := (rb.head - rb.count + ringBufferCapacity) % ringBufferCapacity
	for i := range rb.count {
		idx := (start + i) % ringBufferCapacity
		result[i] = rb.data[idx]
	}
	return result
}

// GetRecent returns the N most recent candlesticks.
func (rb *RingBuffer) GetRecent(n int) []*domain.Candlestick {
	if n <= 0 || rb.count == 0 {
		return nil
	}
	if n > rb.count {
		n = rb.count
	}

	result := make([]*domain.Candlestick, n)
	start := (rb.head - n + ringBufferCapacity) % ringBufferCapacity
	for i := range n {
		idx := (start + i) % ringBufferCapacity
		result[i] = rb.data[idx]
	}
	return result
}

// Publisher receives executions and maintains candlestick data and L2 snapshots.
type Publisher struct {
	mu sync.RWMutex

	// Per-symbol candlestick ring buffers (completed candles)
	candles map[string]*RingBuffer

	// Per-symbol current (building) candle state
	states map[string]*candleState

	// Execution log (for querying)
	executions []*domain.Execution

	// Channel to receive execution events
	ExecutionIn chan *domain.ExecutionEvent

	done   chan struct{}
	ticker *time.Ticker
}

// NewPublisher creates a new market data publisher.
func NewPublisher(bufferSize int) *Publisher {
	return &Publisher{
		candles:     make(map[string]*RingBuffer),
		states:      make(map[string]*candleState),
		ExecutionIn: make(chan *domain.ExecutionEvent, bufferSize),
		done:        make(chan struct{}),
	}
}

// Start begins the publisher's application loop.
func (p *Publisher) Start() {
	p.ticker = time.NewTicker(1 * time.Minute)
	go p.run()
}

// Stop shuts down the publisher.
func (p *Publisher) Stop() {
	if p.ticker != nil {
		p.ticker.Stop()
	}
	close(p.done)
}

// run is the main application loop.
func (p *Publisher) run() {
	log.Println("[marketdata] publisher started")
	for {
		select {
		case event := <-p.ExecutionIn:
			p.processExecutionEvent(event)
		case <-p.ticker.C:
			p.rotateCandlesticks()
		case <-p.done:
			log.Println("[marketdata] publisher stopped")
			return
		}
	}
}

// processExecutionEvent updates candlestick data from executions.
func (p *Publisher) processExecutionEvent(event *domain.ExecutionEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, exec := range event.Executions {
		p.executions = append(p.executions, exec)
		p.updateCandle(exec)
	}
}

// updateCandle updates the current candlestick for a symbol based on an execution.
func (p *Publisher) updateCandle(exec *domain.Execution) {
	state, exists := p.states[exec.Symbol]
	if !exists {
		state = &candleState{
			interval: 1 * time.Minute,
		}
		p.states[exec.Symbol] = state
	}

	if !state.hasData {
		// First trade in this interval
		state.current = &domain.Candlestick{
			Symbol:    exec.Symbol,
			Open:      exec.Price,
			High:      exec.Price,
			Low:       exec.Price,
			Close:     exec.Price,
			Volume:    exec.Quantity,
			Timestamp: exec.Timestamp.Truncate(state.interval),
			Interval:  defaultInterval,
		}
		state.hasData = true
		return
	}

	c := state.current
	if exec.Price > c.High {
		c.High = exec.Price
	}
	if exec.Price < c.Low {
		c.Low = exec.Price
	}
	c.Close = exec.Price
	c.Volume += exec.Quantity
}

// rotateCandlesticks closes the current candle and starts a new interval.
func (p *Publisher) rotateCandlesticks() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for symbol, state := range p.states {
		if !state.hasData {
			continue
		}

		// Push completed candle to ring buffer
		rb, exists := p.candles[symbol]
		if !exists {
			rb = &RingBuffer{}
			p.candles[symbol] = rb
		}
		rb.Push(state.current)

		// Reset state for next interval
		state.hasData = false
		state.current = nil
	}
}

// GetCandles returns recent candlesticks for a symbol.
func (p *Publisher) GetCandles(symbol string, count int) []*domain.Candlestick {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []*domain.Candlestick

	// Include completed candles from ring buffer
	if rb, exists := p.candles[symbol]; exists {
		result = rb.GetRecent(count)
	}

	// Include current building candle if it has data
	if state, exists := p.states[symbol]; exists && state.hasData {
		result = append(result, state.current)
	}

	return result
}

// GetExecutions returns executions matching the filter criteria.
func (p *Publisher) GetExecutions(symbol, orderID string, since time.Time) []*domain.Execution {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []*domain.Execution
	for _, exec := range p.executions {
		if symbol != "" && exec.Symbol != symbol {
			continue
		}
		if orderID != "" && exec.OrderID != orderID && exec.MakerOrderID != orderID && exec.TakerOrderID != orderID {
			continue
		}
		if !since.IsZero() && exec.Timestamp.Before(since) {
			continue
		}
		result = append(result, exec)
	}
	return result
}
