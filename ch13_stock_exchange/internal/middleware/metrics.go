package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestDuration tracks request latency by method and path.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"method", "path", "status"},
	)

	// OrdersTotal counts orders by action.
	OrdersTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "exchange_orders_total",
			Help: "Total number of orders by action",
		},
		[]string{"action", "symbol"},
	)

	// MatchesTotal counts executed matches.
	MatchesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "exchange_matches_total",
			Help: "Total number of matches by symbol",
		},
		[]string{"symbol"},
	)

	// OrderBookDepth tracks order book depth.
	OrderBookDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "exchange_orderbook_depth",
			Help: "Current order book depth",
		},
		[]string{"symbol", "side"},
	)

	// SequencerInboundSeq tracks the current inbound sequence number.
	SequencerInboundSeq = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "exchange_sequencer_inbound_seq",
			Help: "Current inbound sequence number",
		},
	)

	// SequencerOutboundSeq tracks the current outbound sequence number.
	SequencerOutboundSeq = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "exchange_sequencer_outbound_seq",
			Help: "Current outbound sequence number",
		},
	)
)

// PrometheusMiddleware records request metrics.
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()

		HTTPRequestDuration.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			strconv.Itoa(c.Writer.Status()),
		).Observe(duration)
	}
}
