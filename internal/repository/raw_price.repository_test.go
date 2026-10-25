package repository

import (
	"testing"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/util"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestRawPriceRepositoryUpsertManyIdempotent(t *testing.T) {
	db, err := util.NewTestDb()
	require.NoError(t, err)
	require.NoError(t, db.Ping())

	symbol := "TEST.RAW." + uuid.NewString()[:8]
	repo := NewRawPriceRepository(db)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM raw_price WHERE symbol = $1`, symbol)
		_ = db.Close()
	})

	day := time.Date(2024, 6, 7, 0, 0, 0, 0, time.UTC)
	volume := decimal.NewFromInt(1000)
	first := model.RawPrice{
		Symbol:     symbol,
		Date:       day,
		Close:      decimal.RequireFromString("1208.88"),
		Volume:     &volume,
		Source:     "test",
		IngestedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repo.UpsertMany([]model.RawPrice{first}))

	newVolume := decimal.NewFromInt(2000)
	second := first
	second.Close = decimal.RequireFromString("1210.00")
	second.Volume = &newVolume
	second.Source = "test-updated"
	second.IngestedAt = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertMany([]model.RawPrice{second}))

	got, err := repo.ListBySymbolRange(symbol, day.AddDate(0, 0, -1), day.AddDate(0, 0, 1))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, symbol, got[0].Symbol)
	require.True(t, got[0].Close.Equal(decimal.RequireFromString("1210.00")))
	require.NotNil(t, got[0].Volume)
	require.True(t, got[0].Volume.Equal(newVolume))
	require.Equal(t, "test-updated", got[0].Source)
	require.True(t, got[0].IngestedAt.Equal(second.IngestedAt) || got[0].IngestedAt.Equal(second.IngestedAt.UTC()))
}
