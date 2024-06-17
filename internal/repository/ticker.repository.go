package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"
)

type TickerRepository interface {
	List() ([]model.Ticker, error)
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
