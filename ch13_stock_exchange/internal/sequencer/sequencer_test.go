package sequencer

import (
	"testing"
	"time"

	"github.com/nathanyu/stock-exchange/internal/domain"
	"github.com/nathanyu/stock-exchange/internal/matching"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequencer_StampsSequenceIDs(t *testing.T) {
	engine := matching.NewEngine()
	seq := NewSequencer(engine, 100)
	seq.Start()
	defer seq.Stop()

	// Send 3 orders
	for i := range 3 {
		order := &domain.Order{
			OrderID:           "o" + string(rune('1'+i)),
			Symbol:            "AAPL",
			Side:              domain.SideSell,
			Price:             10010,
			Quantity:          100,
			RemainingQuantity: 100,
			Status:            domain.OrderStatusNew,
			UserID:            "user1",
		}
		seq.OrderIn <- &domain.OrderEvent{Action: domain.OrderActionNew, Order: order}
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, uint64(3), seq.CurrentInboundSeq())
}

func TestSequencer_MonotonicIDs(t *testing.T) {
	engine := matching.NewEngine()
	seq := NewSequencer(engine, 100)
	seq.Start()
	defer seq.Stop()

	// Place a sell, then a matching buy to generate executions
	sell := &domain.Order{
		OrderID:           "s1",
		Symbol:            "AAPL",
		Side:              domain.SideSell,
		Price:             10010,
		Quantity:          100,
		RemainingQuantity: 100,
		Status:            domain.OrderStatusNew,
		UserID:            "user1",
	}
	seq.OrderIn <- &domain.OrderEvent{Action: domain.OrderActionNew, Order: sell}

	time.Sleep(20 * time.Millisecond)

	buy := &domain.Order{
		OrderID:           "b1",
		Symbol:            "AAPL",
		Side:              domain.SideBuy,
		Price:             10010,
		Quantity:          100,
		RemainingQuantity: 100,
		Status:            domain.OrderStatusNew,
		UserID:            "user2",
	}
	seq.OrderIn <- &domain.OrderEvent{Action: domain.OrderActionNew, Order: buy}

	// Read execution events
	var events []*domain.ExecutionEvent
	timeout := time.After(200 * time.Millisecond)
	for {
		select {
		case evt := <-seq.ExecutionOut:
			events = append(events, evt)
		case <-timeout:
			goto done
		}
	}
done:
	assert.Equal(t, uint64(2), seq.CurrentInboundSeq())

	// Find event with executions
	var execEvent *domain.ExecutionEvent
	for _, e := range events {
		if len(e.Executions) > 0 {
			execEvent = e
			break
		}
	}
	require.NotNil(t, execEvent)
	assert.Equal(t, uint64(1), execEvent.Executions[0].SequenceID)
}
