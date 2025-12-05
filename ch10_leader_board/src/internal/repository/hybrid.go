package repository

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// HybridRepository implements cache-aside pattern:
// - Read: Redis first, fallback to PostgreSQL on cache miss
// - Write: Write to both Redis and PostgreSQL (write-through)
type HybridRepository struct {
	redis    *RedisRepository
	postgres *PostgresRepository
}

func NewHybridRepository(redis *RedisRepository, postgres *PostgresRepository) *HybridRepository {
	return &HybridRepository{
		redis:    redis,
		postgres: postgres,
	}
}

// UpdateScore updates score in both Redis and PostgreSQL
// Write-through: ensures data consistency
func (h *HybridRepository) UpdateScore(userID string, points int, matchID string) (int, error) {
	ctx := context.Background()

	// 1. Write to PostgreSQL first (source of truth, handles idempotency)
	newScore, err := h.postgres.UpdateScore(userID, points, matchID)
	if err != nil {
		return 0, err
	}

	// 2. Update Redis cache (best effort, don't fail if Redis is down)
	if err := h.redis.SetScore(ctx, userID, newScore); err != nil {
		log.Printf("Warning: failed to update Redis cache for user %s: %v", userID, err)
		// Don't return error - PostgreSQL is the source of truth
	}

	return newScore, nil
}

// GetTopN retrieves top N players
// Cache-aside: Try Redis first, fallback to PostgreSQL
func (h *HybridRepository) GetTopN(n int) ([]LeaderboardEntry, error) {
	ctx := context.Background()

	// 1. Try Redis first
	entries, err := h.redis.GetTopN(ctx, n)
	if err == nil && len(entries) > 0 {
		return entries, nil
	}

	if err != nil {
		log.Printf("Redis GetTopN failed, falling back to PostgreSQL: %v", err)
	}

	// 2. Fallback to PostgreSQL
	entries, err = h.postgres.GetTopN(n)
	if err != nil {
		return nil, err
	}

	// 3. Warm cache asynchronously (best effort)
	go h.warmCacheFromEntries(entries)

	return entries, nil
}

// GetUserRank retrieves user rank and neighbors
// Cache-aside: Try Redis first, fallback to PostgreSQL
func (h *HybridRepository) GetUserRank(userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error) {
	ctx := context.Background()

	// 1. Try Redis first
	userEntry, neighbors, err := h.redis.GetUserRank(ctx, userID, neighborCount)
	if err == nil {
		return userEntry, neighbors, nil
	}

	log.Printf("Redis GetUserRank failed for user %s, falling back to PostgreSQL: %v", userID, err)

	// 2. Fallback to PostgreSQL
	userEntry, neighbors, err = h.postgres.GetUserRank(userID, neighborCount)
	if err != nil {
		return nil, nil, err
	}

	// 3. Warm cache for this user (best effort)
	go func() {
		if userEntry != nil {
			if err := h.redis.SetScore(context.Background(), userEntry.UserID, userEntry.Score); err != nil {
				log.Printf("Failed to warm cache for user %s: %v", userEntry.UserID, err)
			}
		}
	}()

	return userEntry, neighbors, nil
}

// warmCacheFromEntries populates Redis cache from PostgreSQL results
func (h *HybridRepository) warmCacheFromEntries(entries []LeaderboardEntry) {
	ctx := context.Background()
	for _, entry := range entries {
		if err := h.redis.SetScore(ctx, entry.UserID, entry.Score); err != nil {
			log.Printf("Failed to warm cache for user %s: %v", entry.UserID, err)
		}
	}
}

// WarmCache loads all leaderboard data from PostgreSQL into Redis
// Should be called at startup or periodically
func (h *HybridRepository) WarmCache(db *sql.DB) error {
	ctx := context.Background()
	currentMonth := time.Now().Format("2006-01")

	log.Println("Starting cache warming from PostgreSQL...")
	start := time.Now()

	rows, err := db.Query(`
		SELECT user_id, score
		FROM monthly_leaderboard
		WHERE month = $1
	`, currentMonth)
	if err != nil {
		return err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var userID string
		var score int
		if err := rows.Scan(&userID, &score); err != nil {
			log.Printf("Error scanning row during cache warm: %v", err)
			continue
		}

		if err := h.redis.SetScore(ctx, userID, score); err != nil {
			log.Printf("Error setting score in Redis during cache warm: %v", err)
			continue
		}
		count++

		if count%10000 == 0 {
			log.Printf("Cache warming progress: %d users loaded", count)
		}
	}

	log.Printf("Cache warming complete: %d users loaded in %v", count, time.Since(start))
	return rows.Err()
}
