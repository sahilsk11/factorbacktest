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

type StrategyInvestmentHoldingsRepository interface {
	Add(tx *sql.Tx, sih model.StrategyInvestmentHoldings) (*model.StrategyInvestmentHoldings, error)
	Get(id uuid.UUID) (*model.StrategyInvestmentHoldings, error)
	List(HoldingsListFilter) ([]model.StrategyInvestmentHoldings, error)
	GetLatestHoldings(savedStrategyID uuid.UUID) (*domain.Portfolio, error)
}

type HoldingsListFilter struct {
	StrategyID *uuid.UUID
}

type strategyInvestmentHoldingsRepositoryHandler struct {
	Db *sql.DB
}

func NewStrategyInvestmentHoldingsRepository(db *sql.DB) StrategyInvestmentHoldingsRepository {
	return strategyInvestmentHoldingsRepositoryHandler{Db: db}
}

func (h strategyInvestmentHoldingsRepositoryHandler) Add(tx *sql.Tx, sih model.StrategyInvestmentHoldings) (*model.StrategyInvestmentHoldings, error) {
	sih.CreatedAt = time.Now().UTC()
	query := table.StrategyInvestmentHoldings.
		INSERT(
			table.StrategyInvestmentHoldings.StrategyInvestmentID,
			table.StrategyInvestmentHoldings.Date,
			table.StrategyInvestmentHoldings.Ticker,
			table.StrategyInvestmentHoldings.Quantity,
		).
		MODEL(sih).
		RETURNING(table.StrategyInvestmentHoldings.AllColumns)

	out := model.StrategyInvestmentHoldings{}
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

func (h strategyInvestmentHoldingsRepositoryHandler) Get(id uuid.UUID) (*model.StrategyInvestmentHoldings, error) {
	query := table.StrategyInvestmentHoldings.
		SELECT(table.StrategyInvestmentHoldings.AllColumns).
		WHERE(table.StrategyInvestmentHoldings.StrategyInvestmentHoldingsID.EQ(postgres.UUID(id)))

	result := model.StrategyInvestmentHoldings{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy investment holding: %w", err)
	}

	return &result, nil
}

// kind useless bc this gets all holdings, for all time
func (h strategyInvestmentHoldingsRepositoryHandler) List(listFilter HoldingsListFilter) ([]model.StrategyInvestmentHoldings, error) {
	query := table.StrategyInvestmentHoldings.SELECT(table.StrategyInvestmentHoldings.AllColumns)

	if listFilter.StrategyID != nil {
		query = query.WHERE(
			table.StrategyInvestmentHoldings.StrategyInvestmentID.EQ(
				postgres.UUID(*listFilter.StrategyID),
			),
		)
	}

	result := []model.StrategyInvestmentHoldings{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list strategy investment holdings: %w", err)
	}

	return result, nil
}

func (h strategyInvestmentHoldingsRepositoryHandler) GetLatestHoldings(savedStrategyID uuid.UUID) (*domain.Portfolio, error) {
	query := view.LatestStrategyInvestmentHoldings.
		SELECT(view.LatestStrategyInvestmentHoldings.AllColumns).
		WHERE(
			view.LatestStrategyInvestmentHoldings.StrategyInvestmentID.EQ(
				postgres.UUID(savedStrategyID),
			),
		)

	result := []model.LatestStrategyInvestmentHoldings{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list strategy investment holdings: %w", err)
	}

	portfolio := portfolioFromHoldings(result)

	return portfolio, nil
}

func portfolioFromHoldings(holdings []model.LatestStrategyInvestmentHoldings) *domain.Portfolio {
	portfolio := domain.NewPortfolio()
	for _, h := range holdings {
		if *h.Symbol == ":CASH" {
			portfolio.Cash = h.Quantity.InexactFloat64()
			continue
		}
		portfolio.Positions[*h.Symbol] = &domain.Position{
			Symbol:        *h.Symbol,
			TickerID:      *h.Ticker, // should be called TickerID
			Quantity:      h.Quantity.InexactFloat64(),
			ExactQuantity: *h.Quantity,
		}
	}
	return portfolio
}
