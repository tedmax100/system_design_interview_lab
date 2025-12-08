package repository

import (
	"context"
	"database/sql"
	"fmt"
	"leader_board/internal/tracing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
func (r *PostgresRepository) UpdateScore(ctx context.Context, userID string, points int, matchID string) (int, error) {
	ctx, span := tracing.Tracer.Start(ctx, "postgres.UpdateScore",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "UPDATE"),
		),
	)
	defer span.End()

	// Add user info as event (not in span name)
	span.AddEvent("processing_request", trace.WithAttributes(
		attribute.String("user_id", userID),
		attribute.String("match_id", matchID),
		attribute.Int("points", points),
	))

	currentMonth := time.Now().Format("2006-01")

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to begin transaction")
		return 0, err
	}
	defer tx.Rollback()

	// Ensure user exists
	_, userSpan := tracing.Tracer.Start(ctx, "postgres.InsertUser",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "INSERT"),
			attribute.String("db.table", "users"),
		),
	)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (user_id, username)
		VALUES ($1, $1)
		ON CONFLICT (user_id) DO NOTHING
	`, userID)
	if err != nil {
		userSpan.RecordError(err)
		userSpan.SetStatus(codes.Error, err.Error())
		userSpan.End()
		span.RecordError(err)
		return 0, err
	}
	userSpan.SetStatus(codes.Ok, "")
	userSpan.End()

	// Check if match_id already exists (idempotency)
	_, checkSpan := tracing.Tracer.Start(ctx, "postgres.CheckIdempotency",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.table", "score_history"),
		),
	)
	var exists bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM score_history WHERE match_id = $1)`, matchID).Scan(&exists)
	if err != nil {
		checkSpan.RecordError(err)
		checkSpan.SetStatus(codes.Error, err.Error())
		checkSpan.End()
		span.RecordError(err)
		return 0, err
	}
	checkSpan.SetAttributes(attribute.Bool("match_exists", exists))
	checkSpan.SetStatus(codes.Ok, "")
	checkSpan.End()

	if exists {
		// Already processed this match, return current score
		span.AddEvent("idempotency_check", trace.WithAttributes(
			attribute.Bool("duplicate_match", true),
		))

		var currentScore int
		err = tx.QueryRowContext(ctx, `
			SELECT COALESCE(score, 0)
			FROM monthly_leaderboard
			WHERE user_id = $1 AND month = $2
		`, userID, currentMonth).Scan(&currentScore)
		if err != nil && err != sql.ErrNoRows {
			span.RecordError(err)
			return 0, err
		}
		span.SetAttributes(attribute.Int("current_score", currentScore))
		return currentScore, nil
	}

	// Record score history
	_, historySpan := tracing.Tracer.Start(ctx, "postgres.InsertScoreHistory",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "INSERT"),
			attribute.String("db.table", "score_history"),
		),
	)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO score_history (user_id, match_id, points)
		VALUES ($1, $2, $3)
	`, userID, matchID, points)
	if err != nil {
		historySpan.RecordError(err)
		historySpan.SetStatus(codes.Error, err.Error())
		historySpan.End()
		span.RecordError(err)
		return 0, err
	}
	historySpan.SetStatus(codes.Ok, "")
	historySpan.End()

	// Update monthly leaderboard
	_, updateSpan := tracing.Tracer.Start(ctx, "postgres.UpsertLeaderboard",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "UPSERT"),
			attribute.String("db.table", "monthly_leaderboard"),
		),
	)
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
		updateSpan.SetStatus(codes.Error, err.Error())
		updateSpan.End()
		span.RecordError(err)
		return 0, err
	}
	updateSpan.SetAttributes(attribute.Int("new_score", newScore))
	updateSpan.SetStatus(codes.Ok, "")
	updateSpan.End()

	// Commit
	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to commit transaction")
		return 0, err
	}

	span.SetAttributes(attribute.Int("new_score", newScore))
	span.SetStatus(codes.Ok, "")
	return newScore, nil
}

// GetTopN retrieves the top N players for the current month
func (r *PostgresRepository) GetTopN(ctx context.Context, n int) ([]LeaderboardEntry, error) {
	ctx, span := tracing.Tracer.Start(ctx, "postgres.GetTopN",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.table", "monthly_leaderboard"),
			attribute.Int("limit", n),
		),
	)
	defer span.End()

	currentMonth := time.Now().Format("2006-01")

	// This is the problematic query that requires full table scan and sort
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
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.UserID, &entry.Score, &entry.Rank); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		entries = append(entries, entry)
	}

	span.SetAttributes(attribute.Int("result_count", len(entries)))
	span.SetStatus(codes.Ok, "")
	return entries, rows.Err()
}

// GetUserRank retrieves a specific user's rank and nearby players
func (r *PostgresRepository) GetUserRank(ctx context.Context, userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error) {
	ctx, span := tracing.Tracer.Start(ctx, "postgres.GetUserRank",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
		),
	)
	defer span.End()

	// Add user_id as event
	span.AddEvent("query_user", trace.WithAttributes(
		attribute.String("user_id", userID),
		attribute.Int("neighbor_count", neighborCount),
	))

	currentMonth := time.Now().Format("2006-01")

	// This is extremely slow query - requires counting all rows with score >= user's score
	_, rankSpan := tracing.Tracer.Start(ctx, "postgres.SelectUserRank",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.table", "monthly_leaderboard"),
			attribute.String("query.type", "user_rank_with_count"),
		),
	)
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
		rankSpan.SetStatus(codes.Error, "user not found")
		rankSpan.End()
		span.SetStatus(codes.Error, "user not found in leaderboard")
		return nil, nil, fmt.Errorf("user not found in leaderboard")
	}
	if err != nil {
		rankSpan.RecordError(err)
		rankSpan.SetStatus(codes.Error, err.Error())
		rankSpan.End()
		span.RecordError(err)
		return nil, nil, err
	}
	rankSpan.SetAttributes(
		attribute.Int("user_rank", userEntry.Rank),
		attribute.Int("user_score", userEntry.Score),
	)
	rankSpan.SetStatus(codes.Ok, "")
	rankSpan.End()

	span.SetAttributes(
		attribute.Bool("user.found", true),
		attribute.Int("user.rank", userEntry.Rank),
		attribute.Int("user.score", userEntry.Score),
	)

	// Get neighbors
	neighbors := []LeaderboardEntry{}
	if neighborCount > 0 {
		_, neighborSpan := tracing.Tracer.Start(ctx, "postgres.SelectNeighbors",
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system", "postgresql"),
				attribute.String("db.operation", "SELECT"),
				attribute.String("db.table", "monthly_leaderboard"),
				attribute.String("query.type", "neighbors_with_window"),
			),
		)

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
			neighborSpan.SetStatus(codes.Error, err.Error())
			neighborSpan.End()
			span.RecordError(err)
			return &userEntry, nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var entry LeaderboardEntry
			if err := rows.Scan(&entry.UserID, &entry.Score, &entry.Rank); err != nil {
				neighborSpan.RecordError(err)
				neighborSpan.SetStatus(codes.Error, err.Error())
				neighborSpan.End()
				return &userEntry, neighbors, err
			}
			neighbors = append(neighbors, entry)
		}
		neighborSpan.SetAttributes(attribute.Int("neighbor_count", len(neighbors)))
		neighborSpan.SetStatus(codes.Ok, "")
		neighborSpan.End()
	}

	span.SetAttributes(
		attribute.Int("user_rank", userEntry.Rank),
		attribute.Int("neighbor_count", len(neighbors)),
	)
	span.SetStatus(codes.Ok, "")
	return &userEntry, neighbors, nil
}
