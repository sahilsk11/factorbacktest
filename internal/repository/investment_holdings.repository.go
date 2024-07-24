package repository

import (
	"database/sql"
	"fmt"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/db/models/postgres/public/view"
	"factorbacktest/internal/domain"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

type InvestmentHoldingsRepository interface {
	Add(tx *sql.Tx, sih model.InvestmentHoldings) (*model.InvestmentHoldings, error)
	GetLatestVersionID(investmentID uuid.UUID) (*uuid.UUID, error)
	Get(id uuid.UUID) (*model.InvestmentHoldings, error)
	List(HoldingsListFilter) ([]model.InvestmentHoldings, error)
	GetLatestHoldings(tx *sql.Tx, investmentID uuid.UUID) (*domain.Portfolio, error)
}

type HoldingsListFilter struct {
}

type investmentHoldingsRepositoryHandler struct {
	Db *sql.DB
}

func NewInvestmentHoldingsRepository(db *sql.DB) InvestmentHoldingsRepository {
	return investmentHoldingsRepositoryHandler{Db: db}
}

func (h investmentHoldingsRepositoryHandler) GetLatestVersionID(investmentID uuid.UUID) (*uuid.UUID, error) {
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

func (h investmentHoldingsRepositoryHandler) Add(tx *sql.Tx, sih model.InvestmentHoldings) (*model.InvestmentHoldings, error) {
	sih.CreatedAt = time.Now().UTC()

	query := table.InvestmentHoldings.
		INSERT(
			table.InvestmentHoldings.MutableColumns,
		).
		MODEL(sih).
		RETURNING(table.InvestmentHoldings.AllColumns)

	out := model.InvestmentHoldings{}
	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert strategy investment holding: %w", err)
	}

	return &out, nil
}

func (h investmentHoldingsRepositoryHandler) Get(id uuid.UUID) (*model.InvestmentHoldings, error) {
	query := table.InvestmentHoldings.
		SELECT(table.InvestmentHoldings.AllColumns).
		WHERE(table.InvestmentHoldings.InvestmentHoldingsID.EQ(postgres.UUID(id)))

	result := model.InvestmentHoldings{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy investment holding: %w", err)
	}

	return &result, nil
}

// kind useless bc this gets all holdings, for all time
func (h investmentHoldingsRepositoryHandler) List(listFilter HoldingsListFilter) ([]model.InvestmentHoldings, error) {
	query := table.InvestmentHoldings.SELECT(table.InvestmentHoldings.AllColumns)

	result := []model.InvestmentHoldings{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list strategy investment holdings: %w", err)
	}

	return result, nil
}

func (h investmentHoldingsRepositoryHandler) GetLatestHoldings(tx *sql.Tx, investmentID uuid.UUID) (*domain.Portfolio, error) {
	query := view.LatestInvestmentHoldings.
		SELECT(view.LatestInvestmentHoldings.AllColumns).
		WHERE(
			view.LatestInvestmentHoldings.InvestmentID.EQ(
				postgres.UUID(investmentID),
			),
		)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	result := []model.LatestInvestmentHoldings{}
	err := query.Query(db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list strategy investment holdings: %w", err)
	}

	portfolio := portfolioFromHoldings(result)

	return portfolio, nil
}

func portfolioFromHoldings(holdings []model.LatestInvestmentHoldings) *domain.Portfolio {
	portfolio := domain.NewPortfolio()
	for _, h := range holdings {
		if *h.Symbol == ":CASH" {
			portfolio.Cash = h.Quantity
			continue
		}
		portfolio.Positions[*h.Symbol] = &domain.Position{
			Symbol:        *h.Symbol,
			TickerID:      *h.TickerID,
			Quantity:      h.Quantity.InexactFloat64(),
			ExactQuantity: *h.Quantity,
		}
	}
	return portfolio
}
