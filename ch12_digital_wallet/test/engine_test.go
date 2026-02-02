package test

import (
	"os"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nathanyu/digital-wallet/internal/domain"
	"github.com/nathanyu/digital-wallet/internal/engine"
	"github.com/nathanyu/digital-wallet/internal/eventstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper to create a test engine
func setupTestEngine(t *testing.T) (*engine.WalletEngine, *eventstore.EventStore, func()) {
	// Create temp file for event store
	tmpFile, err := os.CreateTemp("", "events-*.log")
	require.NoError(t, err)
	tmpFile.Close()

	store, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)

	// Create mock NATS connection (use embedded server for real tests)
	nc, err := nats.Connect(nats.DefaultURL, nats.NoReconnect())
	if err != nil {
		// Skip NATS-dependent tests if no server available
		t.Skip("NATS server not available")
	}

	eng := engine.NewWalletEngine(store, nc)

	cleanup := func() {
		eng.Stop()
		store.Close()
		nc.Close()
		os.Remove(tmpFile.Name())
	}

	return eng, store, cleanup
}

// AC2: Business Validation Test
// Test that an account with initial balance of 100, making 10 transfers of 20 each,
// results in 5 successful deductions and 5 failures (insufficient funds)
func TestBusinessValidation_InsufficientFunds(t *testing.T) {
	// Create temp file for event store
	tmpFile, err := os.CreateTemp("", "events-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	store, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)
	defer store.Close()

	// Create engine without NATS (we'll call Execute directly)
	eng := engine.NewWalletEngine(store, nil)

	// Set initial balance: 100 cents
	eng.SetBalance("sender", 100)

	var successCount, failCount int
	var allEvents []domain.Event

	// Execute 10 transfers of 20 cents each
	for i := 0; i < 10; i++ {
		cmd := domain.TransferCommand{
			TransactionID: generateTestTxnID(i),
			FromAccount:   "sender",
			ToAccount:     "receiver",
			Amount:        20,
		}

		events, err := eng.Execute(cmd)
		require.NoError(t, err)

		// Store events
		err = store.AppendBatch(events)
		require.NoError(t, err)

		// Apply events to engine state (simulating what happens in handleCommand)
		for _, event := range events {
			switch event.(type) {
			case domain.MoneyDeducted:
				successCount++
			case domain.TransactionFailed:
				failCount++
			}
			allEvents = append(allEvents, event)
		}

		// Manually apply to update balance state
		applyEventsToEngine(eng, events)
	}

	// Verify: 5 successful, 5 failed
	assert.Equal(t, 5, successCount, "Expected 5 successful deductions")
	assert.Equal(t, 5, failCount, "Expected 5 failed transactions")

	// Verify final balance is 0, not negative
	assert.Equal(t, int64(0), eng.GetBalance("sender"), "Sender balance should be 0")
	assert.Equal(t, int64(100), eng.GetBalance("receiver"), "Receiver should have 100")
}

// AC4: Idempotency Test
// Test that duplicate transactions are only processed once
func TestIdempotency_DuplicateTransactions(t *testing.T) {
	// Create temp file for event store
	tmpFile, err := os.CreateTemp("", "events-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	store, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)
	defer store.Close()

	eng := engine.NewWalletEngine(store, nil)

	// Set initial balance
	eng.SetBalance("alice", 1000)

	txnID := "test-txn-123"
	cmd := domain.TransferCommand{
		TransactionID: txnID,
		FromAccount:   "alice",
		ToAccount:     "bob",
		Amount:        100,
	}

	// First execution - should succeed
	events1, err := eng.Execute(cmd)
	require.NoError(t, err)
	assert.Len(t, events1, 2, "First execution should produce 2 events")

	// Store and apply events
	err = store.AppendBatch(events1)
	require.NoError(t, err)
	applyEventsToEngine(eng, events1)

	// Verify balance after first transfer
	assert.Equal(t, int64(900), eng.GetBalance("alice"))
	assert.Equal(t, int64(100), eng.GetBalance("bob"))

	// Second execution with same transaction ID - should be skipped
	events2, err := eng.Execute(cmd)
	require.NoError(t, err)
	assert.Len(t, events2, 0, "Duplicate transaction should produce no events")

	// Verify balance unchanged after duplicate
	assert.Equal(t, int64(900), eng.GetBalance("alice"))
	assert.Equal(t, int64(100), eng.GetBalance("bob"))
}

// AC3: Reproducibility Test
// Test that replaying events from event store reproduces the exact same state
func TestReproducibility_EventReplay(t *testing.T) {
	// Create temp file for event store
	tmpFile, err := os.CreateTemp("", "events-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	store, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)

	eng := engine.NewWalletEngine(store, nil)

	// Set initial balances
	eng.SetBalance("alice", 1000)
	eng.SetBalance("bob", 500)
	eng.SetBalance("charlie", 200)

	// Execute several transfers
	transfers := []domain.TransferCommand{
		{TransactionID: "txn-1", FromAccount: "alice", ToAccount: "bob", Amount: 100},
		{TransactionID: "txn-2", FromAccount: "bob", ToAccount: "charlie", Amount: 50},
		{TransactionID: "txn-3", FromAccount: "charlie", ToAccount: "alice", Amount: 30},
		{TransactionID: "txn-4", FromAccount: "alice", ToAccount: "charlie", Amount: 200},
	}

	for _, cmd := range transfers {
		events, err := eng.Execute(cmd)
		require.NoError(t, err)

		err = store.AppendBatch(events)
		require.NoError(t, err)

		applyEventsToEngine(eng, events)
	}

	// Record original state
	originalBalances := eng.GetAllBalances()
	originalTotal := eng.GetTotalBalance()

	// Close the store
	store.Close()

	// Reopen the store and create a new engine
	store2, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)
	defer store2.Close()

	eng2 := engine.NewWalletEngine(store2, nil)

	// Set same initial balances (these would normally come from initial events)
	eng2.SetBalance("alice", 1000)
	eng2.SetBalance("bob", 500)
	eng2.SetBalance("charlie", 200)

	// Replay events from event store
	err = eng2.InitializeFromEventStore()
	require.NoError(t, err)

	// Verify state matches
	replayedBalances := eng2.GetAllBalances()
	replayedTotal := eng2.GetTotalBalance()

	assert.Equal(t, originalBalances, replayedBalances, "Balances should match after replay")
	assert.Equal(t, originalTotal, replayedTotal, "Total should match after replay")
}

// Test total balance conservation
func TestTotalBalanceConservation(t *testing.T) {
	// Create temp file for event store
	tmpFile, err := os.CreateTemp("", "events-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	store, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)
	defer store.Close()

	eng := engine.NewWalletEngine(store, nil)

	// Set initial balances
	eng.SetBalance("a", 1000)
	eng.SetBalance("b", 2000)
	eng.SetBalance("c", 3000)

	initialTotal := eng.GetTotalBalance()
	assert.Equal(t, int64(6000), initialTotal)

	// Execute many transfers
	accounts := []string{"a", "b", "c"}
	for i := 0; i < 100; i++ {
		from := accounts[i%3]
		to := accounts[(i+1)%3]

		cmd := domain.TransferCommand{
			TransactionID: generateTestTxnID(i),
			FromAccount:   from,
			ToAccount:     to,
			Amount:        int64(10 + i%50),
		}

		events, err := eng.Execute(cmd)
		require.NoError(t, err)

		err = store.AppendBatch(events)
		require.NoError(t, err)

		applyEventsToEngine(eng, events)
	}

	// Verify total is unchanged
	finalTotal := eng.GetTotalBalance()
	assert.Equal(t, initialTotal, finalTotal, "Total balance should be conserved")
}

// Test validation rules
func TestValidation(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "events-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	store, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)
	defer store.Close()

	eng := engine.NewWalletEngine(store, nil)
	eng.SetBalance("alice", 1000)

	tests := []struct {
		name          string
		cmd           domain.TransferCommand
		expectFailure bool
		failReason    string
	}{
		{
			name: "negative amount",
			cmd: domain.TransferCommand{
				TransactionID: "test-1",
				FromAccount:   "alice",
				ToAccount:     "bob",
				Amount:        -100,
			},
			expectFailure: true,
			failReason:    "amount must be positive",
		},
		{
			name: "zero amount",
			cmd: domain.TransferCommand{
				TransactionID: "test-2",
				FromAccount:   "alice",
				ToAccount:     "bob",
				Amount:        0,
			},
			expectFailure: true,
			failReason:    "amount must be positive",
		},
		{
			name: "same account",
			cmd: domain.TransferCommand{
				TransactionID: "test-3",
				FromAccount:   "alice",
				ToAccount:     "alice",
				Amount:        100,
			},
			expectFailure: true,
			failReason:    "cannot transfer to same account",
		},
		{
			name: "insufficient funds",
			cmd: domain.TransferCommand{
				TransactionID: "test-4",
				FromAccount:   "alice",
				ToAccount:     "bob",
				Amount:        10000,
			},
			expectFailure: true,
			failReason:    "insufficient funds",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			events, err := eng.Execute(tc.cmd)
			require.NoError(t, err)

			if tc.expectFailure {
				require.Len(t, events, 1)
				failEvent, ok := events[0].(domain.TransactionFailed)
				require.True(t, ok, "Expected TransactionFailed event")
				assert.Equal(t, tc.failReason, failEvent.Reason)
			}
		})
	}
}

// Helper functions

func generateTestTxnID(i int) string {
	return "test-txn-" + string(rune('0'+i/10)) + string(rune('0'+i%10))
}

func applyEventsToEngine(eng *engine.WalletEngine, events []domain.Event) {
	eng.ApplyEvents(events)
}
