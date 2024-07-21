package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/google/uuid"
)

type TickerRepository interface {
	Get(tickerID uuid.UUID) (*model.Ticker, error)
	List() ([]model.Ticker, error)
	GetOrCreate(tx *sql.Tx, t model.Ticker) (*model.Ticker, error)
	GetCashTicker() (*model.Ticker, error)
}

type tickerRepositoryHandler struct {
	Db *sql.DB
}

func NewTickerRepository(db *sql.DB) TickerRepository {
	return tickerRepositoryHandler{Db: db}
}

const CASH_SYMBOL = ":CASH"

func (h tickerRepositoryHandler) GetCashTicker() (*model.Ticker, error) {
	query := table.Ticker.SELECT(table.Ticker.AllColumns).
		WHERE(table.Ticker.Symbol.EQ(postgres.String(":CASH")))

	result := model.Ticker{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get cash symbol: %w", err)
	}

	return &result, nil
}

func (h tickerRepositoryHandler) List() ([]model.Ticker, error) {
	query := table.Ticker.SELECT(table.Ticker.AllColumns)
	result := []model.Ticker{}
	err := query.Query(h.Db, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get universe symbols: %w", err)
	}

	return result, nil
}

func (h tickerRepositoryHandler) Get(tickerID uuid.UUID) (*model.Ticker, error) {
	query := table.Ticker.
		SELECT(table.Ticker.AllColumns).
		WHERE(table.Ticker.TickerID.EQ(
			postgres.UUID(tickerID),
		))

	out := model.Ticker{}
	err := query.Query(h.Db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker: %w", err)
	}

	return &out, nil
}

func (h tickerRepositoryHandler) GetOrCreate(tx *sql.Tx, t model.Ticker) (*model.Ticker, error) {
	query := table.Ticker.
		INSERT(table.Ticker.MutableColumns).
		MODEL(t).
		ON_CONFLICT(table.Ticker.Symbol).DO_UPDATE(
		postgres.SET(
			table.Ticker.Symbol.SET(table.Ticker.EXCLUDED.Symbol),
		),
	).RETURNING(table.Ticker.AllColumns)

	var db qrm.Queryable = h.Db
	if tx != nil {
		db = tx
	}

	out := model.Ticker{}
	err := query.Query(db, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert ticker: %w", err)
	}

	return &out, nil
}
