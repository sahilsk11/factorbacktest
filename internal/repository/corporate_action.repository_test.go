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

func TestCorporateActionRepositoryUpsertManyIdempotent(t *testing.T) {
	db, err := util.NewTestDb()
	require.NoError(t, err)
	require.NoError(t, db.Ping())

	symbol := "TEST.CA." + uuid.NewString()[:8]
	repo := NewCorporateActionRepository(db)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM corporate_action WHERE symbol = $1`, symbol)
		_ = db.Close()
	})

	day := time.Date(2024, 6, 10, 0, 0, 0, 0, time.UTC)
	ratio := decimal.RequireFromString("10")
	first := model.CorporateAction{
		Symbol:        symbol,
		EffectiveDate: day,
		Type:          "split",
		Ratio:         &ratio,
		Source:        "test",
		IngestedAt:    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repo.UpsertMany([]model.CorporateAction{first}))

	newRatio := decimal.RequireFromString("10.0")
	note := "forward_split"
	second := first
	second.Ratio = &newRatio
	second.Notes = &note
	second.Source = "test-updated"
	second.IngestedAt = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpsertMany([]model.CorporateAction{second}))

	got, err := repo.ListBySymbolRange(symbol, day.AddDate(0, 0, -1), day.AddDate(0, 0, 1))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, symbol, got[0].Symbol)
	require.Equal(t, "split", got[0].Type)
	require.True(t, got[0].Ratio.Equal(decimal.RequireFromString("10")))
	require.Equal(t, "test-updated", got[0].Source)
	require.Equal(t, note, *got[0].Notes)
}
