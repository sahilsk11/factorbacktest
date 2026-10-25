package data

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapRawBarsToRows(t *testing.T) {
	ingestedAt := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	volume := decimal.NewFromInt(12_345_678)
	bars := []RawDailyBar{
		{
			Date:   time.Date(2024, 6, 7, 13, 30, 0, 0, time.UTC),
			Close:  decimal.RequireFromString("1208.88"),
			Volume: &volume,
		},
		{
			Date:  time.Date(2024, 6, 10, 20, 0, 0, 0, time.UTC),
			Close: decimal.Zero,
		},
		{
			Date:  time.Date(2024, 6, 11, 0, 0, 0, 0, time.UTC),
			Close: decimal.RequireFromString("-1"),
		},
		{
			Date:  time.Date(2024, 6, 12, 4, 0, 0, 0, time.FixedZone("ET", -4*3600)),
			Close: decimal.RequireFromString("125.20"),
		},
	}

	rows := MapRawBarsToRows("NVDA", RawPriceSourceAlpaca, bars, ingestedAt)
	require.Len(t, rows, 2)

	require.Equal(t, "NVDA", rows[0].Symbol)
	require.True(t, rows[0].Date.Equal(time.Date(2024, 6, 7, 0, 0, 0, 0, time.UTC)))
	require.True(t, rows[0].Close.Equal(decimal.RequireFromString("1208.88")))
	require.NotNil(t, rows[0].Volume)
	require.True(t, rows[0].Volume.Equal(volume))
	require.Equal(t, RawPriceSourceAlpaca, rows[0].Source)
	require.Equal(t, ingestedAt, rows[0].IngestedAt)

	require.Equal(t, "NVDA", rows[1].Symbol)
	require.True(t, rows[1].Date.Equal(time.Date(2024, 6, 12, 0, 0, 0, 0, time.UTC)))
	require.True(t, rows[1].Close.Equal(decimal.RequireFromString("125.20")))
	require.Nil(t, rows[1].Volume)
}

func TestApplyYahooSplitFactors(t *testing.T) {
	preVolume := decimal.NewFromInt(4_000_000)
	postVolume := decimal.NewFromInt(1_000_000)
	bars := []RawDailyBar{
		{Date: time.Date(2020, 8, 28, 13, 30, 0, 0, time.UTC), Close: decimal.RequireFromString("124.81"), Volume: &preVolume},
		{Date: time.Date(2020, 8, 31, 13, 30, 0, 0, time.UTC), Close: decimal.RequireFromString("129.04"), Volume: &postVolume},
	}
	got := applyYahooSplitFactors(bars, []yahooSplit{{
		Date:        time.Date(2020, 8, 31, 0, 0, 0, 0, time.UTC),
		Numerator:   decimal.NewFromInt(4),
		Denominator: decimal.NewFromInt(1),
	}})
	require.Len(t, got, 2)
	require.True(t, got[0].Close.Equal(decimal.RequireFromString("499.24")))
	require.NotNil(t, got[0].Volume)
	require.True(t, got[0].Volume.Equal(decimal.NewFromInt(1_000_000)))
	require.True(t, got[1].Close.Equal(decimal.RequireFromString("129.04")))
	require.NotNil(t, got[1].Volume)
	require.True(t, got[1].Volume.Equal(postVolume))
}
