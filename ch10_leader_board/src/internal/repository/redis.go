package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRepository struct {
	client *redis.Client
}

func NewRedisRepository(client *redis.Client) *RedisRepository {
	return &RedisRepository{client: client}
}

// leaderboardKey returns the Redis key for the current month's leaderboard
func (r *RedisRepository) leaderboardKey() string {
	return fmt.Sprintf("leaderboard_%s", time.Now().Format("2006_01"))
}

// UpdateScore increments user's score using ZINCRBY
// Time complexity: O(log N)
func (r *RedisRepository) UpdateScore(ctx context.Context, userID string, points int) (int, error) {
	key := r.leaderboardKey()

	// ZINCRBY leaderboard_2024_01 1 "user123"
	newScore, err := r.client.ZIncrBy(ctx, key, float64(points), userID).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to update score in redis: %w", err)
	}

	return int(newScore), nil
}

// GetTopN retrieves top N players using ZREVRANGE
// Time complexity: O(log N + M) where M is the number of elements returned
func (r *RedisRepository) GetTopN(ctx context.Context, n int) ([]LeaderboardEntry, error) {
	key := r.leaderboardKey()

	// ZREVRANGE leaderboard_2024_01 0 9 WITHSCORES
	results, err := r.client.ZRevRangeWithScores(ctx, key, 0, int64(n-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get top N from redis: %w", err)
	}

	entries := make([]LeaderboardEntry, 0, len(results))
	for i, z := range results {
		entries = append(entries, LeaderboardEntry{
			UserID: z.Member.(string),
			Score:  int(z.Score),
			Rank:   i + 1,
		})
	}

	return entries, nil
}

// GetUserRank retrieves a user's rank using ZREVRANK and neighboring players
// Time complexity: O(log N) for rank, O(log N + M) for neighbors
func (r *RedisRepository) GetUserRank(ctx context.Context, userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error) {
	key := r.leaderboardKey()

	// Get user's rank: ZREVRANK leaderboard_2024_01 "user123"
	rank, err := r.client.ZRevRank(ctx, key, userID).Result()
	if err == redis.Nil {
		return nil, nil, fmt.Errorf("user not found in leaderboard")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user rank from redis: %w", err)
	}

	// Get user's score: ZSCORE leaderboard_2024_01 "user123"
	score, err := r.client.ZScore(ctx, key, userID).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user score from redis: %w", err)
	}

	userEntry := &LeaderboardEntry{
		UserID: userID,
		Score:  int(score),
		Rank:   int(rank) + 1, // Redis rank is 0-based
	}

	// Get neighbors if requested
	var neighbors []LeaderboardEntry
	if neighborCount > 0 {
		startRank := int64(rank) - int64(neighborCount)
		if startRank < 0 {
			startRank = 0
		}
		endRank := int64(rank) + int64(neighborCount)

		// ZREVRANGE leaderboard_2024_01 startRank endRank WITHSCORES
		results, err := r.client.ZRevRangeWithScores(ctx, key, startRank, endRank).Result()
		if err != nil {
			return userEntry, nil, fmt.Errorf("failed to get neighbors from redis: %w", err)
		}

		neighbors = make([]LeaderboardEntry, 0, len(results))
		for i, z := range results {
			neighbors = append(neighbors, LeaderboardEntry{
				UserID: z.Member.(string),
				Score:  int(z.Score),
				Rank:   int(startRank) + i + 1,
			})
		}
	}

	return userEntry, neighbors, nil
}

// Exists checks if a user exists in the leaderboard
func (r *RedisRepository) Exists(ctx context.Context, userID string) (bool, error) {
	key := r.leaderboardKey()
	_, err := r.client.ZScore(ctx, key, userID).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// SetScore sets a user's score directly (used for cache warming)
func (r *RedisRepository) SetScore(ctx context.Context, userID string, score int) error {
	key := r.leaderboardKey()
	return r.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(score),
		Member: userID,
	}).Err()
}

// GetLeaderboardSize returns the total number of users in the leaderboard
func (r *RedisRepository) GetLeaderboardSize(ctx context.Context) (int64, error) {
	key := r.leaderboardKey()
	return r.client.ZCard(ctx, key).Result()
}
