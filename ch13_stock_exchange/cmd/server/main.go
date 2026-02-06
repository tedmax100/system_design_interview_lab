package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/nathanyu/stock-exchange/internal/handler"
	"github.com/nathanyu/stock-exchange/internal/marketdata"
	"github.com/nathanyu/stock-exchange/internal/matching"
	"github.com/nathanyu/stock-exchange/internal/middleware"
	"github.com/nathanyu/stock-exchange/internal/ordermanager"
	"github.com/nathanyu/stock-exchange/internal/sequencer"
)

const (
	channelBufferSize = 4096
	maxDailyVolume    = 1_000_000 // max shares per user per symbol per day
)

func main() {
	log.Println("Starting stock exchange service...")

	// --- Core components ---

	// Matching engine (stateless dispatcher over per-symbol order books)
	engine := matching.NewEngine()

	// Sequencer (stamps sequence IDs, feeds matching engine)
	seq := sequencer.NewSequencer(engine, channelBufferSize)

	// Order manager (risk check, wallet, order state)
	manager := ordermanager.NewManager(maxDailyVolume, channelBufferSize)

	// Market data publisher (candlesticks, execution log)
	publisher := marketdata.NewPublisher(channelBufferSize)

	// --- Wire channels (simulating ring buffers / mmap) ---
	//
	// API Handler → Order Manager → [OrderOut] → Sequencer [OrderIn]
	//                                              ↓
	// Market Data Publisher ← [ExecutionOut] ← Sequencer
	// Order Manager         ← [ExecutionOut] ← Sequencer
	//
	// The sequencer's OrderIn reads from the manager's OrderOut.
	// We use a fan-out goroutine to send execution events to both
	// the order manager and the market data publisher.

	// Start the fan-out from manager's OrderOut to sequencer's OrderIn
	go func() {
		for event := range manager.OrderOut {
			seq.OrderIn <- event
		}
	}()

	// Start the fan-out from sequencer's ExecutionOut to both consumers
	go func() {
		for event := range seq.ExecutionOut {
			// Fan out to order manager
			select {
			case manager.ExecutionIn <- event:
			default:
				log.Println("[main] WARN: order manager execution channel full")
			}
			// Fan out to market data publisher
			select {
			case publisher.ExecutionIn <- event:
			default:
				log.Println("[main] WARN: market data execution channel full")
			}
		}
	}()

	// Start component goroutines
	seq.Start()
	manager.Start()
	publisher.Start()

	// --- HTTP Server ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()
	r.Use(middleware.PrometheusMiddleware())

	h := handler.NewHandler(manager, engine, publisher)
	h.RegisterRoutes(r)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// --- Metrics Server ---
	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9090"
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsSrv := &http.Server{
		Addr:    ":" + metricsPort,
		Handler: metricsMux,
	}

	// Start servers
	go func() {
		log.Printf("Metrics server listening on :%s", metricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("metrics server error: %v", err)
		}
	}()

	go func() {
		log.Printf("HTTP server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	// --- Graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	seq.Stop()
	manager.Stop()
	publisher.Stop()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	if err := metricsSrv.Shutdown(ctx); err != nil {
		log.Printf("Metrics server shutdown error: %v", err)
	}

	log.Println("Stock exchange service stopped.")
}
