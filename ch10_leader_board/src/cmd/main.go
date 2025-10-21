package main

import (
	"database/sql"
	"fmt"
	"leader_board/internal/config"
	"leader_board/internal/handler"
	"leader_board/internal/middleware"
	"leader_board/internal/repository"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.DB.DSN)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

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

	// Initialize repository (Version 1: PostgreSQL only)
	repo := repository.NewPostgresRepository(db)

	// Initialize handler
	h := handler.NewHandler(repo)

	// Setup router
	r := mux.NewRouter()

	// API routes
	api := r.PathPrefix("/v1").Subrouter()
	api.Use(middleware.MetricsMiddleware)

	// Score update endpoint
	api.HandleFunc("/scores", h.UpdateScore).Methods("POST")

	// Get top 10 leaderboard
	api.HandleFunc("/scores", h.GetLeaderboard).Methods("GET")

	// Get specific user rank
	api.HandleFunc("/scores/{user_id}", h.GetUserRank).Methods("GET")

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
	log.Printf("Starting server on %s (PostgreSQL-only mode)", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
