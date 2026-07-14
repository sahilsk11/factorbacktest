package data

import (
	"context"
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	mock_repository "factorbacktest/internal/repository/mocks"
	"factorbacktest/internal/util"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// import (
// 	"database/sql"
// 	"factorbacktest/internal/domain"
// 	mock_repository "factorbacktest/internal/repository/mocks"
// 	"testing"
// 	"time"

// 	"go.uber.org/mock/mockgen/gomock"
// 	"github.com/google/go-cmp/cmp"
// 	"github.com/stretchr/testify/require"
// )

// func Test_priceServiceHandler_LoadCache(t *testing.T) {
// 	t.Run("load cache", func(t *testing.T) {
// 		ctrl := gomock.NewController(t)
// 		adjPriceRepository := mock_repository.NewMockAdjustedPriceRepository(ctrl)

// 		h := priceServiceHandler{
// 			AdjPriceRepository: adjPriceRepository,
// 		}

// 		tx := &sql.Tx{}
// 		symbols := []string{}
// 		start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
// 		end := time.Now()

// 		adjPriceRepository.EXPECT().ListTradingDays(
// 			tx,
// 			start,
// 			end,
// 		).Return([]time.Time{}, nil)

// 		adjPriceRepository.EXPECT().List(
// 			tx,
// 			symbols,
// 			start,
// 			end,
// 		).Return([]domain.AssetPrice{
// 			{
// 				Symbol: "AAPL",
// 				Price:  1,
// 				Date:   start,
// 			},
// 			{
// 				Symbol: "AAPL",
// 				Price:  2,
// 				Date:   start.AddDate(1, 0, 0),
// 			},
// 		}, nil)

// 		cache, err := h.LoadCache(tx, symbols, start, end)
// 		require.NoError(t, err)

// 		// trading days
// 		require.Equal(
// 			t,
// 			"",
// 			cmp.Diff(
// 				[]time.Time{},
// 				cache.tradingDays,
// 			),
// 		)

// 		// cache
// 		require.Equal(
// 			t,
// 			"",
// 			cmp.Diff(
// 				map[string]map[time.Time]float64{
// 					"AAPL": {
// 						start:                  1,
// 						start.AddDate(1, 0, 0): 2,
// 					},
// 				},
// 				cache.cache,
// 			),
// 		)
// 	})
// }

// func TestPriceCache_Get(t *testing.T) {
// 	t.Run("cache contains value", func(t *testing.T) {
// 		date1 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
// 		date2 := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
// 		date3 := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)

// 		pr := PriceCache{
// 			cache: map[string]map[time.Time]float64{
// 				"AAPL": {
// 					date1: 1,
// 					date2: 2,
// 					date3: 3,
// 				},
// 			},
// 			tradingDays: []time.Time{
// 				date1, date2, date3,
// 			},
// 		}

// 		price, err := pr.Get("AAPL", date1)
// 		require.NoError(t, err)

// 		require.Equal(t, float64(1), price)
// 	})
// }

func Test_fillPriceCacheGaps(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		cache := map[string]map[string]float64{
			"AAPL": {
				"2020-01-02": 100,
			},
		}

		inputs := []LoadPriceCacheInput{
			{
				Date:   time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC),
				Symbol: "AAPL",
			},
		}
		fillPriceCacheGaps(inputs, cache)

		require.Equal(t, map[string]map[string]float64{
			"AAPL": {
				"2020-01-02": 100,
				"2020-01-03": 100,
			},
		}, cache)
	})

	t.Run("value does not exist", func(t *testing.T) {
		cache := map[string]map[string]float64{
			"AAPL": {
				"2020-01-02": 100,
			},
		}

		inputs := []LoadPriceCacheInput{
			{
				Date:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
				Symbol: "AAPL",
			},
		}
		fillPriceCacheGaps(inputs, cache)

		require.Equal(t, map[string]map[string]float64{
			"AAPL": {
				"2020-01-02": 100,
			},
		}, cache)
	})
}

func Test_constructMinMaxMap(t *testing.T) {
	t.Run("only price inputs", func(t *testing.T) {
		t1 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		t2 := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
		inputs := []LoadPriceCacheInput{
			{
				Date:   t1,
				Symbol: "AAPL",
			},
			{
				Date:   t2,
				Symbol: "AAPL",
			},
		}
		stdevInputs := []LoadStdevCacheInput{}

		min, max, mp := constructMinMaxMap(inputs, stdevInputs)

		require.NotNil(t, min)
		require.NotNil(t, max)
		require.Equal(t, t1, *min)
		require.Equal(t, t2, *max)
		require.Equal(t, map[string]*minMax{
			"AAPL": {
				min: &t1,
				max: &t2,
			},
		}, mp)
	})
}

func Test_priceServiceHandler_GetLatestPrices(t *testing.T) {

	t.Run("test", func(t *testing.T) {
		t.Skip("Skipping test in GitHub Actions")
		ctx := context.Background()
		h := priceServiceHandler{
			// AdjPriceRepository: adjPriceRepository,
		}
		resp, err := h.GetLatestPrices(ctx, []string{"BRK.B"})
		require.NoError(t, err)
		util.Pprint(resp)
		require.Equal(t, map[string]decimal.Decimal{
			"AAPL": decimal.NewFromFloat(100),
		}, resp)
	})
}

func TestPriceServiceUpdatePricesCommitsSuccessfulSymbolsIndependently(t *testing.T) {
	ctrl := gomock.NewController(t)
	prices := mock_repository.NewMockAdjustedPriceRepository(ctrl)
	latestDate := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	prices.EXPECT().LatestPriceDates([]string{"AAPL", "BAD"}).Return(map[string]time.Time{
		"AAPL": latestDate,
	}, nil)
	prices.EXPECT().Add(nil, gomock.Any()).DoAndReturn(func(_ *sql.Tx, models []model.AdjustedPrice) error {
		require.Len(t, models, 1)
		require.Equal(t, "AAPL", models[0].Symbol)
		return nil
	})

	provider := &recordingQuoteProvider{
		points: map[string][]DailyPricePoint{
			"AAPL": {{Date: latestDate.AddDate(0, 0, 1), Price: decimal.NewFromInt(200)}},
		},
		errors: map[string]error{
			"BAD": fmt.Errorf("symbol not found"),
		},
	}
	service := priceServiceHandler{QuoteProvider: provider}

	result, err := service.UpdatePrices(context.Background(), []string{"AAPL", "BAD", "AAPL", repository.CASH_SYMBOL}, prices)

	require.NoError(t, err)
	require.Equal(t, []string{"AAPL"}, result.UpdatedSymbols)
	require.Equal(t, []string{"BAD"}, result.FailedSymbols)
	require.Equal(t, latestDate.Add(-priceRefreshOverlap), provider.startFor("AAPL"))
}

type recordingQuoteProvider struct {
	points map[string][]DailyPricePoint
	errors map[string]error

	mu     sync.Mutex
	starts map[string]time.Time
}

func (p *recordingQuoteProvider) ProviderName() string { return "test" }

func (p *recordingQuoteProvider) GetLatestQuotes(context.Context, []string) (*QuoteResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (p *recordingQuoteProvider) GetDailyAdjCloses(_ context.Context, symbol string, start, _ time.Time) ([]DailyPricePoint, error) {
	p.mu.Lock()
	if p.starts == nil {
		p.starts = map[string]time.Time{}
	}
	p.starts[symbol] = start
	p.mu.Unlock()
	if err := p.errors[symbol]; err != nil {
		return nil, err
	}
	return p.points[symbol], nil
}

func (p *recordingQuoteProvider) startFor(symbol string) time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.starts[symbol]
}
