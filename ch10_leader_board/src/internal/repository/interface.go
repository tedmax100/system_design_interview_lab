package repository

import "context"

// Repository defines the interface for leaderboard operations
// This allows switching between PostgreSQL-only and Redis+PostgreSQL implementations
type Repository interface {
	// UpdateScore updates a user's score for the current month
	// Returns the new total score after the update
	UpdateScore(userID string, points int, matchID string) (int, error)

	// UpdateScoreWithContext updates a user's score with context for tracing
	UpdateScoreWithContext(ctx context.Context, userID string, points int, matchID string) (int, error)

	// GetTopN retrieves the top N players for the current month
	GetTopN(n int) ([]LeaderboardEntry, error)

	// GetTopNWithContext retrieves top N with context for tracing
	GetTopNWithContext(ctx context.Context, n int) ([]LeaderboardEntry, error)

	// GetUserRank retrieves a specific user's rank and nearby players
	GetUserRank(userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error)

	// GetUserRankWithContext retrieves user rank with context for tracing
	GetUserRankWithContext(ctx context.Context, userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error)
}
