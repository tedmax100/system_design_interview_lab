package handler

import (
	"encoding/json"
	"leader_board/internal/repository"
	"net/http"

	"github.com/gorilla/mux"
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
	var req UpdateScoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.MatchID == "" {
		http.Error(w, "user_id and match_id are required", http.StatusBadRequest)
		return
	}

	if req.Points <= 0 {
		req.Points = 1
	}

	newScore, err := h.repo.UpdateScore(req.UserID, req.Points, req.MatchID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UpdateScoreResponse{
		Success:  true,
		NewScore: newScore,
	})
}

// GetLeaderboard handles GET /v2/scores
func (h *HandlerV2) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	entries, err := h.repo.GetTopN(10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
	vars := mux.Vars(r)
	userID := vars["user_id"]

	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	neighborCount := 4

	userEntry, neighbors, err := h.repo.GetUserRank(userID, neighborCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

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
