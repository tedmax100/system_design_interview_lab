package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	redisTracer = otel.Tracer("valkey")
	pgTracer    = otel.Tracer("postgres")
)

// ValkeyRepository implements leaderboard operations using Valkey (Redis) Sorted Set
// with PostgreSQL as the persistent storage for history
type ValkeyRepository struct {
	rdb *redis.Client
	db  *sql.DB
}

func NewValkeyRepository(rdb *redis.Client, db *sql.DB) *ValkeyRepository {
	return &ValkeyRepository{
		rdb: rdb,
		db:  db,
	}
}

// getLeaderboardKey returns the Redis key for the current month's leaderboard
func (r *ValkeyRepository) getLeaderboardKey() string {
	return fmt.Sprintf("leaderboard_%s", time.Now().Format("2006_01"))
}

// UpdateScore updates a user's score using ZINCRBY - O(log n)
func (r *ValkeyRepository) UpdateScore(userID string, points int, matchID string) (int, error) {
	return r.UpdateScoreWithContext(context.Background(), userID, points, matchID)
}

// UpdateScoreWithContext updates score with context for tracing
func (r *ValkeyRepository) UpdateScoreWithContext(ctx context.Context, userID string, points int, matchID string) (int, error) {
	currentMonth := time.Now().Format("2006-01")

	// Start PostgreSQL transaction span
	ctx, txSpan := pgTracer.Start(ctx, "postgres.transaction",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "transaction"),
			attribute.String("user_id", userID),
		))
	defer txSpan.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		txSpan.RecordError(err)
		return 0, err
	}
	defer tx.Rollback()

	// Insert user
	_, insertSpan := pgTracer.Start(ctx, "postgres.insert_user",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "INSERT"),
			attribute.String("db.sql.table", "users"),
		))
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (user_id, username)
		VALUES ($1, $1)
		ON CONFLICT (user_id) DO NOTHING
	`, userID)
	if err != nil {
		insertSpan.RecordError(err)
		insertSpan.End()
		return 0, err
	}
	insertSpan.End()

	// Check idempotency
	_, checkSpan := pgTracer.Start(ctx, "postgres.check_idempotency",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.sql.table", "score_history"),
		))
	var exists bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM score_history WHERE match_id = $1)`, matchID).Scan(&exists)
	if err != nil {
		checkSpan.RecordError(err)
		checkSpan.End()
		return 0, err
	}
	checkSpan.SetAttributes(attribute.Bool("idempotency.exists", exists))
	checkSpan.End()

	if exists {
		// Already processed, get current score from Redis
		_, redisSpan := redisTracer.Start(ctx, "valkey.zscore",
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system", "redis"),
				attribute.String("db.operation", "ZSCORE"),
			))
		score, err := r.rdb.ZScore(ctx, r.getLeaderboardKey(), userID).Result()
		if err == redis.Nil {
			redisSpan.End()
			return 0, nil
		}
		if err != nil {
			redisSpan.RecordError(err)
			redisSpan.End()
			return 0, err
		}
		redisSpan.SetAttributes(attribute.Float64("score", score))
		redisSpan.End()
		return int(score), nil
	}

	// Insert score history
	_, historySpan := pgTracer.Start(ctx, "postgres.insert_score_history",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "INSERT"),
			attribute.String("db.sql.table", "score_history"),
		))
	_, err = tx.ExecContext(ctx, `
		INSERT INTO score_history (user_id, match_id, points)
		VALUES ($1, $2, $3)
	`, userID, matchID, points)
	if err != nil {
		historySpan.RecordError(err)
		historySpan.End()
		return 0, err
	}
	historySpan.End()

	// Update monthly leaderboard in PostgreSQL (for backup/history)
	_, updateSpan := pgTracer.Start(ctx, "postgres.upsert_leaderboard",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "UPSERT"),
			attribute.String("db.sql.table", "monthly_leaderboard"),
		))
	_, err = tx.ExecContext(ctx, `
		INSERT INTO monthly_leaderboard (user_id, score, month)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, month)
		DO UPDATE SET
			score = monthly_leaderboard.score + $2,
			updated_at = CURRENT_TIMESTAMP
	`, userID, points, currentMonth)
	if err != nil {
		updateSpan.RecordError(err)
		updateSpan.End()
		return 0, err
	}
	updateSpan.End()

	// Update Redis Sorted Set - ZINCRBY is O(log n)
	_, redisSpan := redisTracer.Start(ctx, "valkey.zincrby",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "ZINCRBY"),
			attribute.String("key", r.getLeaderboardKey()),
			attribute.Int("increment", points),
		))
	newScore, err := r.rdb.ZIncrBy(ctx, r.getLeaderboardKey(), float64(points), userID).Result()
	if err != nil {
		redisSpan.RecordError(err)
		redisSpan.End()
		return 0, err
	}
	redisSpan.SetAttributes(attribute.Float64("score.new", newScore))
	redisSpan.End()

	// Commit PostgreSQL transaction
	if err := tx.Commit(); err != nil {
		txSpan.RecordError(err)
		return 0, err
	}

	txSpan.SetAttributes(attribute.Int("score.result", int(newScore)))
	return int(newScore), nil
}

// GetTopN retrieves the top N players using ZREVRANGE - O(log n + m)
func (r *ValkeyRepository) GetTopN(n int) ([]LeaderboardEntry, error) {
	return r.GetTopNWithContext(context.Background(), n)
}

// GetTopNWithContext retrieves top N with context for tracing
func (r *ValkeyRepository) GetTopNWithContext(ctx context.Context, n int) ([]LeaderboardEntry, error) {
	ctx, span := redisTracer.Start(ctx, "valkey.zrevrange",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "ZREVRANGE"),
			attribute.String("key", r.getLeaderboardKey()),
			attribute.Int("limit", n),
		))
	defer span.End()

	// ZREVRANGE with WITHSCORES returns members sorted by score descending
	results, err := r.rdb.ZRevRangeWithScores(ctx, r.getLeaderboardKey(), 0, int64(n-1)).Result()
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	// Always a cache hit for this operation (data lives in Redis)
	span.AddEvent("cache.hit", trace.WithAttributes(
		attribute.String("cache.type", "valkey"),
		attribute.Int("result.count", len(results)),
	))

	entries := make([]LeaderboardEntry, 0, len(results))
	for i, z := range results {
		entries = append(entries, LeaderboardEntry{
			UserID: z.Member.(string),
			Score:  int(z.Score),
			Rank:   i + 1,
		})
	}

	span.SetAttributes(attribute.Int("result.count", len(entries)))
	return entries, nil
}

// GetUserRank retrieves a user's rank using ZREVRANK - O(log n)
func (r *ValkeyRepository) GetUserRank(userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error) {
	return r.GetUserRankWithContext(context.Background(), userID, neighborCount)
}

// GetUserRankWithContext retrieves user rank with context for tracing
func (r *ValkeyRepository) GetUserRankWithContext(ctx context.Context, userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error) {
	key := r.getLeaderboardKey()

	ctx, span := redisTracer.Start(ctx, "valkey.get_user_rank",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("key", key),
			attribute.String("user_id", userID),
		))
	defer span.End()

	// Get user's rank using ZREVRANK - O(log n)
	_, rankSpan := redisTracer.Start(ctx, "valkey.zrevrank",
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "ZREVRANK"),
		))
	rank, err := r.rdb.ZRevRank(ctx, key, userID).Result()
	if err == redis.Nil {
		rankSpan.AddEvent("cache.miss", trace.WithAttributes(
			attribute.String("cache.type", "valkey"),
			attribute.String("reason", "user_not_found"),
		))
		rankSpan.End()
		span.SetAttributes(attribute.Bool("cache.hit", false))
		return nil, nil, fmt.Errorf("user not found in leaderboard")
	}
	if err != nil {
		rankSpan.RecordError(err)
		rankSpan.End()
		return nil, nil, err
	}
	rankSpan.AddEvent("cache.hit", trace.WithAttributes(
		attribute.String("cache.type", "valkey"),
	))
	rankSpan.End()

	// Get user's score using ZSCORE - O(1)
	_, scoreSpan := redisTracer.Start(ctx, "valkey.zscore",
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.operation", "ZSCORE"),
		))
	score, err := r.rdb.ZScore(ctx, key, userID).Result()
	if err != nil {
		scoreSpan.RecordError(err)
		scoreSpan.End()
		return nil, nil, err
	}
	scoreSpan.AddEvent("cache.hit", trace.WithAttributes(
		attribute.String("cache.type", "valkey"),
	))
	scoreSpan.End()

	userEntry := &LeaderboardEntry{
		UserID: userID,
		Score:  int(score),
		Rank:   int(rank) + 1, // Convert 0-based to 1-based rank
	}

	span.AddEvent("cache.hit", trace.WithAttributes(
		attribute.String("cache.type", "valkey"),
		attribute.Int("user.rank", userEntry.Rank),
		attribute.Int("user.score", userEntry.Score),
	))
	span.SetAttributes(
		attribute.Bool("cache.hit", true),
		attribute.Int("user.rank", userEntry.Rank),
	)

	// Get neighboring players using ZREVRANGE - O(log n + m)
	var neighbors []LeaderboardEntry
	if neighborCount > 0 {
		_, neighborSpan := redisTracer.Start(ctx, "valkey.zrevrange_neighbors",
			trace.WithAttributes(
				attribute.String("db.system", "redis"),
				attribute.String("db.operation", "ZREVRANGE"),
				attribute.Int("neighbor_count", neighborCount),
			))

		startRank := int64(rank) - int64(neighborCount)
		if startRank < 0 {
			startRank = 0
		}
		endRank := int64(rank) + int64(neighborCount)

		results, err := r.rdb.ZRevRangeWithScores(ctx, key, startRank, endRank).Result()
		if err != nil {
			neighborSpan.RecordError(err)
			neighborSpan.End()
			return userEntry, nil, err
		}

		neighborSpan.AddEvent("cache.hit", trace.WithAttributes(
			attribute.String("cache.type", "valkey"),
			attribute.Int("neighbors.count", len(results)),
		))

		neighbors = make([]LeaderboardEntry, 0, len(results))
		for i, z := range results {
			neighbors = append(neighbors, LeaderboardEntry{
				UserID: z.Member.(string),
				Score:  int(z.Score),
				Rank:   int(startRank) + i + 1,
			})
		}
		neighborSpan.SetAttributes(attribute.Int("neighbors.count", len(neighbors)))
		neighborSpan.End()
	}

	return userEntry, neighbors, nil
}

// SyncFromPostgres rebuilds the Redis leaderboard from PostgreSQL data
func (r *ValkeyRepository) SyncFromPostgres(ctx context.Context) error {
	currentMonth := time.Now().Format("2006-01")
	key := r.getLeaderboardKey()

	// Clear existing data
	if err := r.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to clear existing data from Valkey: %w", err)
	}

	// Load all users from PostgreSQL
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id, score
		FROM monthly_leaderboard
		WHERE month = $1
	`, currentMonth)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Batch insert using pipeline for efficiency
	pipe := r.rdb.Pipeline()
	count := 0
	for rows.Next() {
		var userID string
		var score int
		if err := rows.Scan(&userID, &score); err != nil {
			return err
		}
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  float64(score),
			Member: userID,
		})
		count++

		// Execute in batches of 1000
		if count%1000 == 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				return err
			}
			pipe = r.rdb.Pipeline()
		}
	}

	// Execute remaining commands
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	return rows.Err()
}

// GetLeaderboardSize returns the total number of users in the leaderboard
func (r *ValkeyRepository) GetLeaderboardSize(ctx context.Context) (int64, error) {
	return r.rdb.ZCard(ctx, r.getLeaderboardKey()).Result()
}

// GetScoreRange returns users within a specific score range
func (r *ValkeyRepository) GetScoreRange(ctx context.Context, minScore, maxScore int, offset, count int64) ([]LeaderboardEntry, error) {
	results, err := r.rdb.ZRevRangeByScoreWithScores(ctx, r.getLeaderboardKey(), &redis.ZRangeBy{
		Min:    strconv.Itoa(minScore),
		Max:    strconv.Itoa(maxScore),
		Offset: offset,
		Count:  count,
	}).Result()
	if err != nil {
		return nil, err
	}

	entries := make([]LeaderboardEntry, 0, len(results))
	for _, z := range results {
		entries = append(entries, LeaderboardEntry{
			UserID: z.Member.(string),
			Score:  int(z.Score),
		})
	}

	return entries, nil
}
