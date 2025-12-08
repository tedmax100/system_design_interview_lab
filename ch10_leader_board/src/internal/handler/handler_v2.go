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

// HandlerV2 uses HybridRepository (Redis + PostgreSQL fallback)
type HandlerV2 struct {
	repo *repository.HybridRepository
}

func NewHandlerV2(repo *repository.HybridRepository) *HandlerV2 {
	return &HandlerV2{repo: repo}
}

// UpdateScore handles POST /v2/scores
func (h *HandlerV2) UpdateScore(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracing.Tracer.Start(r.Context(), "handler.v2.UpdateScore",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("api_version", "v2"),
		),
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

	// Add user info as attributes (not in span name)
	span.SetAttributes(
		attribute.String("user_id", req.UserID),
		attribute.String("match_id", req.MatchID),
		attribute.Int("points", req.Points),
	)

	if req.Points <= 0 {
		req.Points = 1
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

// GetLeaderboard handles GET /v2/scores
func (h *HandlerV2) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracing.Tracer.Start(r.Context(), "handler.v2.GetLeaderboard",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("api_version", "v2"),
			attribute.Int("limit", 10),
		),
	)
	defer span.End()

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

// GetUserRank handles GET /v2/scores/{user_id}
func (h *HandlerV2) GetUserRank(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracing.Tracer.Start(r.Context(), "handler.v2.GetUserRank",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("api_version", "v2"),
		),
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
