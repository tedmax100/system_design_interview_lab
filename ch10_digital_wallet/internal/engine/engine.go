package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nathanyu/digital-wallet/internal/domain"
	"github.com/nathanyu/digital-wallet/internal/eventstore"
	"github.com/nathanyu/digital-wallet/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	CommandSubject = "wallet.commands"
	EventSubject   = "wallet.events"
)

// WalletEngine is the deterministic state machine for processing wallet commands
type WalletEngine struct {
	// Current state: account -> balance (in cents)
	balances map[string]int64
	// Track processed transactions for idempotency
	processedTxns map[string]bool

	eventStore    *eventstore.EventStore
	natsConn      *nats.Conn
	subscription  *nats.Subscription
	eventHandlers []EventHandler

	mu       sync.RWMutex
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
}

// EventHandler is a function that handles events (for CQRS)
type EventHandler func(event domain.Event)

// NewWalletEngine creates a new wallet engine
func NewWalletEngine(eventStore *eventstore.EventStore, natsConn *nats.Conn) *WalletEngine {
	ctx, cancel := context.WithCancel(context.Background())
	return &WalletEngine{
		balances:      make(map[string]int64),
		processedTxns: make(map[string]bool),
		eventStore:    eventStore,
		natsConn:      natsConn,
		eventHandlers: make([]EventHandler, 0),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// RegisterEventHandler registers a handler to receive events
func (e *WalletEngine) RegisterEventHandler(handler EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eventHandlers = append(e.eventHandlers, handler)
}

// InitializeFromEventStore replays all events from the event store to rebuild state
func (e *WalletEngine) InitializeFromEventStore() error {
	events, err := e.eventStore.LoadAll()
	if err != nil {
		return fmt.Errorf("failed to load events: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for _, event := range events {
		e.applyEvent(event)
	}

	log.Printf("Wallet engine initialized with %d events, %d accounts", len(events), len(e.balances))
	return nil
}

// Start begins processing commands from NATS
func (e *WalletEngine) Start() error {
	sub, err := e.natsConn.Subscribe(CommandSubject, e.handleCommand)
	if err != nil {
		return fmt.Errorf("failed to subscribe to commands: %w", err)
	}

	e.subscription = sub
	log.Printf("Wallet engine started, listening on subject: %s", CommandSubject)
	return nil
}

// Stop gracefully stops the engine
func (e *WalletEngine) Stop() error {
	var err error
	e.stopOnce.Do(func() {
		e.cancel()

		if e.subscription != nil {
			err = e.subscription.Unsubscribe()
		}

		e.wg.Wait()
	})
	return err
}

// handleCommand processes a single command from NATS
func (e *WalletEngine) handleCommand(msg *nats.Msg) {
	e.wg.Add(1)
	defer e.wg.Done()

	start := time.Now()
	ctx := e.ctx

	// Start tracing span
	if telemetry.Tracer != nil {
		var span trace.Span
		ctx, span = telemetry.Tracer.Start(ctx, "engine.handleCommand",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("messaging.system", "nats"),
				attribute.String("messaging.destination", CommandSubject),
			),
		)
		defer span.End()
	}

	// Record NATS message received
	telemetry.NATSMessagesReceived.WithLabelValues(CommandSubject).Inc()

	var cmd domain.TransferCommand
	if err := json.Unmarshal(msg.Data, &cmd); err != nil {
		log.Printf("Failed to unmarshal command: %v", err)
		e.respondError(msg, "invalid command format")
		return
	}

	// Add command attributes to span
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.SetAttributes(
			attribute.String("transaction_id", cmd.TransactionID),
			attribute.String("from_account", cmd.FromAccount),
			attribute.String("to_account", cmd.ToAccount),
			attribute.Int64("amount", cmd.Amount),
		)
	}

	// Process the command
	events, err := e.ExecuteWithContext(ctx, cmd)
	if err != nil {
		log.Printf("Failed to execute command: %v", err)
		if span := trace.SpanFromContext(ctx); span.IsRecording() {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		e.respondError(msg, err.Error())
		return
	}

	// Persist events
	persistStart := time.Now()
	if err := e.eventStore.AppendBatch(events); err != nil {
		log.Printf("Failed to persist events: %v", err)
		if span := trace.SpanFromContext(ctx); span.IsRecording() {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to persist events")
		}
		e.respondError(msg, "failed to persist events")
		return
	}
	telemetry.EventStoreWriteDuration.Observe(time.Since(persistStart).Seconds())

	// Record event metrics
	for _, event := range events {
		telemetry.EventsStoredTotal.WithLabelValues(event.GetType()).Inc()
	}

	// Apply events to update state
	e.mu.Lock()
	for _, event := range events {
		e.applyEvent(event)
	}
	e.mu.Unlock()

	// Notify event handlers (for CQRS)
	e.notifyEventHandlers(events)

	// Publish events to NATS for other subscribers
	e.publishEvents(events)

	// Record transfer metrics
	telemetry.TransferProcessingDuration.Observe(time.Since(start).Seconds())
	e.recordTransferMetrics(events, cmd.Amount)

	// Update balance metrics
	e.updateBalanceMetrics()

	// Respond with success
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.SetStatus(codes.Ok, "")
		span.SetAttributes(attribute.Int("events_count", len(events)))
	}
	e.respondSuccess(msg, events)
}

// Execute processes a command and generates events without modifying state
func (e *WalletEngine) Execute(cmd domain.TransferCommand) ([]domain.Event, error) {
	return e.ExecuteWithContext(context.Background(), cmd)
}

// ExecuteWithContext processes a command with tracing context
func (e *WalletEngine) ExecuteWithContext(ctx context.Context, cmd domain.TransferCommand) ([]domain.Event, error) {
	// Start tracing span
	if telemetry.Tracer != nil {
		var span trace.Span
		ctx, span = telemetry.Tracer.Start(ctx, "engine.Execute",
			trace.WithAttributes(
				attribute.String("transaction_id", cmd.TransactionID),
				attribute.Int64("amount", cmd.Amount),
			),
		)
		defer span.End()
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Check for idempotency
	if e.processedTxns[cmd.TransactionID] {
		log.Printf("Transaction %s already processed, skipping", cmd.TransactionID)
		telemetry.DuplicateTransactionsTotal.Inc()
		if span := trace.SpanFromContext(ctx); span.IsRecording() {
			span.SetAttributes(attribute.Bool("duplicate", true))
		}
		return []domain.Event{}, nil
	}

	// Validate command
	if cmd.Amount <= 0 {
		return []domain.Event{
			domain.TransactionFailed{
				TransactionID: cmd.TransactionID,
				FromAccount:   cmd.FromAccount,
				Reason:        "amount must be positive",
			},
		}, nil
	}

	if cmd.FromAccount == cmd.ToAccount {
		return []domain.Event{
			domain.TransactionFailed{
				TransactionID: cmd.TransactionID,
				FromAccount:   cmd.FromAccount,
				Reason:        "cannot transfer to same account",
			},
		}, nil
	}

	// Check balance
	fromBalance := e.balances[cmd.FromAccount]
	if fromBalance < cmd.Amount {
		if span := trace.SpanFromContext(ctx); span.IsRecording() {
			span.SetAttributes(
				attribute.String("failure_reason", "insufficient_funds"),
				attribute.Int64("current_balance", fromBalance),
			)
		}
		return []domain.Event{
			domain.TransactionFailed{
				TransactionID: cmd.TransactionID,
				FromAccount:   cmd.FromAccount,
				Reason:        "insufficient funds",
			},
		}, nil
	}

	// Generate success events
	events := []domain.Event{
		domain.MoneyDeducted{
			TransactionID: cmd.TransactionID,
			Account:       cmd.FromAccount,
			Amount:        cmd.Amount,
		},
		domain.MoneyCredited{
			TransactionID: cmd.TransactionID,
			Account:       cmd.ToAccount,
			Amount:        cmd.Amount,
		},
	}

	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.SetAttributes(attribute.Bool("success", true))
	}

	return events, nil
}

// recordTransferMetrics records metrics for a transfer
func (e *WalletEngine) recordTransferMetrics(events []domain.Event, amount int64) {
	for _, event := range events {
		switch event.(type) {
		case domain.MoneyDeducted:
			telemetry.TransfersTotal.WithLabelValues("success").Inc()
			telemetry.TransferAmount.WithLabelValues("success").Observe(float64(amount))
		case domain.TransactionFailed:
			ev := event.(domain.TransactionFailed)
			if ev.Reason == "insufficient funds" {
				telemetry.TransfersTotal.WithLabelValues("insufficient_funds").Inc()
			} else {
				telemetry.TransfersTotal.WithLabelValues("failed").Inc()
			}
			telemetry.TransferAmount.WithLabelValues("failed").Observe(float64(amount))
		}
	}
}

// updateBalanceMetrics updates the balance gauge metrics
func (e *WalletEngine) updateBalanceMetrics() {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var total int64
	for account, balance := range e.balances {
		telemetry.AccountBalanceGauge.WithLabelValues(account).Set(float64(balance))
		total += balance
	}
	telemetry.TotalBalanceGauge.Set(float64(total))
	telemetry.AccountCount.Set(float64(len(e.balances)))
}

// applyEvent updates the internal state based on an event
// This method is NOT thread-safe; caller must hold the lock
func (e *WalletEngine) applyEvent(event domain.Event) {
	switch ev := event.(type) {
	case domain.MoneyDeducted:
		e.balances[ev.Account] -= ev.Amount
		e.processedTxns[ev.TransactionID] = true
	case domain.MoneyCredited:
		e.balances[ev.Account] += ev.Amount
	case domain.TransactionFailed:
		e.processedTxns[ev.TransactionID] = true
	}
}

// ApplyEvents applies a batch of events to update internal state (for testing)
func (e *WalletEngine) ApplyEvents(events []domain.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, event := range events {
		e.applyEvent(event)
	}
}

// notifyEventHandlers sends events to all registered handlers
func (e *WalletEngine) notifyEventHandlers(events []domain.Event) {
	e.mu.RLock()
	handlers := make([]EventHandler, len(e.eventHandlers))
	copy(handlers, e.eventHandlers)
	e.mu.RUnlock()

	for _, event := range events {
		for _, handler := range handlers {
			handler(event)
		}
	}
}

// publishEvents publishes events to NATS for other subscribers
func (e *WalletEngine) publishEvents(events []domain.Event) {
	for _, event := range events {
		data, err := domain.SerializeEvent(event)
		if err != nil {
			log.Printf("Failed to serialize event for publishing: %v", err)
			continue
		}

		if err := e.natsConn.Publish(EventSubject, data); err != nil {
			log.Printf("Failed to publish event: %v", err)
		}
	}
}

// CommandResponse represents the response to a command
type CommandResponse struct {
	Success bool     `json:"success"`
	Error   string   `json:"error,omitempty"`
	Events  []string `json:"events,omitempty"`
}

func (e *WalletEngine) respondSuccess(msg *nats.Msg, events []domain.Event) {
	eventTypes := make([]string, len(events))
	for i, ev := range events {
		eventTypes[i] = ev.GetType()
	}

	resp := CommandResponse{
		Success: true,
		Events:  eventTypes,
	}

	data, _ := json.Marshal(resp)
	if msg.Reply != "" {
		msg.Respond(data)
	}
}

func (e *WalletEngine) respondError(msg *nats.Msg, errMsg string) {
	resp := CommandResponse{
		Success: false,
		Error:   errMsg,
	}

	data, _ := json.Marshal(resp)
	if msg.Reply != "" {
		msg.Respond(data)
	}
}

// GetBalance returns the current balance for an account (for testing)
func (e *WalletEngine) GetBalance(account string) int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.balances[account]
}

// SetBalance sets the balance for an account (for testing/initialization)
func (e *WalletEngine) SetBalance(account string, balance int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.balances[account] = balance
}

// GetAllBalances returns a copy of all balances (for testing)
func (e *WalletEngine) GetAllBalances() map[string]int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]int64, len(e.balances))
	for k, v := range e.balances {
		result[k] = v
	}
	return result
}

// GetTotalBalance returns the sum of all account balances
func (e *WalletEngine) GetTotalBalance() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var total int64
	for _, balance := range e.balances {
		total += balance
	}
	return total
}
