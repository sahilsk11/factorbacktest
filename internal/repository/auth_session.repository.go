package repository

import (
	"context"
	"database/sql"
	"errors"
	"factorbacktest/internal/db/models/postgres/app_auth/model"
	"factorbacktest/internal/db/models/postgres/app_auth/table"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
)

// AuthSessionRepository persists rows for the custom Go auth flow.
// Get returns ErrSessionNotFound (not a generic "no rows") so callers
// can fall through to unauthenticated cleanly.
type AuthSessionRepository interface {
	Create(ctx context.Context, s *model.UserSession) error
	// Get returns the session if it exists AND has not yet expired
	// (per expires_at, the sliding clock). The absolute lifetime cap
	// is enforced in the auth package, not here.
	Get(ctx context.Context, id string) (*model.UserSession, error)
	// Touch bumps expires_at + last_seen_at; called by the middleware
	// on every authenticated request to slide the expiration window.
	Touch(ctx context.Context, id string, newExpiresAt time.Time) error
	Delete(ctx context.Context, id string) error
	// DeleteExpired removes rows where expires_at <= before. Returns
	// the count for callers running it as a periodic cleanup job.
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

var ErrSessionNotFound = errors.New("auth session not found")

type authSessionRepositoryHandler struct {
	DB *sql.DB
}

func NewAuthSessionRepository(db *sql.DB) AuthSessionRepository {
	return authSessionRepositoryHandler{DB: db}
}

func (h authSessionRepositoryHandler) Create(ctx context.Context, s *model.UserSession) error {
	t := table.UserSession
	stmt := t.INSERT(
		t.ID, t.UserAccountID, t.CreatedAt, t.ExpiresAt, t.LastSeenAt, t.IP, t.UserAgent,
	).MODEL(s)
	if _, err := stmt.ExecContext(ctx, h.DB); err != nil {
		return fmt.Errorf("insert auth session: %w", err)
	}
	return nil
}

func (h authSessionRepositoryHandler) Get(ctx context.Context, id string) (*model.UserSession, error) {
	t := table.UserSession
	stmt := t.SELECT(t.AllColumns).WHERE(
		t.ID.EQ(postgres.String(id)).
			AND(t.ExpiresAt.GT(postgres.NOW())),
	).LIMIT(1)
	out := model.UserSession{}
	err := stmt.QueryContext(ctx, h.DB, &out)
	if errors.Is(err, qrm.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select auth session: %w", err)
	}
	return &out, nil
}

func (h authSessionRepositoryHandler) Touch(ctx context.Context, id string, newExpiresAt time.Time) error {
	t := table.UserSession
	stmt := t.UPDATE(t.ExpiresAt, t.LastSeenAt).
		SET(postgres.TimestampzT(newExpiresAt.UTC()), postgres.NOW()).
		WHERE(t.ID.EQ(postgres.String(id)))
	if _, err := stmt.ExecContext(ctx, h.DB); err != nil {
		return fmt.Errorf("touch auth session: %w", err)
	}
	return nil
}

func (h authSessionRepositoryHandler) Delete(ctx context.Context, id string) error {
	t := table.UserSession
	stmt := t.DELETE().WHERE(t.ID.EQ(postgres.String(id)))
	if _, err := stmt.ExecContext(ctx, h.DB); err != nil {
		return fmt.Errorf("delete auth session: %w", err)
	}
	return nil
}

func (h authSessionRepositoryHandler) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	t := table.UserSession
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
