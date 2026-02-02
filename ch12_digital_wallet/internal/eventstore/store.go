package eventstore

import (
	"bufio"
	"fmt"
	"os"
	"sync"

	"github.com/nathanyu/digital-wallet/internal/domain"
)

// EventStore provides append-only storage for events
type EventStore struct {
	filePath string
	file     *os.File
	mu       sync.Mutex
}

// NewEventStore creates a new event store with the given file path
func NewEventStore(filePath string) (*EventStore, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open event store file: %w", err)
	}

	return &EventStore{
		filePath: filePath,
		file:     file,
	}, nil
}

// Append writes an event to the event store
func (s *EventStore) Append(event domain.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := domain.SerializeEvent(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Append newline for line-delimited JSON
	data = append(data, '\n')

	_, err = s.file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	// Ensure durability
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync event store: %w", err)
	}

	return nil
}

// AppendBatch writes multiple events to the event store atomically
func (s *EventStore) AppendBatch(events []domain.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, event := range events {
		data, err := domain.SerializeEvent(event)
		if err != nil {
			return fmt.Errorf("failed to serialize event: %w", err)
		}

		data = append(data, '\n')

		_, err = s.file.Write(data)
		if err != nil {
			return fmt.Errorf("failed to write event: %w", err)
		}
	}

	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync event store: %w", err)
	}

	return nil
}

// LoadAll reads all events from the event store
func (s *EventStore) LoadAll() ([]domain.Event, error) {
	file, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.Event{}, nil
		}
		return nil, fmt.Errorf("failed to open event store for reading: %w", err)
	}
	defer file.Close()

	var events []domain.Event
	scanner := bufio.NewScanner(file)
	// Increase buffer size for potentially large events
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		event, err := domain.DeserializeEvent(line)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize event at line %d: %w", lineNum, err)
		}

		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading event store: %w", err)
	}

	return events, nil
}

// Close closes the event store file
func (s *EventStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

// Clear removes all events from the store (for testing purposes)
func (s *EventStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file != nil {
		s.file.Close()
	}

	// Truncate the file
	file, err := os.OpenFile(s.filePath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to clear event store: %w", err)
	}

	s.file = file
	return nil
}
