package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"

	"github.com/go-jet/jet/v2/postgres"
)

type TickerRepository interface {
	List() ([]model.Ticker, error)
	GetOrCreate(tx *sql.Tx, t model.Ticker) (*model.Ticker, error)
}

type tickerRepositoryHandler struct {
	Db *sql.DB
}

func NewTickerRepository(db *sql.DB) TickerRepository {
	return tickerRepositoryHandler{Db: db}
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

func (h tickerRepositoryHandler) GetOrCreate(tx *sql.Tx, t model.Ticker) (*model.Ticker, error) {
	query := table.Ticker.
		INSERT(table.Ticker.MutableColumns).
		MODEL(t).
		ON_CONFLICT(table.Ticker.Symbol).DO_UPDATE(
		postgres.SET(
			table.Ticker.Symbol.SET(table.Ticker.EXCLUDED.Symbol),
		),
	).RETURNING(table.Ticker.AllColumns)

	out := model.Ticker{}
	err := query.Query(tx, &out)
	if err != nil {
		return nil, fmt.Errorf("failed to insert ticker: %w", err)
	}

	return &out, nil
}
