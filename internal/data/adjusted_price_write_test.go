package data

import (
	"context"
	"testing"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/util"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	_ "github.com/lib/pq"
)

func TestDerivedPriceReplaceIdempotent(t *testing.T) {
	db, err := util.NewTestDb()
	require.NoError(t, err)
	require.NoError(t, db.Ping())

	symbol := "TEST.ADJ." + uuid.NewString()[:8]
	rawRepo := repository.NewRawPriceRepository(db)
	caRepo := repository.NewCorporateActionRepository(db)
	adjRepo := repository.NewAdjustedPriceRepository(db)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM adjusted_price WHERE symbol = $1`, symbol)
		_, _ = db.Exec(`DELETE FROM raw_price WHERE symbol = $1`, symbol)
		_, _ = db.Exec(`DELETE FROM corporate_action WHERE symbol = $1`, symbol)
		_ = db.Close()
	})

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	require.NoError(t, rawRepo.UpsertMany([]model.RawPrice{
		{Symbol: symbol, Date: day("2024-06-07"), Close: d("1208.88"), Source: "test", IngestedAt: now},
		{Symbol: symbol, Date: day("2024-06-10"), Close: d("121.79"), Source: "test", IngestedAt: now},
		{Symbol: symbol, Date: day("2024-06-11"), Close: d("125.20"), Source: "test", IngestedAt: now},
	}))
	require.NoError(t, caRepo.UpsertMany([]model.CorporateAction{{
		Symbol: symbol, EffectiveDate: day("2024-06-10"), Type: "split", Ratio: dp("10"), Source: "test", IngestedAt: now,
	}}))

	svc := NewDerivedPriceService(db, rawRepo, caRepo, adjRepo, nil, nil)
	ctx := context.Background()
	in := DerivedUpdateInput{Symbols: []string{symbol}, Write: true, Full: true}
	first, err := svc.UpdateDerivedPrices(ctx, in)
	require.NoError(t, err)
	require.Equal(t, 3, first.NumRows)
	require.Equal(t, []string{symbol}, first.FullRecompute)

	second, err := svc.UpdateDerivedPrices(ctx, in)
	require.NoError(t, err)
	require.Equal(t, 3, second.NumRows)

	got, err := adjRepo.List([]string{symbol}, day("2024-06-01"), day("2024-06-30"))
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.True(t, got[0].Price.Equal(d("120.888")), got[0].Price)
	require.True(t, got[1].Price.Equal(d("121.79")))
	require.True(t, got[2].Price.Equal(d("125.20")))
}
