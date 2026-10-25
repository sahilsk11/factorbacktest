package data

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/domain"
	mock_repository "factorbacktest/internal/repository/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAdjPricesMatch(t *testing.T) {
	stored := decimal.NewFromInt(100)
	require.True(t, adjPricesMatch(stored, decimal.RequireFromString("100.01")))  // 0.01%
	require.False(t, adjPricesMatch(stored, decimal.RequireFromString("100.02"))) // 0.02%
}

func TestUpdateAdjustedPricesScenarios(t *testing.T) {
	day := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	prev := day.AddDate(0, 0, -1)
	yahooRecent := []DailyPricePoint{
		{Date: prev, Price: decimal.NewFromInt(100)},
		{Date: day, Price: decimal.NewFromInt(101)},
	}

	t.Run("match appends only new days", func(t *testing.T) {
		svc, prices, cas, ingest, fetch := newAdjPriceHarness(t, map[string][]DailyPricePoint{"AAPL": yahooRecent})
		prices.EXPECT().LatestPriceDates([]string{"AAPL"}).Return(map[string]time.Time{"AAPL": prev}, nil)
		prices.EXPECT().List([]string{"AAPL"}, gomock.Any(), gomock.Any()).Return([]domain.AssetPrice{
			{Symbol: "AAPL", Date: prev, Price: decimal.NewFromInt(100)},
		}, nil)
		cas.EXPECT().ListBySymbolRange(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		var added []model.AdjustedPrice
		prices.EXPECT().Add(nil, gomock.Any()).DoAndReturn(func(_ *sql.Tx, rows []model.AdjustedPrice) error {
			added = append(added, rows...)
			return nil
		})

		result, err := svc.UpdateAdjustedPrices(context.Background(), []string{"AAPL"}, false)
		require.NoError(t, err)
		require.Equal(t, 1, result.Appended)
		require.Zero(t, result.FullRepulled)
		require.Empty(t, result.UnexplainedChanges)
		require.Empty(t, result.FailedSymbols)
		require.True(t, ingest.called)
		require.Len(t, added, 1)
		require.Equal(t, "AAPL", added[0].Symbol)
		require.Equal(t, day, added[0].Date)
		require.Equal(t, 1, fetch.calls["AAPL"])
	})

	t.Run("mismatch with recent action full-repulls", func(t *testing.T) {
		full := append(append([]DailyPricePoint{}, yahooRecent...), DailyPricePoint{
			Date: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), Price: decimal.NewFromInt(10),
		})
		svc, prices, cas, _, fetch := newAdjPriceHarness(t, map[string][]DailyPricePoint{"NVDA": full})
		prices.EXPECT().LatestPriceDates([]string{"NVDA"}).Return(map[string]time.Time{"NVDA": prev}, nil)
		prices.EXPECT().List([]string{"NVDA"}, gomock.Any(), gomock.Any()).Return([]domain.AssetPrice{
			{Symbol: "NVDA", Date: prev, Price: decimal.NewFromInt(200)},
			{Symbol: "NVDA", Date: day, Price: decimal.NewFromInt(202)},
		}, nil)
		cas.EXPECT().ListBySymbolRange("NVDA", gomock.Any(), gomock.Any()).Return([]model.CorporateAction{{
			Symbol: "NVDA", Type: "split", EffectiveDate: day,
		}}, nil)
		var added int
		prices.EXPECT().Add(nil, gomock.Any()).DoAndReturn(func(_ *sql.Tx, rows []model.AdjustedPrice) error {
			added += len(rows)
			return nil
		})

		result, err := svc.UpdateAdjustedPrices(context.Background(), []string{"NVDA"}, false)
		require.NoError(t, err)
		require.Zero(t, result.Appended)
		require.Equal(t, 1, result.FullRepulled)
		require.Empty(t, result.UnexplainedChanges)
		require.Equal(t, 3, added)
		require.GreaterOrEqual(t, fetch.calls["NVDA"], 2)
	})

	t.Run("mismatch without action is unexplained and not written", func(t *testing.T) {
		svc, prices, cas, _, fetch := newAdjPriceHarness(t, map[string][]DailyPricePoint{"KO": yahooRecent})
		prices.EXPECT().LatestPriceDates([]string{"KO"}).Return(map[string]time.Time{"KO": prev}, nil)
		prices.EXPECT().List([]string{"KO"}, gomock.Any(), gomock.Any()).Return([]domain.AssetPrice{
			{Symbol: "KO", Date: prev, Price: decimal.NewFromInt(50)},
			{Symbol: "KO", Date: day, Price: decimal.NewFromInt(51)},
		}, nil)
		cas.EXPECT().ListBySymbolRange("KO", gomock.Any(), gomock.Any()).Return(nil, nil)
		prices.EXPECT().Add(gomock.Any(), gomock.Any()).Times(0)

		result, err := svc.UpdateAdjustedPrices(context.Background(), []string{"KO"}, false)
		require.NoError(t, err)
		require.Zero(t, result.Appended)
		require.Zero(t, result.FullRepulled)
		require.Len(t, result.UnexplainedChanges, 1)
		require.Equal(t, "KO", result.UnexplainedChanges[0].Symbol)
		require.Equal(t, prev.Format(time.DateOnly), result.UnexplainedChanges[0].Date)
		require.Equal(t, 1, fetch.calls["KO"])
	})

	t.Run("empty fetch is failed and not written", func(t *testing.T) {
		svc, prices, _, _, _ := newAdjPriceHarness(t, map[string][]DailyPricePoint{})
		prices.EXPECT().LatestPriceDates([]string{"ZZZ"}).Return(map[string]time.Time{"ZZZ": prev}, nil)
		prices.EXPECT().Add(gomock.Any(), gomock.Any()).Times(0)

		result, err := svc.UpdateAdjustedPrices(context.Background(), []string{"ZZZ"}, false)
		require.NoError(t, err)
		require.Equal(t, []string{"ZZZ"}, result.FailedSymbols)
		require.Zero(t, result.Appended)
		require.Zero(t, result.FullRepulled)
		require.Empty(t, result.UnexplainedChanges)
	})

	t.Run("missing history full-repulls", func(t *testing.T) {
		svc, prices, cas, _, fetch := newAdjPriceHarness(t, map[string][]DailyPricePoint{"F": yahooRecent})
		prices.EXPECT().LatestPriceDates([]string{"F"}).Return(map[string]time.Time{}, nil)
		cas.EXPECT().ListBySymbolRange(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
		prices.EXPECT().Add(nil, gomock.Any()).Return(nil)

		result, err := svc.UpdateAdjustedPrices(context.Background(), []string{"F"}, false)
		require.NoError(t, err)
		require.Equal(t, 1, result.FullRepulled)
		require.Zero(t, result.Appended)
		require.Equal(t, 1, fetch.calls["F"])
	})

	t.Run("dryRun reports without writing or ingesting", func(t *testing.T) {
		svc, prices, _, ingest, _ := newAdjPriceHarness(t, map[string][]DailyPricePoint{"AAPL": yahooRecent})
		prices.EXPECT().LatestPriceDates([]string{"AAPL"}).Return(map[string]time.Time{}, nil)
		prices.EXPECT().Add(gomock.Any(), gomock.Any()).Times(0)

		result, err := svc.UpdateAdjustedPrices(context.Background(), []string{"AAPL"}, true)
		require.NoError(t, err)
		require.Equal(t, 1, result.FullRepulled)
		require.False(t, ingest.called)
	})
}

type stubCAIngest struct {
	called bool
}

func (s *stubCAIngest) IngestCorporateActions(context.Context, []string, *time.Time, *time.Time, string) (CorporateActionIngestResult, error) {
	s.called = true
	return CorporateActionIngestResult{Source: CorporateActionSourceYahoo}, nil
}

type countingFetch struct {
	mu     sync.Mutex
	points map[string][]DailyPricePoint
	calls  map[string]int
}

func (f *countingFetch) get(_ context.Context, symbol string, _, _ time.Time) ([]DailyPricePoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[symbol]++
	pts := f.points[symbol]
	if len(pts) == 0 {
		return nil, fmt.Errorf("empty")
	}
	return pts, nil
}

func newAdjPriceHarness(t *testing.T, points map[string][]DailyPricePoint) (*adjustedPriceService, *mock_repository.MockAdjustedPriceRepository, *mock_repository.MockCorporateActionRepository, *stubCAIngest, *countingFetch) {
	t.Helper()
	ctrl := gomock.NewController(t)
	prices := mock_repository.NewMockAdjustedPriceRepository(ctrl)
	cas := mock_repository.NewMockCorporateActionRepository(ctrl)
	ingest := &stubCAIngest{}
	fetch := &countingFetch{points: points, calls: map[string]int{}}
	svc := &adjustedPriceService{
		prices: prices,
		cas:    cas,
		ingest: ingest,
		fetch:  fetch.get,
	}
	return svc, prices, cas, ingest, fetch
}
