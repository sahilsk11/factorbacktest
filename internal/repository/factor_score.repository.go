package repository

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
)

type FactorScoreRepository interface {
	GetMany([]FactorScoreGetManyInput) (map[time.Time]map[uuid.UUID]float64, error)
	AddMany([]*model.FactorScore) error
}

type factorScoreRepositoryHandler struct {
	Db *sql.DB
}

func NewFactorScoreRepository(db *sql.DB) FactorScoreRepository {
	return factorScoreRepositoryHandler{db}
}

func (h factorScoreRepositoryHandler) AddMany(in []*model.FactorScore) error {
	if len(in) == 0 {
		return nil
	}

	for _, x := range in {
		x.CreatedAt = time.Now().UTC()
		x.UpdatedAt = time.Now().UTC()
	}
	query := table.FactorScore.INSERT(table.FactorScore.MutableColumns).
		MODELS(in).
		ON_CONFLICT(
			table.FactorScore.FactorExpressionHash,
			table.FactorScore.Date,
			table.FactorScore.TickerID,
		).
		DO_UPDATE(
			postgres.SET(
				table.FactorScore.UpdatedAt.SET(table.FactorScore.EXCLUDED.UpdatedAt),
			),
		)
	_, err := query.Exec(h.Db)
	if err != nil {
		return fmt.Errorf("failed to create factor scores in db: %w", err)
	}

	return nil
}

type FactorScoreGetManyInput struct {
	FactorExpressionHash string
	Ticker               model.Ticker
	Date                 time.Time
}

func (h factorScoreRepositoryHandler) GetMany(inputs []FactorScoreGetManyInput) (map[time.Time]map[uuid.UUID]float64, error) {
	tickerIdToSymbol := map[uuid.UUID]string{}
	expressions := []postgres.BoolExpression{}
	for _, in := range inputs {
		tickerIdToSymbol[in.Ticker.TickerID] = in.Ticker.Symbol
		expressions = append(expressions, postgres.AND(
			table.FactorScore.FactorExpressionHash.EQ(postgres.String(in.FactorExpressionHash)),
			table.FactorScore.Date.EQ(postgres.DateT(in.Date)),
			table.FactorScore.TickerID.EQ(postgres.UUID(in.Ticker.TickerID)),
		))
	}
	query := table.FactorScore.SELECT(table.FactorScore.AllColumns).
		WHERE(postgres.OR(expressions...))

	out := []model.FactorScore{}
	err := query.Query(h.Db, &out)
	if err != nil {
		return nil, err
	}

	results := map[time.Time]map[uuid.UUID]float64{}
	for _, m := range out {
		if _, ok := results[m.Date]; !ok {
			results[m.Date] = map[uuid.UUID]float64{}
		}
		results[m.Date][m.TickerID] = m.Score
	}

	return results, nil
}
