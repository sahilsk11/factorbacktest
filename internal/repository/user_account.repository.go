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
)

type UserAccountRepository interface {
	GetOrCreate(input *model.UserAccount) (*model.UserAccount, error)
}

type userAccountRepositoryHandler struct {
	DB *sql.DB
}

func NewUserAccountRepository(db *sql.DB) UserAccountRepository {
	return userAccountRepositoryHandler{
		DB: db,
	}
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
