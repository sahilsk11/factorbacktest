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

type UserAccountRepository interface {
	GetOrCreate(input *model.UserAccount) (*model.UserAccount, error)
	// GetOrCreateByProviderIdentity is the identity-correct lookup for
	// federated providers: it finds (or creates) a user keyed by the
	// (provider, provider_id) tuple, NOT by email. This is required for
	// OIDC providers like Google, where the stable identifier is the `sub`
	// claim and email can change. Implemented as INSERT ... ON CONFLICT
	// DO UPDATE so concurrent first-logins don't race-create duplicates.
	GetOrCreateByProviderIdentity(input *model.UserAccount) (*model.UserAccount, error)
	ListUsersWithEmail() ([]model.UserAccount, error)
	GetMany(userAccountIDs []uuid.UUID) ([]model.UserAccount, error)
	GetByID(userAccountID uuid.UUID) (*model.UserAccount, error)
}

type userAccountRepositoryHandler struct {
	DB *sql.DB
}

func NewUserAccountRepository(db *sql.DB) UserAccountRepository {
	return userAccountRepositoryHandler{
		DB: db,
	}
}

func (h userAccountRepositoryHandler) GetByID(userAccountID uuid.UUID) (*model.UserAccount, error) {
	t := table.UserAccount
	query := t.SELECT(t.AllColumns).
		WHERE(t.UserAccountID.EQ(postgres.UUID(userAccountID)))
	out := model.UserAccount{}
	err := query.Query(h.DB, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to get user account: %w", err)
	}
	return &out, nil
}

// GetOrCreateByProviderIdentity upserts a row keyed on the unique
// (provider, provider_id) constraint added in migration 000050. On conflict
// we update mutable profile attributes (email, first_name, last_name,
// phone_number) from the latest IdP claim — Google may change a user's
// email or display name and we want our copy to follow.
//
// Both ProviderID and Provider must be non-empty on input. The other
// fields are optional and only updated on conflict if non-nil.
//
// Implemented in raw SQL (not go-jet) because go-jet's INSERT...ON CONFLICT
// DO UPDATE syntax for upserts is verbose and error-prone for this shape.
func (h userAccountRepositoryHandler) GetOrCreateByProviderIdentity(input *model.UserAccount) (*model.UserAccount, error) {
	if input.ProviderID == nil || *input.ProviderID == "" {
		return nil, fmt.Errorf("GetOrCreateByProviderIdentity: ProviderID is required")
	}
	if input.Provider == "" {
		return nil, fmt.Errorf("GetOrCreateByProviderIdentity: Provider is required")
	}
	now := time.Now().UTC()

	// COALESCE on UPDATE so we don't blank out an existing email/name when
	// the new identity claim happens to be missing it (e.g. Google with a
	// minimal scope). The new value wins when present; the old value is
	// preserved when the new is nil.
	const q = `
INSERT INTO public.user_account
    (first_name, last_name, email, created_at, updated_at, provider, provider_id, phone_number)
VALUES
    ($1, $2, $3, $4, $4, $5, $6, $7)
ON CONFLICT (provider, provider_id) DO UPDATE SET
    email        = COALESCE(EXCLUDED.email,        public.user_account.email),
    first_name   = COALESCE(EXCLUDED.first_name,   public.user_account.first_name),
    last_name    = COALESCE(EXCLUDED.last_name,    public.user_account.last_name),
    phone_number = COALESCE(EXCLUDED.phone_number, public.user_account.phone_number),
    updated_at   = $4
RETURNING user_account_id, first_name, last_name, email, created_at, updated_at, provider, provider_id, phone_number;
`
	out := model.UserAccount{}
	err := h.DB.QueryRow(
		q,
		input.FirstName,
		input.LastName,
		input.Email,
		now,
		string(input.Provider),
		*input.ProviderID,
		input.PhoneNumber,
	).Scan(
		&out.UserAccountID,
		&out.FirstName,
		&out.LastName,
		&out.Email,
		&out.CreatedAt,
		&out.UpdatedAt,
		&out.Provider,
		&out.ProviderID,
		&out.PhoneNumber,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert user_account by (provider, provider_id): %w", err)
	}
	return &out, nil
}

func (h userAccountRepositoryHandler) GetOrCreate(input *model.UserAccount) (*model.UserAccount, error) {
	input.CreatedAt = time.Now().UTC()
	input.UpdatedAt = time.Now().UTC()

	t := table.UserAccount

	getQuery := t.SELECT(t.AllColumns)

	if input.Email != nil {
		getQuery = getQuery.WHERE(t.Email.EQ(postgres.String(*input.Email)))
	} else if input.PhoneNumber != nil {
		getQuery = getQuery.WHERE(t.PhoneNumber.EQ(postgres.String(*input.PhoneNumber)))
	}

	out := model.UserAccount{}
	err := getQuery.Query(h.DB, &out)
	if err != nil && !errors.Is(err, qrm.ErrNoRows) {
		return nil, fmt.Errorf("failed to get user account: %w", err)
	} else if err == nil {
		return &out, nil
	}

	createQuery := t.INSERT(t.MutableColumns).MODEL(input).RETURNING(t.AllColumns)

	err = createQuery.Query(h.DB, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &out, nil
}

func (h userAccountRepositoryHandler) ListUsersWithEmail() ([]model.UserAccount, error) {
	t := table.UserAccount
	query := t.SELECT(t.AllColumns).
		WHERE(t.Email.IS_NOT_NULL())

	result := []model.UserAccount{}
	err := query.Query(h.DB, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list users with email: %w", err)
	}

	// Filter out users with empty email strings
	filtered := []model.UserAccount{}
	for _, user := range result {
		if user.Email != nil && *user.Email != "" {
			filtered = append(filtered, user)
		}
	}

	return filtered, nil
}

func (h userAccountRepositoryHandler) GetMany(userAccountIDs []uuid.UUID) ([]model.UserAccount, error) {
	if len(userAccountIDs) == 0 {
		return []model.UserAccount{}, nil
	}

	t := table.UserAccount
	ids := []postgres.Expression{}
	for _, id := range userAccountIDs {
		ids = append(ids, postgres.UUID(id))
	}

	query := t.SELECT(t.AllColumns).
		WHERE(t.UserAccountID.IN(ids...))

	out := []model.UserAccount{}
	err := query.Query(h.DB, &out)
	if errors.Is(err, qrm.ErrNoRows) {
		return []model.UserAccount{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user accounts: %w", err)
	}

	return out, nil
}
