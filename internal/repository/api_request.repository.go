package repository

import (
	"context"
	"factorbacktest/internal/db/models/postgres/public/model"
	. "factorbacktest/internal/db/models/postgres/public/table"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

type ApiRequestRepository interface {
	Add(db qrm.Queryable, ar model.APIRequest) (*model.APIRequest, error)
	AddContext(ctx context.Context, db qrm.Queryable, ar model.APIRequest) (*model.APIRequest, error)
	Update(db qrm.Executable, ar model.APIRequest) error
	UpdateContext(ctx context.Context, db qrm.Executable, ar model.APIRequest) error
}

type ApiRequestRepositoryHandler struct{}

func (h ApiRequestRepositoryHandler) Add(db qrm.Queryable, ar model.APIRequest) (*model.APIRequest, error) {
	return h.AddContext(context.Background(), db, ar)
}

func (h ApiRequestRepositoryHandler) AddContext(ctx context.Context, db qrm.Queryable, ar model.APIRequest) (*model.APIRequest, error) {
	ar.RequestID = uuid.New()

	query := APIRequest.
		INSERT(APIRequest.MutableColumns).
		MODEL(ar).
		RETURNING(APIRequest.AllColumns)

	out := &model.APIRequest{}
	err := query.QueryContext(ctx, db, out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert API request: %w", err)
	}

	return out, nil
}

func (h ApiRequestRepositoryHandler) Update(db qrm.Executable, ar model.APIRequest) error {
	return h.UpdateContext(context.Background(), db, ar)
}

func (h ApiRequestRepositoryHandler) UpdateContext(ctx context.Context, db qrm.Executable, ar model.APIRequest) error {
	query := APIRequest.
		UPDATE(APIRequest.DurationMs, APIRequest.StatusCode, APIRequest.ResponseBody).
		MODEL(ar).
		WHERE(APIRequest.RequestID.EQ(postgres.UUID(ar.RequestID)))

	_, err := query.ExecContext(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to update API request: %w", err)
	}

	return nil
}
