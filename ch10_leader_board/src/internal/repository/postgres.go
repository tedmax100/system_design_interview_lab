package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var dbTracer = otel.Tracer("postgres")

type LeaderboardEntry struct {
	UserID string `json:"user_id"`
	Score  int    `json:"score"`
	Rank   int    `json:"rank"`
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// UpdateScore updates a user's score for the current month
func (r *PostgresRepository) UpdateScore(userID string, points int, matchID string) (int, error) {
	return r.UpdateScoreWithContext(context.Background(), userID, points, matchID)
}

// UpdateScoreWithContext updates a user's score with context for tracing
func (r *PostgresRepository) UpdateScoreWithContext(ctx context.Context, userID string, points int, matchID string) (int, error) {
	currentMonth := time.Now().Format("2006-01")

	// Start DB transaction span
	ctx, span := dbTracer.Start(ctx, "postgres.transaction",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "transaction"),
			attribute.String("user_id", userID),
		))
	defer span.End()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		return 0, err
	}
	defer tx.Rollback()

	// Insert user
	_, insertSpan := dbTracer.Start(ctx, "postgres.insert_user",
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
	_, checkSpan := dbTracer.Start(ctx, "postgres.check_idempotency",
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
		var currentScore int
		err = tx.QueryRowContext(ctx, `
			SELECT COALESCE(score, 0)
			FROM monthly_leaderboard
			WHERE user_id = $1 AND month = $2
		`, userID, currentMonth).Scan(&currentScore)
		if err != nil && err != sql.ErrNoRows {
			return 0, err
		}
		return currentScore, nil
	}

	// Insert score history
	_, historySpan := dbTracer.Start(ctx, "postgres.insert_score_history",
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

	// Update leaderboard
	_, updateSpan := dbTracer.Start(ctx, "postgres.upsert_leaderboard",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "UPSERT"),
			attribute.String("db.sql.table", "monthly_leaderboard"),
		))
	var newScore int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO monthly_leaderboard (user_id, score, month)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, month)
		DO UPDATE SET
			score = monthly_leaderboard.score + $2,
			updated_at = CURRENT_TIMESTAMP
		RETURNING score
	`, userID, points, currentMonth).Scan(&newScore)
	if err != nil {
		updateSpan.RecordError(err)
		updateSpan.End()
		return 0, err
	}
	updateSpan.SetAttributes(attribute.Int("score.new", newScore))
	updateSpan.End()

	// Commit
	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		return 0, err
	}

	span.SetAttributes(attribute.Int("score.result", newScore))
	return newScore, nil
}

// GetTopN retrieves the top N players for the current month
func (r *PostgresRepository) GetTopN(n int) ([]LeaderboardEntry, error) {
	return r.GetTopNWithContext(context.Background(), n)
}

// GetTopNWithContext retrieves top N with context for tracing
func (r *PostgresRepository) GetTopNWithContext(ctx context.Context, n int) ([]LeaderboardEntry, error) {
	currentMonth := time.Now().Format("2006-01")

	ctx, span := dbTracer.Start(ctx, "postgres.get_top_n",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.sql.table", "monthly_leaderboard"),
			attribute.Int("limit", n),
		))
	defer span.End()

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			user_id,
			score,
			RANK() OVER (ORDER BY score DESC) as rank
		FROM monthly_leaderboard
		WHERE month = $1
		ORDER BY score DESC
		LIMIT $2
	`, currentMonth, n)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.UserID, &entry.Score, &entry.Rank); err != nil {
			span.RecordError(err)
			return nil, err
		}
		entries = append(entries, entry)
	}

	span.SetAttributes(attribute.Int("result.count", len(entries)))
	return entries, rows.Err()
}

// GetUserRank retrieves a specific user's rank and nearby players
func (r *PostgresRepository) GetUserRank(userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error) {
	return r.GetUserRankWithContext(context.Background(), userID, neighborCount)
}

// GetUserRankWithContext retrieves user rank with context for tracing
func (r *PostgresRepository) GetUserRankWithContext(ctx context.Context, userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error) {
	currentMonth := time.Now().Format("2006-01")

	ctx, span := dbTracer.Start(ctx, "postgres.get_user_rank",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.sql.table", "monthly_leaderboard"),
			attribute.String("user_id", userID),
		))
	defer span.End()

	var userEntry LeaderboardEntry
	err := r.db.QueryRowContext(ctx, `
		SELECT
			lb1.user_id,
			lb1.score,
			(SELECT COUNT(*) FROM monthly_leaderboard lb2
			 WHERE lb2.month = $2 AND lb2.score >= lb1.score) AS rank
		FROM monthly_leaderboard lb1
		WHERE lb1.user_id = $1 AND lb1.month = $2
	`, userID, currentMonth).Scan(&userEntry.UserID, &userEntry.Score, &userEntry.Rank)

	if err == sql.ErrNoRows {
		span.SetAttributes(attribute.Bool("user.found", false))
		return nil, nil, fmt.Errorf("user not found in leaderboard")
	}
	if err != nil {
		span.RecordError(err)
		return nil, nil, err
	}

	span.SetAttributes(
		attribute.Bool("user.found", true),
		attribute.Int("user.rank", userEntry.Rank),
		attribute.Int("user.score", userEntry.Score),
	)

	// Get neighbors
	neighbors := []LeaderboardEntry{}
	if neighborCount > 0 {
		_, neighborSpan := dbTracer.Start(ctx, "postgres.get_neighbors",
			trace.WithAttributes(
				attribute.String("db.system", "postgresql"),
				attribute.String("db.operation", "SELECT"),
				attribute.Int("neighbor_count", neighborCount),
			))

		startRank := userEntry.Rank - neighborCount
		if startRank < 1 {
			startRank = 1
		}
		endRank := userEntry.Rank + neighborCount

		rows, err := r.db.QueryContext(ctx, `
			WITH ranked AS (
				SELECT
					user_id,
					score,
					RANK() OVER (ORDER BY score DESC) as rank
				FROM monthly_leaderboard
				WHERE month = $1
			)
			SELECT user_id, score, rank
			FROM ranked
			WHERE rank BETWEEN $2 AND $3
			ORDER BY rank
		`, currentMonth, startRank, endRank)
		if err != nil {
			neighborSpan.RecordError(err)
			neighborSpan.End()
			return &userEntry, nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var entry LeaderboardEntry
			if err := rows.Scan(&entry.UserID, &entry.Score, &entry.Rank); err != nil {
				neighborSpan.RecordError(err)
				neighborSpan.End()
				return &userEntry, neighbors, err
			}
			neighbors = append(neighbors, entry)
		}
		neighborSpan.SetAttributes(attribute.Int("neighbors.count", len(neighbors)))
		neighborSpan.End()
	}

	return &userEntry, neighbors, nil
}
