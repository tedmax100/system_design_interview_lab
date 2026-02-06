package sequencer

import (
	"log"
	"sync/atomic"

	"github.com/nathanyu/stock-exchange/internal/domain"
	"github.com/nathanyu/stock-exchange/internal/matching"
)

// Sequencer stamps monotonically increasing sequence IDs on incoming orders,
// then forwards them to the matching engine. It also stamps outbound executions
// with outbound sequence IDs.
//
// This models the single-writer architecture from Figure 13.19 in the book.
type Sequencer struct {
	inboundSeq  atomic.Uint64
	outboundSeq atomic.Uint64
	engine      *matching.Engine

	// Channels for the pipeline
	OrderIn     chan *domain.OrderEvent     // inbound orders from order manager
	ExecutionOut chan *domain.ExecutionEvent // outbound executions to order manager + market data

	done chan struct{}
}

// NewSequencer creates a new sequencer wired to the given matching engine.
func NewSequencer(engine *matching.Engine, bufferSize int) *Sequencer {
	return &Sequencer{
		engine:       engine,
		OrderIn:      make(chan *domain.OrderEvent, bufferSize),
		ExecutionOut: make(chan *domain.ExecutionEvent, bufferSize),
		done:         make(chan struct{}),
	}
}

// Start begins the sequencer's application loop in a goroutine.
func (s *Sequencer) Start() {
	go s.run()
}

// Stop signals the sequencer to shut down.
func (s *Sequencer) Stop() {
	close(s.done)
}

// run is the main application loop. Single-writer consuming from OrderIn.
func (s *Sequencer) run() {
	log.Println("[sequencer] started")
	for {
		select {
		case event := <-s.OrderIn:
			s.processEvent(event)
		case <-s.done:
			log.Println("[sequencer] stopped")
			return
		}
	}
}

// processEvent stamps sequence IDs and dispatches to the matching engine.
func (s *Sequencer) processEvent(event *domain.OrderEvent) {
	// Stamp inbound sequence ID
	seq := s.inboundSeq.Add(1)
	event.Order.SequenceID = seq

	// Dispatch to matching engine (synchronous — single-threaded critical path)
	result := s.engine.HandleOrder(event)
	if result == nil {
		return
	}

	// Stamp outbound sequence IDs on executions
	for _, exec := range result.Executions {
		outSeq := s.outboundSeq.Add(1)
		exec.SequenceID = outSeq
	}

	// Send execution event downstream (non-blocking with buffered channel)
	select {
	case s.ExecutionOut <- result:
	default:
		log.Println("[sequencer] WARN: execution output channel full, dropping event")
	}
}

// CurrentInboundSeq returns the current inbound sequence number.
func (s *Sequencer) CurrentInboundSeq() uint64 {
	return s.inboundSeq.Load()
}

// CurrentOutboundSeq returns the current outbound sequence number.
func (s *Sequencer) CurrentOutboundSeq() uint64 {
	return s.outboundSeq.Load()
}
