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
	"github.com/google/uuid"
)

// EmailOTPRepository persists rows for the email-OTP login flow.
// LatestUnconsumedByEmail returns ErrEmailOTPNotFound (not a generic
// "no rows") so the auth handler can map it cleanly to its 401-on-failure
// response shape.
type EmailOTPRepository interface {
	// Create inserts a new OTP row. The returned row has the
	// server-side defaults filled in (email_otp_id, created_at).
	Create(ctx context.Context, in *model.EmailOtp) (*model.EmailOtp, error)

	// LatestUnconsumedByEmail returns the most recent unexpired,
	// unconsumed OTP for the given (caller-lowercased, trimmed)
	// email, or ErrEmailOTPNotFound if none exists. Stale rows
	// (expired or already consumed) are NOT returned — verify
	// treats their absence as "no pending verification" and 401s.
	LatestUnconsumedByEmail(ctx context.Context, email string, now time.Time) (*model.EmailOtp, error)

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

func (h emailOTPRepositoryHandler) Create(ctx context.Context, in *model.EmailOtp) (*model.EmailOtp, error) {
	if in == nil {
		return nil, errors.New("email_otp create: input is nil")
	}
	t := table.EmailOtp
	stmt := t.INSERT(
		t.Email, t.CodeHash, t.ExpiresAt, t.AttemptsLeft, t.IPCreatedFrom,
	).MODEL(in).RETURNING(t.AllColumns)

	out := model.EmailOtp{}
	if err := stmt.QueryContext(ctx, h.DB, &out); err != nil {
		return nil, fmt.Errorf("insert email_otp: %w", err)
	}
	return &out, nil
}

func (h emailOTPRepositoryHandler) LatestUnconsumedByEmail(ctx context.Context, email string, now time.Time) (*model.EmailOtp, error) {
	t := table.EmailOtp
	stmt := t.SELECT(t.AllColumns).WHERE(
		t.Email.EQ(postgres.String(email)).
			AND(t.ConsumedAt.IS_NULL()).
			AND(t.ExpiresAt.GT(postgres.TimestampzT(now.UTC()))),
	).ORDER_BY(t.CreatedAt.DESC()).LIMIT(1)

	out := model.EmailOtp{}
	err := stmt.QueryContext(ctx, h.DB, &out)
	if errors.Is(err, qrm.ErrNoRows) {
		return nil, ErrEmailOTPNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select email_otp: %w", err)
	}
	return &out, nil
}

func (h emailOTPRepositoryHandler) MarkConsumed(ctx context.Context, id uuid.UUID, now time.Time) error {
	t := table.EmailOtp
	stmt := t.UPDATE(t.ConsumedAt).
		SET(postgres.TimestampzT(now.UTC())).
		WHERE(
			t.EmailOtpID.EQ(postgres.UUID(id)).
				AND(t.ConsumedAt.IS_NULL()),
		)
	if _, err := stmt.ExecContext(ctx, h.DB); err != nil {
		return fmt.Errorf("update email_otp consumed_at: %w", err)
	}
	return nil
}

// DecrementAttempts subtracts 1 from attempts_left, floored at 0. The
// floor matters because the bcrypt-compare guard in handleEmailVerify
// trips on attempts_left <= 0; without the GREATEST a row could
// overflow into negative values and the comparison logic would still
// be correct, but the DB column would lie about reality. Doing the
// arithmetic in SQL also keeps it race-free against concurrent verify
// calls on the same OTP row.
func (h emailOTPRepositoryHandler) DecrementAttempts(ctx context.Context, id uuid.UUID) error {
	t := table.EmailOtp
	floored := postgres.GREATEST(
		t.AttemptsLeft.SUB(postgres.Int(1)),
		postgres.Int(0),
	)
	stmt := t.UPDATE(t.AttemptsLeft).
		SET(floored).
		WHERE(t.EmailOtpID.EQ(postgres.UUID(id)))
	if _, err := stmt.ExecContext(ctx, h.DB); err != nil {
		return fmt.Errorf("update email_otp attempts_left: %w", err)
	}
	return nil
}
