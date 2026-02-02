package test

import (
	"os"
	"testing"

	"github.com/nathanyu/digital-wallet/internal/domain"
	"github.com/nathanyu/digital-wallet/internal/eventstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventStore_AppendAndLoad(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "events-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create store
	store, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)

	// Append events
	events := []domain.Event{
		domain.MoneyDeducted{
			TransactionID: "txn-1",
			Account:       "alice",
			Amount:        100,
		},
		domain.MoneyCredited{
			TransactionID: "txn-1",
			Account:       "bob",
			Amount:        100,
		},
		domain.TransactionFailed{
			TransactionID: "txn-2",
			FromAccount:   "charlie",
			Reason:        "insufficient funds",
		},
	}

	for _, event := range events {
		err := store.Append(event)
		require.NoError(t, err)
	}

	// Close and reopen
	store.Close()

	store2, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)
	defer store2.Close()

	// Load events
	loaded, err := store2.LoadAll()
	require.NoError(t, err)
	assert.Len(t, loaded, 3)

	// Verify event types and data
	deducted, ok := loaded[0].(domain.MoneyDeducted)
	require.True(t, ok)
	assert.Equal(t, "txn-1", deducted.TransactionID)
	assert.Equal(t, "alice", deducted.Account)
	assert.Equal(t, int64(100), deducted.Amount)

	credited, ok := loaded[1].(domain.MoneyCredited)
	require.True(t, ok)
	assert.Equal(t, "txn-1", credited.TransactionID)
	assert.Equal(t, "bob", credited.Account)
	assert.Equal(t, int64(100), credited.Amount)

	failed, ok := loaded[2].(domain.TransactionFailed)
	require.True(t, ok)
	assert.Equal(t, "txn-2", failed.TransactionID)
	assert.Equal(t, "charlie", failed.FromAccount)
	assert.Equal(t, "insufficient funds", failed.Reason)
}

func TestEventStore_AppendBatch(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "events-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)
	defer store.Close()

	// Append batch of events
	events := []domain.Event{
		domain.MoneyDeducted{TransactionID: "txn-1", Account: "a", Amount: 50},
		domain.MoneyCredited{TransactionID: "txn-1", Account: "b", Amount: 50},
	}

	err = store.AppendBatch(events)
	require.NoError(t, err)

	// Load and verify
	loaded, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, loaded, 2)
}

func TestEventStore_EmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "events-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)
	defer store.Close()

	// Load from empty file
	loaded, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, loaded, 0)
}

func TestEventStore_Clear(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "events-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := eventstore.NewEventStore(tmpFile.Name())
	require.NoError(t, err)
	defer store.Close()

	// Add some events
	err = store.Append(domain.MoneyDeducted{TransactionID: "txn-1", Account: "a", Amount: 100})
	require.NoError(t, err)

	// Clear
	err = store.Clear()
	require.NoError(t, err)

	// Verify empty
	loaded, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, loaded, 0)
}
