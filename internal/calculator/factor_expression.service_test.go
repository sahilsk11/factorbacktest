package calculator

import (
	"context"
	"testing"
	"time"

	"factorbacktest/internal/data"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/testseed"
	"factorbacktest/internal/util"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCalculateFactorScores_excludesSymbolWithoutPrice(t *testing.T) {
	db, err := util.NewTestDb()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	suffix := uuid.NewString()[:8]
	priced := []model.Ticker{
		testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AAA." + suffix, Name: "priced-a"}),
		testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "BBB." + suffix, Name: "priced-b"}),
		testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "CCC." + suffix, Name: "priced-c"}),
	}
	unpriced := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "EA." + suffix, Name: "unpriced"})
	tickers := append(priced, unpriced)

	date := time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC)
	adj := repository.NewAdjustedPriceRepository(db)
	rows := make([]model.AdjustedPrice, 0, len(priced))
	for _, ticker := range priced {
		rows = append(rows, model.AdjustedPrice{
			Date:   date,
			Symbol: ticker.Symbol,
			Price:  decimal.NewFromInt(100),
		})
	}
	require.NoError(t, adj.Add(nil, rows))

	t.Cleanup(func() {
		for _, ticker := range tickers {
			_, _ = db.Exec(`DELETE FROM factor_score WHERE ticker_id = $1`, ticker.TickerID)
			_, _ = db.Exec(`DELETE FROM adjusted_price WHERE symbol = $1`, ticker.Symbol)
			_, _ = db.Exec(`DELETE FROM ticker WHERE ticker_id = $1`, ticker.TickerID)
		}
	})

	svc := NewFactorExpressionService(
		db,
		NewFactorMetricsHandler(adj, repository.AssetFundamentalsRepositoryHandler{}),
		data.NewPriceService(db, adj, nil, nil),
		repository.NewFactorScoreRepository(db),
		adj,
	)

	profile, endProfile := domain.NewProfile()
	defer endProfile()
	ctx := context.WithValue(context.Background(), domain.ContextProfileKey, profile)

	scores, err := svc.CalculateFactorScores(
		ctx,
		[]time.Time{date},
		tickers,
		"price(currentDate)",
	)
	require.NoError(t, err)
	require.Contains(t, scores, date)

	for _, ticker := range priced {
		require.NotNil(t, scores[date].SymbolScores[ticker.Symbol], "priced symbol %s should be scored", ticker.Symbol)
	}
	_, scoredUnpriced := scores[date].SymbolScores[unpriced.Symbol]
	require.False(t, scoredUnpriced, "unpriced symbol must be excluded from scoring")
}
