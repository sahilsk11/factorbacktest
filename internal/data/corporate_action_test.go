package data

import (
	"encoding/json"
	"os"
	"testing"
	"time"

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
