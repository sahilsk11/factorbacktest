package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuthSession is the storage shape for one row in app_auth.user_session.
// The auth package (internal/auth) is the only intended caller of this
// repository; if you find yourself reaching for it from a handler,
// reach for `auth.CurrentUser(c)` instead.
type AuthSession struct {
	ID            string
	UserAccountID uuid.UUID
	CreatedAt     time.Time
	ExpiresAt     time.Time
	LastSeenAt    time.Time
	IP            string
	UserAgent     string
}

// AuthSessionRepository persists session rows for the custom Go auth flow.
//
// Read paths return ErrSessionNotFound (NOT a generic "no rows" error) when
// the session id doesn't match anything. Callers can treat that as "fall
// through to unauthenticated" cleanly.
type AuthSessionRepository interface {
	Create(ctx context.Context, s AuthSession) error
	// Get returns the session if it exists AND has not yet expired (per
	// expires_at, the sliding clock). Absolute lifetime cap (created_at +
	// max age) is enforced in the auth package, not here, so this layer
	// stays purely about persistence.
	Get(ctx context.Context, id string) (*AuthSession, error)
	// Touch bumps expires_at and last_seen_at for a session. Used by the
	// auth middleware on every authenticated request to slide the
	// expiration window.
	Touch(ctx context.Context, id string, newExpiresAt time.Time) error
	Delete(ctx context.Context, id string) error
	// DeleteExpired removes rows where expires_at <= before. Returns the
	// number of rows deleted. Intended for a daily cleanup goroutine.
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

// ErrSessionNotFound is returned by Get when no row matches the id.
// Callers should treat it as "no session" rather than wrap it.
var ErrSessionNotFound = errors.New("auth session not found")

type authSessionRepositoryHandler struct {
	DB *sql.DB
}

func NewAuthSessionRepository(db *sql.DB) AuthSessionRepository {
	return authSessionRepositoryHandler{DB: db}
}

func (h authSessionRepositoryHandler) Create(ctx context.Context, s AuthSession) error {
	const q = `
INSERT INTO app_auth.user_session
    (id, user_account_id, created_at, expires_at, last_seen_at, ip, user_agent)
VALUES
    ($1, $2, $3, $4, $3, NULLIF($5, '')::inet, NULLIF($6, ''))
`
	if _, err := h.DB.ExecContext(ctx, q, s.ID, s.UserAccountID, s.CreatedAt.UTC(), s.ExpiresAt.UTC(), s.IP, s.UserAgent); err != nil {
		return fmt.Errorf("insert auth session: %w", err)
	}
	return nil
}

func (h authSessionRepositoryHandler) Get(ctx context.Context, id string) (*AuthSession, error) {
	const q = `
SELECT id, user_account_id, created_at, expires_at, last_seen_at,
       COALESCE(host(ip), ''), COALESCE(user_agent, '')
FROM app_auth.user_session
WHERE id = $1 AND expires_at > NOW()
`
	out := AuthSession{}
	err := h.DB.QueryRowContext(ctx, q, id).Scan(
		&out.ID,
		&out.UserAccountID,
		&out.CreatedAt,
		&out.ExpiresAt,
		&out.LastSeenAt,
		&out.IP,
		&out.UserAgent,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select auth session: %w", err)
	}
	return &out, nil
}

func (h authSessionRepositoryHandler) Touch(ctx context.Context, id string, newExpiresAt time.Time) error {
	const q = `
UPDATE app_auth.user_session
SET expires_at = $2, last_seen_at = NOW()
WHERE id = $1
`
	if _, err := h.DB.ExecContext(ctx, q, id, newExpiresAt.UTC()); err != nil {
		return fmt.Errorf("touch auth session: %w", err)
	}
	return nil
}

func (h authSessionRepositoryHandler) Delete(ctx context.Context, id string) error {
	if _, err := h.DB.ExecContext(ctx, `DELETE FROM app_auth.user_session WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete auth session: %w", err)
	}
	return nil
}

func (h authSessionRepositoryHandler) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	res, err := h.DB.ExecContext(ctx, `DELETE FROM app_auth.user_session WHERE expires_at <= $1`, before.UTC())
	if err != nil {
		return 0, fmt.Errorf("delete expired auth sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}
