package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// EventType constants
const (
	EventTypeMoneyDeducted     = "MoneyDeducted"
	EventTypeMoneyCredited     = "MoneyCredited"
	EventTypeTransactionFailed = "TransactionFailed"
)

// Event is the base interface for all events
type Event interface {
	GetType() string
	GetTransactionID() string
}

// EventEnvelope wraps an event with metadata for serialization
type EventEnvelope struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// MoneyDeducted represents a successful deduction from an account
type MoneyDeducted struct {
	TransactionID string `json:"transaction_id"`
	Account       string `json:"account"`
	Amount        int64  `json:"amount"`
}

func (e MoneyDeducted) GetType() string          { return EventTypeMoneyDeducted }
func (e MoneyDeducted) GetTransactionID() string { return e.TransactionID }

// MoneyCredited represents a successful credit to an account
type MoneyCredited struct {
	TransactionID string `json:"transaction_id"`
	Account       string `json:"account"`
	Amount        int64  `json:"amount"`
}

func (e MoneyCredited) GetType() string          { return EventTypeMoneyCredited }
func (e MoneyCredited) GetTransactionID() string { return e.TransactionID }

// TransactionFailed represents a failed transaction (e.g., insufficient funds)
type TransactionFailed struct {
	TransactionID string `json:"transaction_id"`
	FromAccount   string `json:"from_account"`
	Reason        string `json:"reason"`
}

func (e TransactionFailed) GetType() string          { return EventTypeTransactionFailed }
func (e TransactionFailed) GetTransactionID() string { return e.TransactionID }

// SerializeEvent converts an event to JSON bytes with envelope
func SerializeEvent(event Event) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	envelope := EventEnvelope{
		Type:      event.GetType(),
		Timestamp: time.Now().UTC(),
		Data:      data,
	}

	return json.Marshal(envelope)
}

// DeserializeEvent converts JSON bytes back to an Event
func DeserializeEvent(data []byte) (Event, error) {
	var envelope EventEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}

	var event Event
	switch envelope.Type {
	case EventTypeMoneyDeducted:
		var e MoneyDeducted
		if err := json.Unmarshal(envelope.Data, &e); err != nil {
			return nil, err
		}
		event = e
	case EventTypeMoneyCredited:
		var e MoneyCredited
		if err := json.Unmarshal(envelope.Data, &e); err != nil {
			return nil, err
		}
		event = e
	case EventTypeTransactionFailed:
		var e TransactionFailed
		if err := json.Unmarshal(envelope.Data, &e); err != nil {
			return nil, err
		}
		event = e
	default:
		return nil, fmt.Errorf("unknown event type: %s", envelope.Type)
	}

	return event, nil
}
