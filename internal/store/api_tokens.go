package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/puppe1990/vitalis/internal/models"
)

// tokenPrefix makes a leaked token recognizable in a screenshot or a log,
// which is what lets a user tell an API token from a session cookie.
const tokenPrefix = "vit_"

// CreateAPIToken mints a token and returns the plaintext exactly once. Only
// the SHA-256 hash is persisted, mirroring the password-reset policy: a DB
// dump or backup must not be replayable against the API.
func (s *SQLiteStore) CreateAPIToken(userID int64, label string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	plain := tokenPrefix + hex.EncodeToString(raw)

	_, err := s.db.Exec(
		"INSERT INTO api_tokens (user_id, token_hash, label) VALUES (?, ?, ?)",
		userID, hashAPIToken(plain), label,
	)
	if err != nil {
		return "", fmt.Errorf("create api token for user %d: %w", userID, err)
	}
	return plain, nil
}

// FindUserIDByAPIToken resolves a bearer token to its owner.
func (s *SQLiteStore) FindUserIDByAPIToken(plain string) (int64, bool) {
	// An empty token would hash to the empty-string digest and could match a
	// row written by a bug; reject it before touching the DB.
	if strings.TrimSpace(plain) == "" {
		return 0, false
	}
	var userID int64
	err := s.db.QueryRow(
		"SELECT user_id FROM api_tokens WHERE token_hash = ?", hashAPIToken(plain),
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		return 0, false
	}
	return userID, true
}

func (s *SQLiteStore) ListAPITokens(userID int64) ([]models.APIToken, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, label, created_at FROM api_tokens
		WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api tokens for user %d: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.APIToken
	for rows.Next() {
		var t models.APIToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Label, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api token: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) RevokeAPIToken(userID, id int64) error {
	result, err := s.db.Exec("DELETE FROM api_tokens WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("revoke api token %d: %w", id, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke api token rows %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("revoke api token %d: %w", id, ErrNotFound)
	}
	return nil
}

// TokenFingerprint is the short, non-secret handle shown in the UI so a user
// can tell two tokens apart without the secret ever being stored.
func TokenFingerprint(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])[:8]
}

func hashAPIToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// EnsureDemoAPIToken returns a stable token for the demo user in development
// so the MCP server can be pointed at a local instance without a manual step.
// Production never calls this: seeding is gated on ENV=development.
func (s *SQLiteStore) EnsureDemoAPIToken() (string, error) {
	const demoToken = "vit_demo_dev_token_do_not_use_in_production"

	var userID int64
	err := s.db.QueryRow("SELECT id FROM users WHERE email = 'demo@example.com'").Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find demo user: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO api_tokens (user_id, token_hash, label) VALUES (?, ?, 'Demo local')
		 ON CONFLICT(token_hash) DO NOTHING`,
		userID, hashAPIToken(demoToken),
	)
	if err != nil {
		return "", fmt.Errorf("ensure demo api token: %w", err)
	}
	return demoToken, nil
}
