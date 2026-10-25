package data

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMapAlpacaCorporateActions(t *testing.T) {
	body, err := os.ReadFile("testdata/alpaca_corporate_actions.json")
	require.NoError(t, err)

	var page alpacaCAPage
	require.NoError(t, json.Unmarshal(body, &page))

	ingestedAt := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	rows := MapAlpacaCorporateActions(page.CorporateActions, ingestedAt)
	require.Len(t, rows, 6)

	by := map[string]int{}
	for i, row := range rows {
		by[row.Symbol+"|"+row.EffectiveDate.Format("2006-01-02")+"|"+row.Type] = i
	}

	aaplSplit := rows[by["AAPL|2020-08-31|split"]]
	require.True(t, aaplSplit.Ratio.Equal(decimal.RequireFromString("4")))
	require.Equal(t, "forward_split", *aaplSplit.Notes)

	nvda4 := rows[by["NVDA|2021-07-20|split"]]
	require.True(t, nvda4.Ratio.Equal(decimal.RequireFromString("4")))

	nvda10 := rows[by["NVDA|2024-06-10|split"]]
	require.True(t, nvda10.Ratio.Equal(decimal.RequireFromString("10")))

	aaplDiv := rows[by["AAPL|2020-08-07|dividend"]]
	require.True(t, aaplDiv.CashAmount.Equal(decimal.RequireFromString("0.82")))

	ea := rows[by["EA|2026-08-04|acquired"]]
	require.True(t, ea.CashAmount.Equal(decimal.RequireFromString("210")))

	wbd := rows[by["WBD|2026-10-06|acquired"]]
	require.True(t, wbd.CashAmount.Equal(decimal.RequireFromString("31.01666668")))
	require.Equal(t, CorporateActionSourceAlpaca, wbd.Source)
	require.Equal(t, ingestedAt, wbd.IngestedAt)
}

func TestYahooChartSymbol(t *testing.T) {
	require.Equal(t, "BRK-B", yahooChartSymbol("BRK-B"))
	require.Equal(t, "BRK-B", yahooChartSymbol("BRK.B"))
	require.Equal(t, "AAPL", yahooChartSymbol("AAPL"))
}

func TestMapYahooChartEvents(t *testing.T) {
	ingestedAt := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	events := &yahooCAEvents{
		Splits: map[string]struct {
			Date        int64   `json:"date"`
			Numerator   float64 `json:"numerator"`
			Denominator float64 `json:"denominator"`
		}{
			"1598880600": {Date: 1598880600, Numerator: 4, Denominator: 1},
		},
		Dividends: map[string]struct {
			Amount float64 `json:"amount"`
			Date   int64   `json:"date"`
		}{
			"1596807000": {Amount: 0.205, Date: 1596807000},
		},
	}

	rows := MapYahooChartEvents("AAPL", events, ingestedAt)
	require.Len(t, rows, 2)

	by := map[string]model.CorporateAction{}
	for _, row := range rows {
		by[row.Type] = row
	}
	require.True(t, by["split"].Ratio.Equal(decimal.RequireFromString("4")))
	require.True(t, by["split"].EffectiveDate.Equal(time.Date(2020, 8, 31, 0, 0, 0, 0, time.UTC)))
	require.Equal(t, CorporateActionSourceYahoo, by["split"].Source)
	require.True(t, by["dividend"].CashAmount.Equal(decimal.RequireFromString("0.205")))
	require.True(t, by["dividend"].EffectiveDate.Equal(time.Date(2020, 8, 7, 0, 0, 0, 0, time.UTC)))
}

func TestCoalesceCorporateActions(t *testing.T) {
	day := time.Date(2012, 12, 12, 0, 0, 0, 0, time.UTC)
	a := decimal.RequireFromString("0.10")
	b := decimal.RequireFromString("0.25")
	ratio := decimal.RequireFromString("2")
	rows := []model.CorporateAction{
		{Symbol: "KO", EffectiveDate: day, Type: "dividend", CashAmount: &a, Source: CorporateActionSourceYahoo},
		{Symbol: "KO", EffectiveDate: day, Type: "dividend", CashAmount: &b, Source: CorporateActionSourceYahoo},
		{Symbol: "KO", EffectiveDate: day, Type: "split", Ratio: &ratio},
	}
	got := coalesceCorporateActions(rows)
	require.Len(t, got, 2)
	require.Equal(t, "dividend", got[0].Type)
	require.True(t, got[0].CashAmount.Equal(decimal.RequireFromString("0.35")))
	require.Equal(t, "split", got[1].Type)
}
