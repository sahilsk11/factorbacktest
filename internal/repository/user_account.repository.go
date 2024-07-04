package repository

import (
	"database/sql"
	"errors"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	googleauth "factorbacktest/pkg/google-auth"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
)

type UserAccountRepository interface {
	GetOrCreate(googleauth.GetUserDetailsResponse) (*model.UserAccount, error)
}

type userAccountRepositoryHandler struct {
	DB *sql.DB
}

func NewUserAccountRepository(db *sql.DB) UserAccountRepository {
	return userAccountRepositoryHandler{
		DB: db,
	}
}

func (h userAccountRepositoryHandler) GetOrCreate(googleUser googleauth.GetUserDetailsResponse) (*model.UserAccount, error) {
	t := table.UserAccount

	getQuery := t.SELECT(t.AllColumns).WHERE(t.Email.EQ(postgres.String(googleUser.Email)))
	out := model.UserAccount{}
	err := getQuery.Query(h.DB, &out)
	if err != nil && !errors.Is(err, qrm.ErrNoRows) {
		return nil, fmt.Errorf("failed to get user account: %w", err)
	} else if err == nil {
		return &out, nil
	}

	newModel := model.UserAccount{
		FirstName: googleUser.FirstName,
		LastName:  googleUser.LastName,
		Email:     googleUser.Email,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	createQuery := t.INSERT(t.MutableColumns).MODEL(newModel).RETURNING(t.AllColumns)

	err = createQuery.Query(h.DB, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &out, nil
}
