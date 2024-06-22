package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"time"
)

type FactorScoreRepository interface{}

type factorScoreRepositoryHandler struct {
	Db *sql.DB
}

func NewFactorScoreRepository(db *sql.DB) FactorScoreRepository {
	return factorScoreRepositoryHandler{db}
}

func Add(tx *sql.Tx, ticker model.Ticker, score float64, date time.Time) error {
	return nil
}

type FactorScoreGetManyInput struct {
	FactorExpressionHash string
	Ticker               model.Ticker
	Date                 time.Time
}

func GetMany(inputs []FactorScoreGetManyInput)
