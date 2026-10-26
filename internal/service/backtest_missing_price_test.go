package service

import (
	"context"
	"testing"
	"time"

	"factorbacktest/internal/calculator"
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

func TestBacktest_skipsUnpricedSymbolForNewEntries(t *testing.T) {
	db, err := util.NewTestDb()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	suffix := uuid.NewString()[:8]
	priced := []model.Ticker{
		testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AAA." + suffix, Name: "priced-a"}),
		testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "BBB." + suffix, Name: "priced-b"}),
		testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "CCC." + suffix, Name: "priced-c"}),
		testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "DDD." + suffix, Name: "priced-d"}),
	}
	unpriced := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "EA." + suffix, Name: "unpriced"})
	tickers := append(append([]model.Ticker{}, priced...), unpriced)

	universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: "missing-px-" + suffix})
	for _, ticker := range tickers {
		testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, ticker.TickerID)
	}

	day := time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC)
	adj := repository.NewAdjustedPriceRepository(db)
	insertPriced := func(date time.Time) {
		rows := make([]model.AdjustedPrice, 0, len(priced))
		for i, ticker := range priced {
			rows = append(rows, model.AdjustedPrice{
				Date:   date,
				Symbol: ticker.Symbol,
				Price:  decimal.NewFromInt(int64(100 + i)),
			})
		}
		require.NoError(t, adj.Add(nil, rows))
	}
	insertPriced(day)
	if latest, err := adj.LatestTradingDay(); err == nil && latest != nil {
		insertPriced(*latest)
	}

	t.Cleanup(func() {
		for _, ticker := range tickers {
			_, _ = db.Exec(`DELETE FROM factor_score WHERE ticker_id = $1`, ticker.TickerID)
			_, _ = db.Exec(`DELETE FROM adjusted_price WHERE symbol = $1`, ticker.Symbol)
		}
		_, _ = db.Exec(`DELETE FROM asset_universe_ticker WHERE asset_universe_id = $1`, universe.AssetUniverseID)
		_, _ = db.Exec(`DELETE FROM asset_universe WHERE asset_universe_id = $1`, universe.AssetUniverseID)
		for _, ticker := range tickers {
			_, _ = db.Exec(`DELETE FROM ticker WHERE ticker_id = $1`, ticker.TickerID)
		}
	})

	handler := BacktestHandler{
		PriceRepository:         adj,
		AssetUniverseRepository: repository.NewAssetUniverseRepository(db),
		Db:                      db,
		PriceService:            data.NewPriceService(db, adj, nil, nil),
		FactorExpressionService: calculator.NewFactorExpressionService(
			db,
			calculator.NewFactorMetricsHandler(adj, repository.AssetFundamentalsRepositoryHandler{}),
			data.NewPriceService(db, adj, nil, nil),
			repository.NewFactorScoreRepository(db),
			adj,
		),
	}

	profile, endProfile := domain.NewProfile()
	defer endProfile()
	ctx := context.WithValue(context.Background(), domain.ContextProfileKey, profile)

	resp, err := handler.Backtest(ctx, BacktestInput{
		FactorExpression:  "price(currentDate)",
		BacktestStart:     day,
		BacktestEnd:       day.AddDate(0, 0, 1),
		RebalanceInterval: 24 * time.Hour,
		StartingCash:      10000,
		NumTickers:        3,
		AssetUniverse:     universe.AssetUniverseName,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Results)

	for _, result := range resp.Results {
		_, held := result.Portfolio.Positions[unpriced.Symbol]
		require.False(t, held, "backtest must not enter unpriced symbol %s", unpriced.Symbol)
		_, weighted := result.AssetWeights[unpriced.Symbol]
		require.False(t, weighted)
	}
}
