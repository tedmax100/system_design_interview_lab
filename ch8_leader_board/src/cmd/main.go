package main

import (
	"context"
	"database/sql"
	"fmt"
	"leader_board/internal/config"
	"leader_board/internal/handler"
	"leader_board/internal/middleware"
	"leader_board/internal/repository"
	"leader_board/internal/tracing"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
)

func main() {
	// Initialize tracing
	cleanup, err := tracing.InitTracer("leaderboard-service")
	if err != nil {
		log.Printf("Warning: Failed to initialize tracing: %v", err)
	} else {
		defer cleanup()
	}

	// Load configuration
	cfg := config.Load()

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.DB.DSN)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(10) // Maximum number of open connections
	db.SetMaxIdleConns(5)  // Maximum number of idle connections
	db.SetConnMaxLifetime(5 * time.Minute)

	// Register DB stats collector for Prometheus metrics
	prometheus.MustRegister(collectors.NewDBStatsCollector(db, "postgres"))

	// Test database connection with retry
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		err = db.Ping()
		if err == nil {
			break
		}
		log.Printf("Waiting for database... (attempt %d/%d)", i+1, maxRetries)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Database not available after retries: %v", err)
	}
	log.Println("Successfully connected to PostgreSQL")

	// Initialize PostgreSQL repository (for v1 endpoints)
	postgresRepo := repository.NewPostgresRepository(db)

	// Initialize v1 handler (PostgreSQL only)
	h := handler.NewHandler(postgresRepo)

	// Setup router
	r := mux.NewRouter()

	// Add OpenTelemetry middleware for automatic HTTP tracing
	r.Use(otelmux.Middleware("leaderboard-service"))

	// ============================================
	// v1 API routes - PostgreSQL only (Scenario 1)
	// ============================================
	apiV1 := r.PathPrefix("/v1").Subrouter()
	apiV1.Use(middleware.MetricsMiddleware)

	apiV1.HandleFunc("/scores", h.UpdateScore).Methods("POST")
	apiV1.HandleFunc("/scores", h.GetLeaderboard).Methods("GET")
	apiV1.HandleFunc("/scores/{user_id}", h.GetUserRank).Methods("GET")

	// ============================================
	// v2 API routes - Redis + PostgreSQL (Scenario 2)
	// ============================================
	// Connect to Redis/Valkey
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection with retry
	ctx := context.Background()
	for i := 0; i < maxRetries; i++ {
		_, err = redisClient.Ping(ctx).Result()
		if err == nil {
			break
		}
		log.Printf("Waiting for Redis... (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(3 * time.Second)
	}

	var hV2 *handler.HandlerV2
	if err != nil {
		log.Printf("Warning: Redis not available, v2 endpoints will fallback to PostgreSQL only: %v", err)
		// Create hybrid repo that will always fallback to PostgreSQL
		redisRepo := repository.NewRedisRepository(redisClient)
		hybridRepo := repository.NewHybridRepository(redisRepo, postgresRepo)
		hV2 = handler.NewHandlerV2(hybridRepo)
	} else {
		log.Println("Successfully connected to Redis")

		// Initialize Redis repository
		redisRepo := repository.NewRedisRepository(redisClient)

		// Initialize Hybrid repository
		hybridRepo := repository.NewHybridRepository(redisRepo, postgresRepo)

		// Warm cache from PostgreSQL at startup
		go func() {
			if err := hybridRepo.WarmCache(db); err != nil {
				log.Printf("Warning: Cache warming failed: %v", err)
			}
		}()

		// Initialize v2 handler
		hV2 = handler.NewHandlerV2(hybridRepo)
	}

	apiV2 := r.PathPrefix("/v2").Subrouter()
	apiV2.Use(middleware.MetricsMiddleware)

	apiV2.HandleFunc("/scores", hV2.UpdateScore).Methods("POST")
	apiV2.HandleFunc("/scores", hV2.GetLeaderboard).Methods("GET")
	apiV2.HandleFunc("/scores/{user_id}", hV2.GetUserRank).Methods("GET")

	// Health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Metrics endpoint for Prometheus
	r.Handle("/metrics", promhttp.Handler())

	// Start server
	port := 8080
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting server on %s", addr)
	log.Println("  - v1 endpoints: PostgreSQL only (Scenario 1)")
	log.Println("  - v2 endpoints: Redis + PostgreSQL hybrid (Scenario 2)")
	log.Fatal(http.ListenAndServe(addr, r))
}
