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
	GetMany([]FactorScoreGetManyInput) (map[time.Time]map[uuid.UUID]model.FactorScore, error)
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

	batchSize := 5000

	for start := 0; start < len(in); start += batchSize {
		end := start + batchSize
		if end > len(in) {
			end = len(in)
		}

		batch := in[start:end]
		query := table.FactorScore.INSERT(table.FactorScore.MutableColumns).
			MODELS(batch).
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
	}

	return nil
}

type FactorScoreGetManyInput struct {
	FactorExpressionHash string
	Ticker               model.Ticker
	Date                 time.Time
}

func (h factorScoreRepositoryHandler) GetMany(inputs []FactorScoreGetManyInput) (map[time.Time]map[uuid.UUID]model.FactorScore, error) {
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

	batchSize := int(60)
	results := map[time.Time]map[uuid.UUID]model.FactorScore{}

	x := time.Now()

	for start := 0; start < len(expressions); start += batchSize {
		end := start + batchSize
		if end > len(expressions) {
			end = len(expressions)
		}

		expr := expressions[start:end]

		query := table.FactorScore.SELECT(table.FactorScore.AllColumns).
			WHERE(postgres.OR(expr...))

		out := []model.FactorScore{}
		err := query.Query(h.Db, &out)
		if err != nil {
			return nil, err
		}

		for _, m := range out {
			if _, ok := results[m.Date]; !ok {
				results[m.Date] = map[uuid.UUID]model.FactorScore{}
			}
			results[m.Date][m.TickerID] = m
		}

	}
	fmt.Printf("query took %d ms\n", time.Since(x).Milliseconds())

	return results, nil
}
