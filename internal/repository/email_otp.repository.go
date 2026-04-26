package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EmailOTP is the persisted state of one outstanding email login code.
// Mirrors auth_session in lifecycle: created when /auth/email/send is
// called, consumed (single-use) on /auth/email/verify success, and
// otherwise pruned by expires_at. There are no JET models for this
// table — the row shape is small and stable enough that hand-rolled
// SQL is simpler than running the JET regen against a live Postgres.
type EmailOTP struct {
	EmailOTPID    uuid.UUID
	Email         string
	CodeHash      string
	ExpiresAt     time.Time
	AttemptsLeft  int
	ConsumedAt    *time.Time
	IPCreatedFrom *string
	CreatedAt     time.Time
}

// EmailOTPRepository is a small CRUD surface for app_auth.email_otp.
// All callers run inside the auth package; semantics are spelled out
// in each method comment so the auth handler doesn't have to repeat
// invariants.
type EmailOTPRepository interface {
	// Create inserts a new OTP row. The returned EmailOTP has the
	// server-side defaults filled in (email_otp_id, created_at).
	Create(ctx context.Context, in *EmailOTP) (*EmailOTP, error)

	// LatestUnconsumedByEmail returns the most recent unexpired,
	// unconsumed OTP for the given (caller-lowercased, trimmed)
	// email, or ErrEmailOTPNotFound if none exists. Stale rows
	// (expired or consumed) are NOT returned — verify treats their
	// absence as "no pending verification" and 401s.
	LatestUnconsumedByEmail(ctx context.Context, email string, now time.Time) (*EmailOTP, error)

	// MarkConsumed atomically sets consumed_at = now. The WHERE
	// includes consumed_at IS NULL so a double-consume is a no-op
	// at the SQL layer (single-use guarantee even under handler
	// reentrance).
	MarkConsumed(ctx context.Context, id uuid.UUID, now time.Time) error

	// DecrementAttempts atomically decreases attempts_left by 1,
	// floored at 0. Called on every wrong-code submission; once it
	// hits 0 the handler stops calling bcrypt.Compare entirely.
	DecrementAttempts(ctx context.Context, id uuid.UUID) error
}

// ErrEmailOTPNotFound is returned by LatestUnconsumedByEmail when no
// active OTP row exists. Sentinel-checked by the auth package; do not
// wrap into a different error type.
var ErrEmailOTPNotFound = errors.New("email otp not found")

type emailOTPRepositoryHandler struct {
	DB *sql.DB
}

func NewEmailOTPRepository(db *sql.DB) EmailOTPRepository {
	return emailOTPRepositoryHandler{DB: db}
}

func (h emailOTPRepositoryHandler) Create(ctx context.Context, in *EmailOTP) (*EmailOTP, error) {
	if in == nil {
		return nil, errors.New("email_otp create: input is nil")
	}
	const q = `
		INSERT INTO app_auth.email_otp (
			email, code_hash, expires_at, attempts_left, ip_created_from
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING email_otp_id, email, code_hash, expires_at,
		          attempts_left, consumed_at, ip_created_from, created_at`
	out := EmailOTP{}
	err := h.DB.QueryRowContext(ctx, q,
		in.Email, in.CodeHash, in.ExpiresAt.UTC(), in.AttemptsLeft, in.IPCreatedFrom,
	).Scan(
		&out.EmailOTPID, &out.Email, &out.CodeHash, &out.ExpiresAt,
		&out.AttemptsLeft, &out.ConsumedAt, &out.IPCreatedFrom, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert email_otp: %w", err)
	}
	return &out, nil
}

func (h emailOTPRepositoryHandler) LatestUnconsumedByEmail(ctx context.Context, email string, now time.Time) (*EmailOTP, error) {
	const q = `
		SELECT email_otp_id, email, code_hash, expires_at,
		       attempts_left, consumed_at, ip_created_from, created_at
		FROM app_auth.email_otp
		WHERE email = $1
		  AND consumed_at IS NULL
		  AND expires_at > $2
		ORDER BY created_at DESC
		LIMIT 1`
	out := EmailOTP{}
	err := h.DB.QueryRowContext(ctx, q, email, now.UTC()).Scan(
		&out.EmailOTPID, &out.Email, &out.CodeHash, &out.ExpiresAt,
		&out.AttemptsLeft, &out.ConsumedAt, &out.IPCreatedFrom, &out.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEmailOTPNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select email_otp: %w", err)
	}
	return &out, nil
}

func (h emailOTPRepositoryHandler) MarkConsumed(ctx context.Context, id uuid.UUID, now time.Time) error {
	const q = `
		UPDATE app_auth.email_otp
		SET consumed_at = $2
		WHERE email_otp_id = $1
		  AND consumed_at IS NULL`
	if _, err := h.DB.ExecContext(ctx, q, id, now.UTC()); err != nil {
		return fmt.Errorf("update email_otp consumed_at: %w", err)
	}
	return nil
}

func (h emailOTPRepositoryHandler) DecrementAttempts(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE app_auth.email_otp
		SET attempts_left = GREATEST(attempts_left - 1, 0)
		WHERE email_otp_id = $1`
	if _, err := h.DB.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("update email_otp attempts_left: %w", err)
	}
	return nil
}
