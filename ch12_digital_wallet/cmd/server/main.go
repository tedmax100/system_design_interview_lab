package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nathanyu/digital-wallet/internal/cqrs"
	"github.com/nathanyu/digital-wallet/internal/engine"
	"github.com/nathanyu/digital-wallet/internal/eventstore"
	"github.com/nathanyu/digital-wallet/internal/handler"
	"github.com/nathanyu/digital-wallet/internal/middleware"
	"github.com/nathanyu/digital-wallet/internal/queue"
	"github.com/nathanyu/digital-wallet/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const serviceName = "digital-wallet"

// Config holds application configuration
type Config struct {
	Port           int
	MetricsPort    int
	NATSUrl        string
	EventStorePath string
	GinMode        string
}

func main() {
	// Parse command line flags
	cfg := parseFlags()

	// Initialize structured logging
	telemetry.InitLogger(serviceName)

	// Initialize OpenTelemetry tracing
	cleanup, err := telemetry.InitTracer(serviceName)
	if err != nil {
		log.Printf("Warning: Failed to initialize tracer: %v", err)
	} else {
		defer cleanup()
	}

	// Set Gin mode
	gin.SetMode(cfg.GinMode)

	// Initialize components
	log.Println("Starting Digital Wallet service...")

	// 1. Connect to NATS
	log.Printf("Connecting to NATS at %s...", cfg.NATSUrl)
	natsClient, err := queue.NewNATSClient(cfg.NATSUrl)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer natsClient.Close()
	log.Println("Connected to NATS")

	// 2. Initialize Event Store
	log.Printf("Initializing event store at %s...", cfg.EventStorePath)
	eventStore, err := eventstore.NewEventStore(cfg.EventStorePath)
	if err != nil {
		log.Fatalf("Failed to initialize event store: %v", err)
	}
	defer eventStore.Close()
	log.Println("Event store initialized")

	// 3. Initialize Wallet Engine (State Machine)
	walletEngine := engine.NewWalletEngine(eventStore, natsClient.GetConn())

	// 4. Initialize CQRS Read Model
	readModel := cqrs.NewReadModel(natsClient.GetConn())

	// 5. Register read model as event handler for direct updates
	walletEngine.RegisterEventHandler(readModel.HandleEventDirect)

	// 6. Replay events to rebuild state
	log.Println("Replaying events to rebuild state...")
	if err := walletEngine.InitializeFromEventStore(); err != nil {
		log.Fatalf("Failed to initialize wallet engine: %v", err)
	}
	if err := readModel.InitializeFromEventStore(eventStore); err != nil {
		log.Fatalf("Failed to initialize read model: %v", err)
	}

	// 7. Start the wallet engine
	if err := walletEngine.Start(); err != nil {
		log.Fatalf("Failed to start wallet engine: %v", err)
	}
	defer walletEngine.Stop()

	// 8. Start the read model (subscribe to events via NATS)
	if err := readModel.Start(engine.EventSubject); err != nil {
		log.Fatalf("Failed to start read model: %v", err)
	}
	defer readModel.Stop()

	// 9. Initialize HTTP handler
	h := handler.NewHandler(natsClient, readModel, walletEngine)

	// 10. Setup Gin router with middleware
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Tracing())
	router.Use(middleware.Metrics())
	handler.SetupRoutes(router, h)

	// 11. Start HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 12. Start metrics server (separate port for Prometheus scraping)
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
		Handler: metricsMux,
	}

	// Start servers in goroutines
	go func() {
		log.Printf("HTTP server listening on port %d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	go func() {
		log.Printf("Metrics server listening on port %d", cfg.MetricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Metrics server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server forced to shutdown: %v", err)
	}
	if err := metricsSrv.Shutdown(ctx); err != nil {
		log.Printf("Metrics server forced to shutdown: %v", err)
	}

	log.Println("Service stopped")
}

func parseFlags() *Config {
	cfg := &Config{}

	flag.IntVar(&cfg.Port, "port", getEnvInt("PORT", 8080), "HTTP server port")
	flag.IntVar(&cfg.MetricsPort, "metrics-port", getEnvInt("METRICS_PORT", 9090), "Metrics server port")
	flag.StringVar(&cfg.NATSUrl, "nats-url", getEnv("NATS_URL", "nats://localhost:4222"), "NATS server URL")
	flag.StringVar(&cfg.EventStorePath, "event-store", getEnv("EVENT_STORE_PATH", "data/events.log"), "Event store file path")
	flag.StringVar(&cfg.GinMode, "gin-mode", getEnv("GIN_MODE", "release"), "Gin mode (debug/release)")

	flag.Parse()

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var v int
		if _, err := fmt.Sscanf(value, "%d", &v); err == nil {
			return v
		}
	}
	return defaultValue
}
