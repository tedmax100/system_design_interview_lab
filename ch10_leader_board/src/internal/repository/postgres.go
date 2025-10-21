package repository

import (
	"database/sql"
	"fmt"
	"time"
)

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
	currentMonth := time.Now().Format("2006-01")

	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Ensure user exists
	_, err = tx.Exec(`
		INSERT INTO users (user_id, username)
		VALUES ($1, $1)
		ON CONFLICT (user_id) DO NOTHING
	`, userID)
	if err != nil {
		return 0, err
	}

	// Check if match_id already exists (idempotency)
	var exists bool
	err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM score_history WHERE match_id = $1)`, matchID).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if exists {
		// Already processed this match, return current score
		var currentScore int
		err = tx.QueryRow(`
			SELECT COALESCE(score, 0)
			FROM monthly_leaderboard
			WHERE user_id = $1 AND month = $2
		`, userID, currentMonth).Scan(&currentScore)
		if err != nil && err != sql.ErrNoRows {
			return 0, err
		}
		return currentScore, nil
	}

	// Record score history
	_, err = tx.Exec(`
		INSERT INTO score_history (user_id, match_id, points)
		VALUES ($1, $2, $3)
	`, userID, matchID, points)
	if err != nil {
		return 0, err
	}

	// Update monthly leaderboard
	var newScore int
	err = tx.QueryRow(`
		INSERT INTO monthly_leaderboard (user_id, score, month)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, month)
		DO UPDATE SET
			score = monthly_leaderboard.score + $2,
			updated_at = CURRENT_TIMESTAMP
		RETURNING score
	`, userID, points, currentMonth).Scan(&newScore)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return newScore, nil
}

// GetTopN retrieves the top N players for the current month
func (r *PostgresRepository) GetTopN(n int) ([]LeaderboardEntry, error) {
	currentMonth := time.Now().Format("2006-01")

	// This is the problematic query that requires full table scan and sort
	rows, err := r.db.Query(`
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
		return nil, err
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.UserID, &entry.Score, &entry.Rank); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// GetUserRank retrieves a specific user's rank and nearby players
func (r *PostgresRepository) GetUserRank(userID string, neighborCount int) (*LeaderboardEntry, []LeaderboardEntry, error) {
	currentMonth := time.Now().Format("2006-01")

	// This is extremely slow query - requires counting all rows with score >= user's score
	var userEntry LeaderboardEntry
	err := r.db.QueryRow(`
		SELECT
			lb1.user_id,
			lb1.score,
			(SELECT COUNT(*) FROM monthly_leaderboard lb2
			 WHERE lb2.month = $2 AND lb2.score >= lb1.score) AS rank
		FROM monthly_leaderboard lb1
		WHERE lb1.user_id = $1 AND lb1.month = $2
	`, userID, currentMonth).Scan(&userEntry.UserID, &userEntry.Score, &userEntry.Rank)

	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("user not found in leaderboard")
	}
	if err != nil {
		return nil, nil, err
	}

	// Get neighboring players - also slow due to window function
	neighbors := []LeaderboardEntry{}
	if neighborCount > 0 {
		startRank := userEntry.Rank - neighborCount
		if startRank < 1 {
			startRank = 1
		}
		endRank := userEntry.Rank + neighborCount

		rows, err := r.db.Query(`
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
			return &userEntry, nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var entry LeaderboardEntry
			if err := rows.Scan(&entry.UserID, &entry.Score, &entry.Rank); err != nil {
				return &userEntry, neighbors, err
			}
			neighbors = append(neighbors, entry)
		}
	}

	return &userEntry, neighbors, nil
}
