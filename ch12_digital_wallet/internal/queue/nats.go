package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nathanyu/digital-wallet/internal/domain"
	"github.com/nathanyu/digital-wallet/internal/engine"
)

// NATSClient wraps NATS connection for command publishing
type NATSClient struct {
	conn *nats.Conn
}

// NewNATSClient creates a new NATS client
func NewNATSClient(url string) (*NATSClient, error) {
	opts := []nats.Option{
		nats.Name("digital-wallet"),
		nats.ReconnectWait(time.Second),
		nats.MaxReconnects(10),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				fmt.Printf("NATS disconnected: %v\n", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			fmt.Printf("NATS reconnected to %s\n", nc.ConnectedUrl())
		}),
	}

	conn, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &NATSClient{conn: conn}, nil
}

// GetConn returns the underlying NATS connection
func (c *NATSClient) GetConn() *nats.Conn {
	return c.conn
}

// PublishCommand publishes a transfer command and waits for response
func (c *NATSClient) PublishCommand(cmd domain.TransferCommand, timeout time.Duration) (*engine.CommandResponse, error) {
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	msg, err := c.conn.Request(engine.CommandSubject, data, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to publish command: %w", err)
	}

	var resp engine.CommandResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

// PublishCommandAsync publishes a transfer command without waiting for response
func (c *NATSClient) PublishCommandAsync(cmd domain.TransferCommand) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	if err := c.conn.Publish(engine.CommandSubject, data); err != nil {
		return fmt.Errorf("failed to publish command: %w", err)
	}

	return nil
}

// Close closes the NATS connection
func (c *NATSClient) Close() {
	if c.conn != nil {
		c.conn.Drain()
		c.conn.Close()
	}
}
