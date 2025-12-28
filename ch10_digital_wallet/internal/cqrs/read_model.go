package cqrs

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/nathanyu/digital-wallet/internal/domain"
	"github.com/nathanyu/digital-wallet/internal/eventstore"
)

// ReadModel provides a read-only view of wallet balances (CQRS pattern)
type ReadModel struct {
	// Read-only balances map
	balances map[string]int64
	mu       sync.RWMutex

	natsConn     *nats.Conn
	subscription *nats.Subscription

	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
}

// NewReadModel creates a new CQRS read model
func NewReadModel(natsConn *nats.Conn) *ReadModel {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReadModel{
		balances: make(map[string]int64),
		natsConn: natsConn,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// InitializeFromEventStore replays all events to rebuild the read model
func (r *ReadModel) InitializeFromEventStore(store *eventstore.EventStore) error {
	events, err := store.LoadAll()
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, event := range events {
		r.applyEvent(event)
	}

	log.Printf("Read model initialized with %d events, %d accounts", len(events), len(r.balances))
	return nil
}

// Start subscribes to the event stream
func (r *ReadModel) Start(eventSubject string) error {
	sub, err := r.natsConn.Subscribe(eventSubject, r.handleEvent)
	if err != nil {
		return err
	}

	r.subscription = sub
	log.Printf("Read model started, listening for events on: %s", eventSubject)
	return nil
}

// Stop gracefully stops the read model
func (r *ReadModel) Stop() error {
	var err error
	r.stopOnce.Do(func() {
		r.cancel()
		if r.subscription != nil {
			err = r.subscription.Unsubscribe()
		}
	})
	return err
}

// handleEvent processes events from NATS
func (r *ReadModel) handleEvent(msg *nats.Msg) {
	event, err := domain.DeserializeEvent(msg.Data)
	if err != nil {
		log.Printf("Failed to deserialize event in read model: %v", err)
		return
	}

	r.mu.Lock()
	r.applyEvent(event)
	r.mu.Unlock()
}

// HandleEventDirect processes an event directly (for direct engine integration)
func (r *ReadModel) HandleEventDirect(event domain.Event) {
	r.mu.Lock()
	r.applyEvent(event)
	r.mu.Unlock()
}

// applyEvent updates the read model based on an event
// This method is NOT thread-safe; caller must hold the lock
func (r *ReadModel) applyEvent(event domain.Event) {
	switch ev := event.(type) {
	case domain.MoneyDeducted:
		r.balances[ev.Account] -= ev.Amount
	case domain.MoneyCredited:
		r.balances[ev.Account] += ev.Amount
	case domain.TransactionFailed:
		// No state change for failed transactions
	}
}

// GetBalance returns the current balance for an account
func (r *ReadModel) GetBalance(account string) (int64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	balance, exists := r.balances[account]
	return balance, exists
}

// GetAllBalances returns a copy of all balances
func (r *ReadModel) GetAllBalances() map[string]int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]int64, len(r.balances))
	for k, v := range r.balances {
		result[k] = v
	}
	return result
}

// GetTotalBalance returns the sum of all account balances
func (r *ReadModel) GetTotalBalance() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var total int64
	for _, balance := range r.balances {
		total += balance
	}
	return total
}

// SetBalance sets the balance for an account (for initialization/testing)
func (r *ReadModel) SetBalance(account string, balance int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.balances[account] = balance
}

// BalanceResponse is the JSON response for balance queries
type BalanceResponse struct {
	Account string `json:"account"`
	Balance int64  `json:"balance"`
	Exists  bool   `json:"exists"`
}

// ToJSON returns the balance as a JSON response
func (r *ReadModel) ToJSON(account string) []byte {
	balance, exists := r.GetBalance(account)
	resp := BalanceResponse{
		Account: account,
		Balance: balance,
		Exists:  exists,
	}
	data, _ := json.Marshal(resp)
	return data
}
