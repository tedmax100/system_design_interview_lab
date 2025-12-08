package repository

import (
	"context"
	"database/sql"
	"leader_board/internal/tracing"
	"log"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
func (h *HybridRepository) UpdateScore(ctx context.Context, userID string, points int, matchID string) (int, error) {
	ctx, span := tracing.Tracer.Start(ctx, "hybrid.UpdateScore",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("strategy", "write-through"),
		),
	)
	defer span.End()

	// Add user info as event
	span.AddEvent("processing_request", trace.WithAttributes(
		attribute.String("user_id", userID),
		attribute.String("match_id", matchID),
		attribute.Int("points", points),
	))

	// 1. Write to PostgreSQL first (source of truth, handles idempotency)
	newScore, err := h.postgres.UpdateScore(ctx, userID, points, matchID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "postgres write failed")
		return 0, err
	}

	// 2. Update Redis cache (best effort, don't fail if Redis is down)
	if err := h.redis.SetScore(ctx, userID, newScore); err != nil {
		span.AddEvent("redis_cache_update_failed", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		log.Printf("Warning: failed to update Redis cache for user %s: %v", userID, err)
		// Don't return error - PostgreSQL is the source of truth
	} else {
		span.AddEvent("redis_cache_updated", trace.WithAttributes(
			attribute.Int("new_score", newScore),
		))
	}

	span.SetAttributes(attribute.Int("new_score", newScore))
	span.SetStatus(codes.Ok, "")
	return newScore, nil
}

// GetTopN retrieves top N players
// Cache-aside: Try Redis first, fallback to PostgreSQL
func (h *HybridRepository) GetTopN(ctx context.Context, n int) ([]LeaderboardEntry, error) {
	ctx, span := tracing.Tracer.Start(ctx, "hybrid.GetTopN",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("strategy", "cache-aside"),
			attribute.Int("limit", n),
		),
	)
	defer span.End()

	// 1. Try Redis first
	entries, err := h.redis.GetTopN(ctx, n)
	if err == nil && len(entries) > 0 {
		span.SetAttributes(
			attribute.Bool("cache.hit", true),
			attribute.String("data_source", "redis"),
			attribute.Int("result_count", len(entries)),
		)
		span.AddEvent("cache_hit", trace.WithAttributes(
			attribute.String("source", "redis"),
			attribute.Int("entries_returned", len(entries)),
		))
		span.SetStatus(codes.Ok, "")
		return entries, nil
	}

	if err != nil {
		span.AddEvent("redis_fallback", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		log.Printf("Redis GetTopN failed, falling back to PostgreSQL: %v", err)
	} else {
		span.AddEvent("redis_fallback", trace.WithAttributes(
			attribute.String("reason", "empty_result"),
		))
	}

	span.SetAttributes(attribute.Bool("cache.hit", false))

	// 2. Fallback to PostgreSQL
	entries, err = h.postgres.GetTopN(ctx, n)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "postgres fallback failed")
		return nil, err
	}

	span.SetAttributes(
		attribute.String("data_source", "postgresql"),
		attribute.Int("result_count", len(entries)),
	)
	span.AddEvent("cache_miss", trace.WithAttributes(
		attribute.String("fallback_source", "postgresql"),
		attribute.Int("entries_returned", len(entries)),
	))

	// 3. Warm cache asynchronously (best effort)
	go h.warmCacheFromEntries(entries)

	span.SetStatus(codes.Ok, "")
	return entries, nil
}

// GetUserRank retrieves user rank and neighbors
// Cache-aside: Try Redis first, fallback to PostgreSQL
func (h *HybridRepository) GetUserRank(ctx context.Context, userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error) {
	ctx, span := tracing.Tracer.Start(ctx, "hybrid.GetUserRank",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("strategy", "cache-aside"),
		),
	)
	defer span.End()

	// Add user_id as event
	span.AddEvent("query_user", trace.WithAttributes(
		attribute.String("user_id", userID),
		attribute.Int("neighbor_count", neighborCount),
	))

	// 1. Try Redis first
	userEntry, neighbors, err := h.redis.GetUserRank(ctx, userID, neighborCount)
	if err == nil {
		span.SetAttributes(
			attribute.Bool("cache.hit", true),
			attribute.String("data_source", "redis"),
			attribute.Int("user_rank", userEntry.Rank),
			attribute.Int("neighbor_count", len(neighbors)),
		)
		span.AddEvent("cache_hit", trace.WithAttributes(
			attribute.String("source", "redis"),
			attribute.Int("user_rank", userEntry.Rank),
		))
		span.SetStatus(codes.Ok, "")
		return userEntry, neighbors, nil
	}

	span.AddEvent("redis_fallback", trace.WithAttributes(
		attribute.String("error", err.Error()),
	))
	span.SetAttributes(attribute.Bool("cache.hit", false))
	log.Printf("Redis GetUserRank failed for user %s, falling back to PostgreSQL: %v", userID, err)

	// 2. Fallback to PostgreSQL
	userEntry, neighbors, err = h.postgres.GetUserRank(ctx, userID, neighborCount)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "postgres fallback failed")
		return nil, nil, err
	}

	span.SetAttributes(
		attribute.String("data_source", "postgresql"),
		attribute.Int("user_rank", userEntry.Rank),
		attribute.Int("neighbor_count", len(neighbors)),
	)
	span.AddEvent("cache_miss", trace.WithAttributes(
		attribute.String("fallback_source", "postgresql"),
		attribute.Int("user_rank", userEntry.Rank),
	))

	// 3. Warm cache for this user (best effort)
	go func() {
		if userEntry != nil {
			if err := h.redis.SetScore(context.Background(), userEntry.UserID, userEntry.Score); err != nil {
				log.Printf("Failed to warm cache for user %s: %v", userEntry.UserID, err)
			}
		}
	}()

	span.SetStatus(codes.Ok, "")
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
	ctx, span := tracing.Tracer.Start(context.Background(), "hybrid.WarmCache",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	currentMonth := time.Now().Format("2006-01")

	log.Println("Starting cache warming from PostgreSQL...")
	start := time.Now()

	rows, err := db.QueryContext(ctx, `
		SELECT user_id, score
		FROM monthly_leaderboard
		WHERE month = $1
	`, currentMonth)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to query PostgreSQL")
		return err
	}
	defer rows.Close()

	count := 0
	errors := 0
	for rows.Next() {
		var userID string
		var score int
		if err := rows.Scan(&userID, &score); err != nil {
			log.Printf("Error scanning row during cache warm: %v", err)
			errors++
			continue
		}

		if err := h.redis.SetScore(ctx, userID, score); err != nil {
			log.Printf("Error setting score in Redis during cache warm: %v", err)
			errors++
			continue
		}
		count++

		if count%10000 == 0 {
			log.Printf("Cache warming progress: %d users loaded", count)
		}
	}

	duration := time.Since(start)
	span.SetAttributes(
		attribute.Int("users_loaded", count),
		attribute.Int("errors", errors),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)
	span.SetStatus(codes.Ok, "")

	log.Printf("Cache warming complete: %d users loaded in %v", count, duration)
	return rows.Err()
}
