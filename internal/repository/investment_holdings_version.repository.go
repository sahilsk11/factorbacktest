package repository

import (
	"database/sql"
	"fmt"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
)

type InvestmentHoldingsVersionRepository interface {
	Add(tx *sql.Tx, ihv model.InvestmentHoldingsVersion) (*model.InvestmentHoldingsVersion, error)
	Get(id uuid.UUID) (*model.InvestmentHoldingsVersion, error)
	List() ([]model.InvestmentHoldingsVersion, error)
	GetLatestVersionID(investmentID uuid.UUID) (*uuid.UUID, error)
	GetEarliestVersionID(investmentID uuid.UUID) (*uuid.UUID, error)
}

type investmentHoldingsVersionRepositoryHandler struct {
	Db *sql.DB
}

func NewInvestmentHoldingsVersionRepository(db *sql.DB) InvestmentHoldingsVersionRepository {
	return investmentHoldingsVersionRepositoryHandler{Db: db}
}

func (h investmentHoldingsVersionRepositoryHandler) Add(tx *sql.Tx, ihv model.InvestmentHoldingsVersion) (*model.InvestmentHoldingsVersion, error) {
	ihv.CreatedAt = time.Now().UTC()
	query := table.InvestmentHoldingsVersion.
		INSERT(
			table.InvestmentHoldingsVersion.MutableColumns,
		).
		MODEL(ihv).
		RETURNING(table.InvestmentHoldingsVersion.AllColumns)

	out := model.InvestmentHoldingsVersion{}
	err := query.Query(tx, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert investment holdings version: %w", err)
	}

	return &out, nil
}

func (h investmentHoldingsVersionRepositoryHandler) Get(id uuid.UUID) (*model.InvestmentHoldingsVersion, error) {
	query := table.InvestmentHoldingsVersion.
		SELECT(table.InvestmentHoldingsVersion.AllColumns).
		WHERE(table.InvestmentHoldingsVersion.InvestmentHoldingsVersionID.EQ(postgres.UUID(id)))

	result := model.InvestmentHoldingsVersion{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get investment holdings version: %w", err)
	}

	return &result, nil
}

func (h investmentHoldingsVersionRepositoryHandler) List() ([]model.InvestmentHoldingsVersion, error) {
	query := table.InvestmentHoldingsVersion.SELECT(table.InvestmentHoldingsVersion.AllColumns)
	result := []model.InvestmentHoldingsVersion{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list investment holdings versions: %w", err)
	}

	return result, nil
}

func (h investmentHoldingsVersionRepositoryHandler) GetLatestVersionID(investmentID uuid.UUID) (*uuid.UUID, error) {
	query := table.InvestmentHoldingsVersion.SELECT(
		table.InvestmentHoldingsVersion.InvestmentHoldingsVersionID,
	).WHERE(
		table.InvestmentHoldingsVersion.InvestmentID.EQ(postgres.UUID(investmentID)),
	).ORDER_BY(
		table.InvestmentHoldingsVersion.CreatedAt.DESC(),
	).LIMIT(1)

	type InvestmentHoldingsVersion struct {
		InvestmentHoldingsVersionID uuid.UUID
	}

	var out InvestmentHoldingsVersion
	err := query.Query(h.Db, &out)
	if err != nil {
		return nil, fmt.Errorf("could not get latest holdings version for investment %s: %w", investmentID.String(), err)
	}

	return &out.InvestmentHoldingsVersionID, nil
}

func (h investmentHoldingsVersionRepositoryHandler) GetEarliestVersionID(investmentID uuid.UUID) (*uuid.UUID, error) {
	query := table.InvestmentHoldingsVersion.SELECT(
		table.InvestmentHoldingsVersion.InvestmentHoldingsVersionID,
	).WHERE(
		table.InvestmentHoldingsVersion.InvestmentID.EQ(postgres.UUID(investmentID)),
	).ORDER_BY(
		table.InvestmentHoldingsVersion.CreatedAt.ASC(),
	).LIMIT(1)

	type InvestmentHoldingsVersion struct {
		InvestmentHoldingsVersionID uuid.UUID
	}

	var out InvestmentHoldingsVersion
	err := query.Query(h.Db, &out)
	if err != nil {
		return nil, fmt.Errorf("could not get latest holdings version for investment %s: %w", investmentID.String(), err)
	}

	return &out.InvestmentHoldingsVersionID, nil
}
