package handler

import (
	"encoding/json"
	"leader_board/internal/repository"
	"net/http"

	"github.com/gorilla/mux"
)

type Handler struct {
	repo *repository.PostgresRepository
}

func NewHandler(repo *repository.PostgresRepository) *Handler {
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
	Status string                          `json:"status"`
	Data   LeaderboardData                 `json:"data"`
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
	UserID    string                          `json:"user_id"`
	Score     int                             `json:"score"`
	Rank      int                             `json:"rank"`
	Neighbors []repository.LeaderboardEntry   `json:"neighbors,omitempty"`
}

// UpdateScore handles POST /v1/scores
func (h *Handler) UpdateScore(w http.ResponseWriter, r *http.Request) {
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
		req.Points = 1 // Default to 1 point per win
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

// GetLeaderboard handles GET /v1/scores
func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
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

// GetUserRank handles GET /v1/scores/{user_id}
func (h *Handler) GetUserRank(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	// Get neighbors count from query parameter (default: 4)
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
