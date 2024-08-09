package repository

import (
	"database/sql"
	"fmt"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type RebalancePriceRepository interface {
	Add(tx *sql.Tx, rp model.RebalancePrice) (*model.RebalancePrice, error)
	AddMany(pm map[string]decimal.Decimal, tickerIDMap map[string]uuid.UUID, rebalancerRunID uuid.UUID) error
	Get(tx *sql.Tx, id uuid.UUID) (*model.RebalancePrice, error)
	List(tx *sql.Tx) ([]model.RebalancePrice, error)
}

type rebalancePriceRepositoryHandler struct {
	Db *sql.DB
}

func NewRebalancePriceRepository(db *sql.DB) RebalancePriceRepository {
	return rebalancePriceRepositoryHandler{Db: db}
}

func (h rebalancePriceRepositoryHandler) Add(tx *sql.Tx, rp model.RebalancePrice) (*model.RebalancePrice, error) {
	rp.CreatedAt = time.Now().UTC()
	query := table.RebalancePrice.
		INSERT(table.RebalancePrice.MutableColumns).
		MODEL(rp).
		RETURNING(table.RebalancePrice.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.RebalancePrice{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert rebalance price: %w", err)
	}

	return &out, nil
}

func (h rebalancePriceRepositoryHandler) Get(tx *sql.Tx, id uuid.UUID) (*model.RebalancePrice, error) {
	query := table.RebalancePrice.
		SELECT(table.RebalancePrice.AllColumns).
		WHERE(table.RebalancePrice.RebalancePriceID.EQ(postgres.UUID(id)))

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	result := model.RebalancePrice{}
	err := query.Query(db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get rebalance price: %w", err)
	}

	return &result, nil
}

func (h rebalancePriceRepositoryHandler) AddMany(pm map[string]decimal.Decimal, tickerIDMap map[string]uuid.UUID, rebalancerRunID uuid.UUID) error {
	t := table.RebalancePrice

	models := []model.RebalancePrice{}
	for symbol, price := range pm {
		tickerID, ok := tickerIDMap[symbol]
		if !ok {
			return fmt.Errorf("missing tickerID for %s", symbol)
		}
		models = append(models, model.RebalancePrice{
			TickerID:        tickerID,
			Price:           price,
			RebalancerRunID: rebalancerRunID,
			CreatedAt:       time.Time{},
		})
	}

	query := t.INSERT(t.MutableColumns).MODELS(models)
	_, err := query.Exec(h.Db)
	if err != nil {
		return err
	}

	return nil
}

func (h rebalancePriceRepositoryHandler) List(tx *sql.Tx) ([]model.RebalancePrice, error) {
	query := table.RebalancePrice.SELECT(table.RebalancePrice.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	result := []model.RebalancePrice{}
	err := query.Query(db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list rebalance prices: %w", err)
	}

	return result, nil
}
