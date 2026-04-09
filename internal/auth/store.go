package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserExists   = errors.New("username already taken")
	ErrUserNotFound = errors.New("user not found")
)

// UserStat holds public stats for a single user.
type UserStat struct {
	Username       string
	Pages          int
	TotalSizeBytes int
	JoinedAt       time.Time
}

// UserStore handles user persistence.
type UserStore struct {
	DB *pgxpool.Pool
}

// Create inserts a new user. Returns ErrUserExists if the username is taken.
func (s *UserStore) Create(ctx context.Context, username, passwordHash string) error {
	_, err := s.DB.Exec(ctx,
		"INSERT INTO users (username, password_hash) VALUES ($1, $2)",
		username, passwordHash)
	if err != nil {
		if isDuplicateKey(err) {
			return ErrUserExists
		}
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

// GetPasswordHash returns the stored password hash for a user.
func (s *UserStore) GetPasswordHash(ctx context.Context, username string) (string, error) {
	var hash string
	err := s.DB.QueryRow(ctx,
		"SELECT password_hash FROM users WHERE username = $1",
		username).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrUserNotFound
		}
		return "", fmt.Errorf("getting password hash: %w", err)
	}
	return hash, nil
}

// TokenStore handles token persistence.
type TokenStore struct {
	DB *pgxpool.Pool
}

// Create inserts a new token hash and returns its ID.
func (s *TokenStore) Create(ctx context.Context, username, label, tokenHash string) error {
	_, err := s.DB.Exec(ctx,
		"INSERT INTO tokens (username, label, token_hash) VALUES ($1, $2, $3)",
		username, label, tokenHash)
	if err != nil {
		return fmt.Errorf("creating token: %w", err)
	}
	return nil
}

// GetHashesByUsername returns all token hashes for a user.
func (s *TokenStore) GetHashesByUsername(ctx context.Context, username string) ([]string, error) {
	rows, err := s.DB.Query(ctx,
		"SELECT token_hash FROM tokens WHERE username = $1",
		username)
	if err != nil {
		return nil, fmt.Errorf("querying tokens: %w", err)
	}
	defer rows.Close()

	var hashes []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, fmt.Errorf("scanning token: %w", err)
		}
		hashes = append(hashes, h)
	}
	return hashes, rows.Err()
}

func isDuplicateKey(err error) bool {
	return err != nil && contains(err.Error(), "duplicate key")
}

// ListUserStats returns public stats for all users, ordered by join date.
func (s *UserStore) ListUserStats(ctx context.Context) ([]UserStat, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT u.username, COUNT(p.id) AS pages,
		       COALESCE(SUM(p.size_bytes), 0) AS total_size_bytes,
		       u.created_at
		FROM users u
		LEFT JOIN pages p ON u.username = p.username
		GROUP BY u.username, u.created_at
		ORDER BY u.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	var stats []UserStat
	for rows.Next() {
		var s UserStat
		if err := rows.Scan(&s.Username, &s.Pages, &s.TotalSizeBytes, &s.JoinedAt); err != nil {
			return nil, fmt.Errorf("scanning user stats: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
