package repository

import (
	"database/sql"
	"errors"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

// EmailPreferenceRepository manages user email preference records.
//
// Note: The underlying table name is `email_preference` (singular) as defined in migration 000051.
type EmailPreferenceRepository interface {
	Upsert(tx *sql.Tx, pref model.EmailPreference) (*model.EmailPreference, error)
	Get(userAccountID uuid.UUID, emailType model.EmailType) (*model.EmailPreference, error)
	// ListOptedInByEmailType returns EmailPreference rows for users who are opted-in for the given email type.
	// For now, we treat any frequency other than OFF as "opted in".
	ListOptedInByEmailType(emailType model.EmailType) ([]model.EmailPreference, error)
}

type emailPreferenceRepositoryHandler struct {
	Db *sql.DB
}

func NewEmailPreferenceRepository(db *sql.DB) EmailPreferenceRepository {
	return emailPreferenceRepositoryHandler{Db: db}
}

func (h emailPreferenceRepositoryHandler) Upsert(tx *sql.Tx, pref model.EmailPreference) (*model.EmailPreference, error) {
	now := time.Now().UTC()
	pref.CreatedAt = now
	pref.UpdatedAt = now

	t := table.EmailPreference
	query := t.INSERT(t.MutableColumns).
		MODEL(pref).
		ON_CONFLICT(
			t.UserAccountID,
			t.EmailType,
		).
		DO_UPDATE(
			postgres.SET(
				t.Frequency.SET(t.EXCLUDED.Frequency),
				t.UpdatedAt.SET(t.EXCLUDED.UpdatedAt),
			),
		).
		RETURNING(t.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.EmailPreference{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert email preference: %w", err)
	}

	return &out, nil
}

func (h emailPreferenceRepositoryHandler) Get(userAccountID uuid.UUID, emailType model.EmailType) (*model.EmailPreference, error) {
	t := table.EmailPreference
	query := t.SELECT(t.AllColumns).
		WHERE(
			postgres.AND(
				t.UserAccountID.EQ(postgres.UUID(userAccountID)),
				// email_type is a Postgres enum; use NewEnumValue to avoid `operator does not exist: email_type = text`
				t.EmailType.EQ(postgres.NewEnumValue(emailType.String())),
			),
		).
		LIMIT(1)

	out := model.EmailPreference{}
	err := query.Query(h.Db, &out)
	if errors.Is(err, qrm.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email preference: %w", err)
	}

	return &out, nil
}

func (h emailPreferenceRepositoryHandler) ListOptedInByEmailType(emailType model.EmailType) ([]model.EmailPreference, error) {
	t := table.EmailPreference
	query := t.SELECT(t.AllColumns).
		// email_type is a Postgres enum; use NewEnumValue to avoid `operator does not exist: email_type = text`
		WHERE(t.EmailType.EQ(postgres.NewEnumValue(emailType.String())))

	rows := []model.EmailPreference{}
	err := query.Query(h.Db, &rows)
	if errors.Is(err, qrm.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list opted-in preferences for email type %s: %w", emailType.String(), err)
	}

	out := []model.EmailPreference{}
	for _, r := range rows {
		if r.Frequency != model.EmailFrequency_Off {
			out = append(out, r)
		}
	}

	return out, nil
}
