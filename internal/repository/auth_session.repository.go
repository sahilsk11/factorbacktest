package repository

import (
	"context"
	"database/sql"
	"errors"
	authmodel "factorbacktest/internal/db/models/postgres/app_auth/model"
	authtable "factorbacktest/internal/db/models/postgres/app_auth/table"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

// AuthSession is the storage shape of a row in app_auth.user_session.
// The auth package (internal/auth) is the only intended caller; if you
// need it from a handler, reach for `auth.CurrentUser(c)` instead.
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
// Get returns ErrSessionNotFound (not a generic "no rows") so callers can
// fall through to unauthenticated cleanly.
type AuthSessionRepository interface {
	Create(ctx context.Context, s AuthSession) error
	// Get returns the session if it exists AND has not yet expired (per
	// expires_at, the sliding clock). The absolute lifetime cap is
	// enforced in the auth package, not here.
	Get(ctx context.Context, id string) (*AuthSession, error)
	// Touch bumps expires_at + last_seen_at; called by the middleware on
	// every authenticated request to slide the expiration window.
	Touch(ctx context.Context, id string, newExpiresAt time.Time) error
	Delete(ctx context.Context, id string) error
	// DeleteExpired removes rows where expires_at <= before. Returns the
	// count for callers running it as a periodic cleanup job.
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

var ErrSessionNotFound = errors.New("auth session not found")

type authSessionRepositoryHandler struct {
	DB *sql.DB
}

func NewAuthSessionRepository(db *sql.DB) AuthSessionRepository {
	return authSessionRepositoryHandler{DB: db}
}

func (h authSessionRepositoryHandler) Create(ctx context.Context, s AuthSession) error {
	row := authmodel.UserSession{
		ID:            s.ID,
		UserAccountID: s.UserAccountID,
		CreatedAt:     s.CreatedAt.UTC(),
		ExpiresAt:     s.ExpiresAt.UTC(),
		LastSeenAt:    s.LastSeenAt.UTC(),
		IP:            nilIfEmpty(s.IP),
		UserAgent:     nilIfEmpty(s.UserAgent),
	}
	t := authtable.UserSession
	stmt := t.INSERT(t.ID, t.UserAccountID, t.CreatedAt, t.ExpiresAt, t.LastSeenAt, t.IP, t.UserAgent).MODEL(row)
	if _, err := stmt.ExecContext(ctx, h.DB); err != nil {
		return fmt.Errorf("insert auth session: %w", err)
	}
	return nil
}

func (h authSessionRepositoryHandler) Get(ctx context.Context, id string) (*AuthSession, error) {
	t := authtable.UserSession
	stmt := t.SELECT(t.AllColumns).WHERE(
		t.ID.EQ(postgres.String(id)).
			AND(t.ExpiresAt.GT(postgres.NOW())),
	).LIMIT(1)
	out := authmodel.UserSession{}
	err := stmt.QueryContext(ctx, h.DB, &out)
	if errors.Is(err, qrm.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select auth session: %w", err)
	}
	return &AuthSession{
		ID:            out.ID,
		UserAccountID: out.UserAccountID,
		CreatedAt:     out.CreatedAt,
		ExpiresAt:     out.ExpiresAt,
		LastSeenAt:    out.LastSeenAt,
		IP:            stringOrEmpty(out.IP),
		UserAgent:     stringOrEmpty(out.UserAgent),
	}, nil
}

func (h authSessionRepositoryHandler) Touch(ctx context.Context, id string, newExpiresAt time.Time) error {
	t := authtable.UserSession
	stmt := t.UPDATE(t.ExpiresAt, t.LastSeenAt).
		SET(postgres.TimestampzT(newExpiresAt.UTC()), postgres.NOW()).
		WHERE(t.ID.EQ(postgres.String(id)))
	if _, err := stmt.ExecContext(ctx, h.DB); err != nil {
		return fmt.Errorf("touch auth session: %w", err)
	}
	return nil
}

func (h authSessionRepositoryHandler) Delete(ctx context.Context, id string) error {
	t := authtable.UserSession
	stmt := t.DELETE().WHERE(t.ID.EQ(postgres.String(id)))
	if _, err := stmt.ExecContext(ctx, h.DB); err != nil {
		return fmt.Errorf("delete auth session: %w", err)
	}
	return nil
}

func (h authSessionRepositoryHandler) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	t := authtable.UserSession
	stmt := t.DELETE().WHERE(t.ExpiresAt.LT_EQ(postgres.TimestampzT(before.UTC())))
	res, err := stmt.ExecContext(ctx, h.DB)
	if err != nil {
		return 0, fmt.Errorf("delete expired auth sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func stringOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
