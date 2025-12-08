package repository

import "context"

// Repository defines the interface for leaderboard operations
// This allows switching between PostgreSQL-only and Redis+PostgreSQL implementations
type Repository interface {
	// UpdateScore updates a user's score for the current month
	// Returns the new total score after the update
	UpdateScore(ctx context.Context, userID string, points int, matchID string) (int, error)

	// GetTopN retrieves the top N players for the current month
	GetTopN(ctx context.Context, n int) ([]LeaderboardEntry, error)

	// GetUserRank retrieves a specific user's rank and nearby players
	GetUserRank(ctx context.Context, userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error)
}
