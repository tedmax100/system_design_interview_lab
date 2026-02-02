package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wallet_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "wallet_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// Transfer metrics
	TransfersTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wallet_transfers_total",
			Help: "Total number of transfer attempts",
		},
		[]string{"status"}, // success, failed, insufficient_funds
	)

	TransferAmount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "wallet_transfer_amount",
			Help:    "Transfer amount distribution (in cents)",
			Buckets: []float64{10, 50, 100, 500, 1000, 5000, 10000, 50000, 100000},
		},
		[]string{"status"},
	)

	TransferProcessingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "wallet_transfer_processing_duration_seconds",
			Help:    "Time to process a transfer command",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
	)

	// Event store metrics
	EventsStoredTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wallet_events_stored_total",
			Help: "Total number of events stored",
		},
		[]string{"type"}, // MoneyDeducted, MoneyCredited, TransactionFailed
	)

	EventStoreWriteDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "wallet_event_store_write_duration_seconds",
			Help:    "Time to write events to event store",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
		},
	)

	// NATS metrics
	NATSMessagesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wallet_nats_messages_published_total",
			Help: "Total number of NATS messages published",
		},
		[]string{"subject"},
	)

	NATSMessagesReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wallet_nats_messages_received_total",
			Help: "Total number of NATS messages received",
		},
		[]string{"subject"},
	)

	// Account metrics
	AccountBalanceGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "wallet_account_balance",
			Help: "Current account balance (in cents)",
		},
		[]string{"account"},
	)

	TotalBalanceGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "wallet_total_balance",
			Help: "Total balance across all accounts (in cents)",
		},
	)

	AccountCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "wallet_account_count",
			Help: "Total number of accounts",
		},
	)

	// Idempotency metrics
	DuplicateTransactionsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "wallet_duplicate_transactions_total",
			Help: "Total number of duplicate transactions detected",
		},
	)
)
