package handler

import (
	"encoding/json"
	"leader_board/internal/repository"
	"leader_board/internal/tracing"
	"net/http"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	repo repository.Repository
}

func NewHandler(repo repository.Repository) *Handler {
	return &Handler{repo: repo}
}

// UpdateScoreRequest represents the request body for updating scores
type UpdateScoreRequest struct {
	UserID  string `json:"user_id"`
	Points  int    `json:"points"`
	MatchID string `json:"match_id"`
}

// UpdateScoreResponse represents the response for score update
type UpdateScoreResponse struct {
	Success  bool `json:"success"`
	NewScore int  `json:"new_score"`
}

// LeaderboardResponse represents the response for top N leaderboard
type LeaderboardResponse struct {
	Status string          `json:"status"`
	Data   LeaderboardData `json:"data"`
}

type LeaderboardData struct {
	Leaderboard []repository.LeaderboardEntry `json:"leaderboard"`
	Count       int                           `json:"count"`
}

// UserRankResponse represents the response for user rank query
type UserRankResponse struct {
	Status string       `json:"status"`
	Data   UserRankData `json:"data"`
}

type UserRankData struct {
	UserID    string                        `json:"user_id"`
	Score     int                           `json:"score"`
	Rank      int                           `json:"rank"`
	Neighbors []repository.LeaderboardEntry `json:"neighbors,omitempty"`
}

// UpdateScore handles POST /v1/scores or /v2/scores
func (h *Handler) UpdateScore(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracing.Tracer.Start(r.Context(), "handler.UpdateScore",
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	var req UpdateScoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.MatchID == "" {
		span.SetStatus(codes.Error, "Missing required fields")
		http.Error(w, "user_id and match_id are required", http.StatusBadRequest)
		return
	}

	// Add user info as attributes (not in span name to avoid high cardinality)
	span.SetAttributes(
		attribute.String("user_id", req.UserID),
		attribute.String("match_id", req.MatchID),
		attribute.Int("points", req.Points),
	)

	if req.Points <= 0 {
		req.Points = 1 // Default to 1 point per win
	}

	newScore, err := h.repo.UpdateScore(ctx, req.UserID, req.Points, req.MatchID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.Int("new_score", newScore))
	span.SetStatus(codes.Ok, "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UpdateScoreResponse{
		Success:  true,
		NewScore: newScore,
	})
}

// GetLeaderboard handles GET /v1/scores or /v2/scores
func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracing.Tracer.Start(r.Context(), "handler.GetLeaderboard",
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	span.SetAttributes(attribute.Int("limit", 10))

	entries, err := h.repo.GetTopN(ctx, 10)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.Int("result_count", len(entries)))
	span.SetStatus(codes.Ok, "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LeaderboardResponse{
		Status: "success",
		Data: LeaderboardData{
			Leaderboard: entries,
			Count:       len(entries),
		},
	})
}

// GetUserRank handles GET /v1/scores/{user_id} or /v2/scores/{user_id}
func (h *Handler) GetUserRank(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracing.Tracer.Start(r.Context(), "handler.GetUserRank",
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	vars := mux.Vars(r)
	userID := vars["user_id"]

	if userID == "" {
		span.SetStatus(codes.Error, "user_id is required")
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	// Add user_id as attribute (not in span name)
	span.SetAttributes(attribute.String("user_id", userID))

	// Get neighbors count from query parameter (default: 4)
	neighborCount := 4

	userEntry, neighbors, err := h.repo.GetUserRank(ctx, userID, neighborCount)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	span.SetAttributes(
		attribute.Int("user_rank", userEntry.Rank),
		attribute.Int("user_score", userEntry.Score),
		attribute.Int("neighbor_count", len(neighbors)),
	)
	span.SetStatus(codes.Ok, "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserRankResponse{
		Status: "success",
		Data: UserRankData{
			UserID:    userEntry.UserID,
			Score:     userEntry.Score,
			Rank:      userEntry.Rank,
			Neighbors: neighbors,
		},
	})
}
