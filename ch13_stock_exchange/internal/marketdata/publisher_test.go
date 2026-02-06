package marketdata

import (
	"testing"
	"time"

	"github.com/nathanyu/stock-exchange/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRingBuffer_Push(t *testing.T) {
	rb := &RingBuffer{}

	for i := range 5 {
		rb.Push(&domain.Candlestick{
			Open: int64(i),
		})
	}

	assert.Equal(t, 5, rb.count)
	all := rb.GetAll()
	require.Len(t, all, 5)
	assert.Equal(t, int64(0), all[0].Open)
	assert.Equal(t, int64(4), all[4].Open)
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := &RingBuffer{}

	// Push more than capacity
	for i := range ringBufferCapacity + 10 {
		rb.Push(&domain.Candlestick{
			Open: int64(i),
		})
	}

	assert.Equal(t, ringBufferCapacity, rb.count)
	all := rb.GetAll()
	require.Len(t, all, ringBufferCapacity)
	// Oldest should be index 10 (first 10 were overwritten)
	assert.Equal(t, int64(10), all[0].Open)
	assert.Equal(t, int64(ringBufferCapacity+9), all[ringBufferCapacity-1].Open)
}

func TestRingBuffer_GetRecent(t *testing.T) {
	rb := &RingBuffer{}

	for i := range 10 {
		rb.Push(&domain.Candlestick{Open: int64(i)})
	}

	recent := rb.GetRecent(3)
	require.Len(t, recent, 3)
	assert.Equal(t, int64(7), recent[0].Open)
	assert.Equal(t, int64(9), recent[2].Open)
}

func TestRingBuffer_GetRecent_MoreThanAvailable(t *testing.T) {
	rb := &RingBuffer{}
	rb.Push(&domain.Candlestick{Open: 42})

	recent := rb.GetRecent(10)
	require.Len(t, recent, 1)
	assert.Equal(t, int64(42), recent[0].Open)
}

func TestPublisher_CandlestickGeneration(t *testing.T) {
	pub := NewPublisher(100)
	now := time.Now()

	// Simulate executions
	event := &domain.ExecutionEvent{
		Executions: []*domain.Execution{
			{Symbol: "AAPL", Price: 10010, Quantity: 100, Timestamp: now},
			{Symbol: "AAPL", Price: 10020, Quantity: 200, Timestamp: now},
			{Symbol: "AAPL", Price: 10005, Quantity: 50, Timestamp: now},
		},
	}

	pub.processExecutionEvent(event)

	candles := pub.GetCandles("AAPL", 10)
	require.Len(t, candles, 1) // One building candle

	c := candles[0]
	assert.Equal(t, int64(10010), c.Open)   // First trade
	assert.Equal(t, int64(10020), c.High)   // Highest
	assert.Equal(t, int64(10005), c.Low)    // Lowest
	assert.Equal(t, int64(10005), c.Close)  // Last trade
	assert.Equal(t, int64(350), c.Volume)   // 100 + 200 + 50
}

func TestPublisher_CandlestickRotation(t *testing.T) {
	pub := NewPublisher(100)
	now := time.Now()

	// First interval
	pub.processExecutionEvent(&domain.ExecutionEvent{
		Executions: []*domain.Execution{
			{Symbol: "AAPL", Price: 10010, Quantity: 100, Timestamp: now},
		},
	})

	// Rotate
	pub.rotateCandlesticks()

	// Second interval
	pub.processExecutionEvent(&domain.ExecutionEvent{
		Executions: []*domain.Execution{
			{Symbol: "AAPL", Price: 10020, Quantity: 200, Timestamp: now.Add(time.Minute)},
		},
	})

	candles := pub.GetCandles("AAPL", 10)
	require.Len(t, candles, 2) // 1 completed + 1 building
	assert.Equal(t, int64(10010), candles[0].Open) // Completed candle
	assert.Equal(t, int64(10020), candles[1].Open) // Building candle
}

func TestPublisher_GetExecutions(t *testing.T) {
	pub := NewPublisher(100)
	now := time.Now()

	pub.processExecutionEvent(&domain.ExecutionEvent{
		Executions: []*domain.Execution{
			{Symbol: "AAPL", OrderID: "o1", TakerOrderID: "o1", MakerOrderID: "o2", Price: 10010, Quantity: 100, Timestamp: now},
			{Symbol: "GOOG", OrderID: "o3", TakerOrderID: "o3", MakerOrderID: "o4", Price: 20000, Quantity: 50, Timestamp: now},
		},
	})

	// Filter by symbol
	aapl := pub.GetExecutions("AAPL", "", time.Time{})
	assert.Len(t, aapl, 1)

	// Filter by order ID (taker)
	byOrder := pub.GetExecutions("", "o1", time.Time{})
	assert.Len(t, byOrder, 1)

	// Filter by order ID (maker)
	byMaker := pub.GetExecutions("", "o2", time.Time{})
	assert.Len(t, byMaker, 1)

	// All
	all := pub.GetExecutions("", "", time.Time{})
	assert.Len(t, all, 2)
}

func TestPublisher_GetCandles_Empty(t *testing.T) {
	pub := NewPublisher(100)
	candles := pub.GetCandles("AAPL", 10)
	assert.Empty(t, candles)
}

func TestPublisher_MultipleSymbols(t *testing.T) {
	pub := NewPublisher(100)
	now := time.Now()

	pub.processExecutionEvent(&domain.ExecutionEvent{
		Executions: []*domain.Execution{
			{Symbol: "AAPL", Price: 10010, Quantity: 100, Timestamp: now},
			{Symbol: "GOOG", Price: 20000, Quantity: 50, Timestamp: now},
		},
	})

	aapl := pub.GetCandles("AAPL", 10)
	goog := pub.GetCandles("GOOG", 10)

	require.Len(t, aapl, 1)
	require.Len(t, goog, 1)
	assert.Equal(t, int64(10010), aapl[0].Open)
	assert.Equal(t, int64(20000), goog[0].Open)
}
