package data

import (
	"testing"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func dp(s string) *decimal.Decimal {
	v := decimal.RequireFromString(s)
	return &v
}
func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
func rawBar(date, close string) model.RawPrice {
	return model.RawPrice{Date: day(date), Close: d(close)}
}

func TestComputeAdjustedSeries(t *testing.T) {
	t.Run("nvda 10:1 split and latest factor 1", func(t *testing.T) {
		points, warnings := ComputeAdjustedSeries(
			[]model.RawPrice{
				rawBar("2024-06-07", "1208.88"),
				rawBar("2024-06-10", "121.79"),
			},
			[]model.CorporateAction{{
				Symbol: "NVDA", Type: "split", EffectiveDate: day("2024-06-10"), Ratio: dp("10"),
			}},
		)
		require.Empty(t, warnings)
		require.Len(t, points, 2)
		require.True(t, points[0].Factor.Equal(d("0.1")), points[0].Factor)
		require.True(t, points[0].Price.Equal(d("120.888")), points[0].Price)
		require.True(t, points[1].Factor.Equal(d("1")))
		require.True(t, points[1].Price.Equal(d("121.79")))
	})

	t.Run("aapl split and dividend", func(t *testing.T) {
		points, warnings := ComputeAdjustedSeries(
			[]model.RawPrice{
				rawBar("2020-08-06", "455.00"),
				rawBar("2020-08-07", "444.45"),
				rawBar("2020-08-28", "499.23"),
				rawBar("2020-08-31", "129.04"),
			},
			[]model.CorporateAction{
				{Symbol: "AAPL", Type: "dividend", EffectiveDate: day("2020-08-07"), CashAmount: dp("0.82")},
				{Symbol: "AAPL", Type: "split", EffectiveDate: day("2020-08-31"), Ratio: dp("4")},
			},
		)
		require.Empty(t, warnings)
		require.Len(t, points, 4)
		require.True(t, points[3].Factor.Equal(d("1")))
		require.True(t, points[2].Factor.Equal(d("0.25")))
		require.True(t, points[2].Price.Equal(d("124.8075")))
		require.True(t, points[1].Factor.Equal(d("0.25")))
		divFactor := d("1").Sub(d("0.82").Div(d("455.00")))
		require.True(t, points[0].Factor.Equal(divFactor.Mul(d("0.25"))))
	})

	t.Run("same-day split and dividend, missing prior, acquired", func(t *testing.T) {
		points, warnings := ComputeAdjustedSeries(
			[]model.RawPrice{
				rawBar("2024-01-02", "100"),
				rawBar("2024-01-03", "50"),
				rawBar("2024-06-03", "210"),
				rawBar("2024-06-04", "211"),
			},
			[]model.CorporateAction{
				{Symbol: "X", Type: "split", EffectiveDate: day("2024-01-03"), Ratio: dp("2")},
				{Symbol: "X", Type: "dividend", EffectiveDate: day("2024-01-03"), CashAmount: dp("2")},
				{Symbol: "X", Type: "dividend", EffectiveDate: day("2023-12-01"), CashAmount: dp("1")},
				{Symbol: "X", Type: "acquired", EffectiveDate: day("2024-06-04"), CashAmount: dp("210")},
			},
		)
		require.Len(t, warnings, 1)
		require.Contains(t, warnings[0], "no prior raw close")
		require.Len(t, points, 3)
		require.Equal(t, "2024-06-03", points[2].Date.Format(time.DateOnly))
		require.True(t, points[2].Factor.Equal(d("1")))
		require.True(t, points[1].Factor.Equal(d("1")))
		require.True(t, points[0].Factor.Equal(d("0.49")), points[0].Factor)
		require.True(t, points[0].Price.Equal(d("49")))
	})
}
