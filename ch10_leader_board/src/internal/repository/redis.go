package repository

import (
	"context"
	"fmt"
	"leader_board/internal/tracing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	ctx, span := tracing.Tracer.Start(ctx, "redis.UpdateScore",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "ZINCRBY"),
		),
	)
	defer span.End()

	// Add user info as event
	span.AddEvent("update_request", trace.WithAttributes(
		attribute.String("user_id", userID),
		attribute.Int("points", points),
	))

	key := r.leaderboardKey()

	// ZINCRBY leaderboard_2024_01 1 "user123"
	newScore, err := r.client.ZIncrBy(ctx, key, float64(points), userID).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update score in redis")
		return 0, fmt.Errorf("failed to update score in redis: %w", err)
	}

	span.SetAttributes(attribute.Int("new_score", int(newScore)))
	span.SetStatus(codes.Ok, "")
	return int(newScore), nil
}

// GetTopN retrieves top N players using ZREVRANGE
// Time complexity: O(log N + M) where M is the number of elements returned
func (r *RedisRepository) GetTopN(ctx context.Context, n int) ([]LeaderboardEntry, error) {
	ctx, span := tracing.Tracer.Start(ctx, "redis.GetTopN",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "ZREVRANGE"),
			attribute.Int("limit", n),
		),
	)
	defer span.End()

	key := r.leaderboardKey()

	// ZREVRANGE leaderboard_2024_01 0 9 WITHSCORES
	results, err := r.client.ZRevRangeWithScores(ctx, key, 0, int64(n-1)).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get top N from redis")
		span.SetAttributes(attribute.Bool("cache.hit", false))
		return nil, fmt.Errorf("failed to get top N from redis: %w", err)
	}

	// Determine cache hit/miss
	cacheHit := len(results) > 0
	span.SetAttributes(
		attribute.Bool("cache.hit", cacheHit),
		attribute.Int("result_count", len(results)),
	)

	if !cacheHit {
		span.AddEvent("cache_miss", trace.WithAttributes(
			attribute.String("reason", "empty_result"),
		))
	} else {
		span.AddEvent("cache_hit", trace.WithAttributes(
			attribute.Int("entries_returned", len(results)),
		))
	}

	entries := make([]LeaderboardEntry, 0, len(results))
	for i, z := range results {
		entries = append(entries, LeaderboardEntry{
			UserID: z.Member.(string),
			Score:  int(z.Score),
			Rank:   i + 1,
		})
	}

	span.SetStatus(codes.Ok, "")
	return entries, nil
}

// GetUserRank retrieves a user's rank using ZREVRANK and neighboring players
// Time complexity: O(log N) for rank, O(log N + M) for neighbors
func (r *RedisRepository) GetUserRank(ctx context.Context, userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error) {
	ctx, span := tracing.Tracer.Start(ctx, "redis.GetUserRank",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
		),
	)
	defer span.End()

	// Add user_id as event
	span.AddEvent("query_user", trace.WithAttributes(
		attribute.String("user_id", userID),
		attribute.Int("neighbor_count", neighborCount),
	))

	key := r.leaderboardKey()

	// Get user's rank: ZREVRANK leaderboard_2024_01 "user123"
	_, rankSpan := tracing.Tracer.Start(ctx, "redis.ZREVRANK",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "ZREVRANK"),
		),
	)
	rank, err := r.client.ZRevRank(ctx, key, userID).Result()
	if err == redis.Nil {
		rankSpan.SetAttributes(attribute.Bool("cache.hit", false))
		rankSpan.AddEvent("cache_miss", trace.WithAttributes(
			attribute.String("reason", "user_not_found"),
			attribute.String("user_id", userID),
		))
		rankSpan.SetStatus(codes.Error, "user not found")
		rankSpan.End()
		span.SetAttributes(attribute.Bool("cache.hit", false))
		span.SetStatus(codes.Error, "user not found in leaderboard")
		return nil, nil, fmt.Errorf("user not found in leaderboard")
	}
	if err != nil {
		rankSpan.RecordError(err)
		rankSpan.SetAttributes(attribute.Bool("cache.hit", false))
		rankSpan.SetStatus(codes.Error, err.Error())
		rankSpan.End()
		span.RecordError(err)
		return nil, nil, fmt.Errorf("failed to get user rank from redis: %w", err)
	}
	rankSpan.SetAttributes(
		attribute.Bool("cache.hit", true),
		attribute.Int64("rank", rank),
	)
	rankSpan.AddEvent("cache_hit", trace.WithAttributes(
		attribute.Int64("rank", rank),
	))
	rankSpan.SetStatus(codes.Ok, "")
	rankSpan.End()

	// Get user's score: ZSCORE leaderboard_2024_01 "user123"
	_, scoreSpan := tracing.Tracer.Start(ctx, "redis.ZSCORE",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "ZSCORE"),
		),
	)
	score, err := r.client.ZScore(ctx, key, userID).Result()
	if err != nil {
		scoreSpan.RecordError(err)
		scoreSpan.SetAttributes(attribute.Bool("cache.hit", false))
		scoreSpan.SetStatus(codes.Error, err.Error())
		scoreSpan.End()
		span.RecordError(err)
		return nil, nil, fmt.Errorf("failed to get user score from redis: %w", err)
	}
	scoreSpan.SetAttributes(
		attribute.Bool("cache.hit", true),
		attribute.Float64("score", score),
	)
	scoreSpan.AddEvent("cache_hit", trace.WithAttributes(
		attribute.Float64("score", score),
	))
	scoreSpan.SetStatus(codes.Ok, "")
	scoreSpan.End()

	userEntry := &LeaderboardEntry{
		UserID: userID,
		Score:  int(score),
		Rank:   int(rank) + 1, // Redis rank is 0-based
	}

	// Get neighbors if requested
	var neighbors []LeaderboardEntry
	if neighborCount > 0 {
		_, neighborSpan := tracing.Tracer.Start(ctx, "redis.ZREVRANGE_neighbors",
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system", "redis"),
				attribute.String("db.operation", "ZREVRANGE"),
			),
		)

		startRank := int64(rank) - int64(neighborCount)
		if startRank < 0 {
			startRank = 0
		}
		endRank := int64(rank) + int64(neighborCount)

		// ZREVRANGE leaderboard_2024_01 startRank endRank WITHSCORES
		results, err := r.client.ZRevRangeWithScores(ctx, key, startRank, endRank).Result()
		if err != nil {
			neighborSpan.RecordError(err)
			neighborSpan.SetAttributes(attribute.Bool("cache.hit", false))
			neighborSpan.SetStatus(codes.Error, err.Error())
			neighborSpan.End()
			span.RecordError(err)
			return userEntry, nil, fmt.Errorf("failed to get neighbors from redis: %w", err)
		}

		neighborSpan.SetAttributes(
			attribute.Bool("cache.hit", true),
			attribute.Int("neighbor_count", len(results)),
		)
		neighborSpan.AddEvent("cache_hit", trace.WithAttributes(
			attribute.Int("neighbors_returned", len(results)),
		))
		neighborSpan.SetStatus(codes.Ok, "")
		neighborSpan.End()

		neighbors = make([]LeaderboardEntry, 0, len(results))
		for i, z := range results {
			neighbors = append(neighbors, LeaderboardEntry{
				UserID: z.Member.(string),
				Score:  int(z.Score),
				Rank:   int(startRank) + i + 1,
			})
		}
	}

	span.SetAttributes(
		attribute.Bool("cache.hit", true),
		attribute.Int("user_rank", userEntry.Rank),
		attribute.Int("neighbor_count", len(neighbors)),
	)
	span.SetStatus(codes.Ok, "")
	return userEntry, neighbors, nil
}

// Exists checks if a user exists in the leaderboard
func (r *RedisRepository) Exists(ctx context.Context, userID string) (bool, error) {
	ctx, span := tracing.Tracer.Start(ctx, "redis.Exists",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "ZSCORE"),
		),
	)
	defer span.End()

	span.AddEvent("check_user", trace.WithAttributes(
		attribute.String("user_id", userID),
	))

	key := r.leaderboardKey()
	_, err := r.client.ZScore(ctx, key, userID).Result()
	if err == redis.Nil {
		span.SetAttributes(attribute.Bool("exists", false))
		span.SetStatus(codes.Ok, "")
		return false, nil
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	span.SetAttributes(attribute.Bool("exists", true))
	span.SetStatus(codes.Ok, "")
	return true, nil
}

// SetScore sets a user's score directly (used for cache warming)
func (r *RedisRepository) SetScore(ctx context.Context, userID string, score int) error {
	ctx, span := tracing.Tracer.Start(ctx, "redis.SetScore",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "ZADD"),
		),
	)
	defer span.End()

	span.AddEvent("set_score", trace.WithAttributes(
		attribute.String("user_id", userID),
		attribute.Int("score", score),
	))

	key := r.leaderboardKey()
	err := r.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(score),
		Member: userID,
	}).Err()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// GetLeaderboardSize returns the total number of users in the leaderboard
func (r *RedisRepository) GetLeaderboardSize(ctx context.Context) (int64, error) {
	ctx, span := tracing.Tracer.Start(ctx, "redis.GetLeaderboardSize",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "ZCARD"),
		),
	)
	defer span.End()

	key := r.leaderboardKey()
	size, err := r.client.ZCard(ctx, key).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, err
	}

	span.SetAttributes(attribute.Int64("size", size))
	span.SetStatus(codes.Ok, "")
	return size, nil
}
