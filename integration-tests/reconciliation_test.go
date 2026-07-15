package integration_tests

import (
	"factorbacktest/internal/repository"
	"factorbacktest/internal/testseed"
	"net/http"
	"testing"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestReconciliationPreviewAndApply(t *testing.T) {
	t.Setenv("CRON_SECRET", "test-cron-secret")
	manager, err := NewTestDbManager()
	require.NoError(t, err)
	defer manager.Close()
	db := manager.DB()

	aapl := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AAPL", Name: "Apple"})
	user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: "reconciliation@example.com"})
	universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: "RECONCILIATION_TEST"})
	testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, aapl.TickerID)
	strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name: "reconciliation strategy", UserAccountID: user.UserAccountID,
		AssetUniverse: universe.AssetUniverseName, NumAssets: 1,
		RebalanceInterval: "DAILY", FactorExpression: "price()",
	})

	investments := make([]string, 0, 2)
	for range 2 {
		investment := testseed.CreateInvestment(db, testseed.InvestmentOpts{
			StrategyID: strategy.StrategyID, UserAccountID: user.UserAccountID, AmountDollars: 100,
		})
		version := testseed.CreateInvestmentHoldingsVersion(db, investment.InvestmentID)
		testseed.CreateInvestmentHolding(db, testseed.InvestmentHoldingOpts{
			VersionID: version.InvestmentHoldingsVersionID, TickerID: aapl.TickerID,
			Quantity: decimal.NewFromInt(4),
		})
		investments = append(investments, investment.InvestmentID.String())
	}

	broker := newLiquidationTestBroker([]alpaca.Position{{
		Symbol: "AAPL", Qty: decimal.NewFromInt(6), QtyAvailable: decimal.NewFromInt(6),
	}})
	server, err := NewTestServerWithDependencies(manager, broker, nil)
	require.NoError(t, err)
	defer server.Stop()

	var preview struct {
		ReconciliationRunID string `json:"reconciliationRunID"`
		Status              string `json:"status"`
		ProposedAdjustments []struct {
			ToQuantity string `json:"toQuantity"`
		} `json:"proposedAdjustments"`
	}
	require.NoError(t, hitEndpoint(server.URL, "internal/reconciliation/preview", http.MethodPost, map[string]string{}, &preview))
	require.Equal(t, "MISMATCH", preview.Status)
	require.Len(t, preview.ProposedAdjustments, 2)
	for _, adjustment := range preview.ProposedAdjustments {
		require.Equal(t, "3", adjustment.ToQuantity)
	}

	require.NoError(t, hitEndpoint(server.URL, "internal/reconciliation/"+preview.ReconciliationRunID+"/apply", http.MethodPost, map[string]string{}, &map[string]bool{}))
	holdingsRepository := repository.NewInvestmentHoldingsRepository(db)
	for _, investmentID := range investments {
		id, err := uuid.Parse(investmentID)
		require.NoError(t, err)
		holdings, err := holdingsRepository.GetLatestHoldings(nil, id)
		require.NoError(t, err)
		require.True(t, holdings.Positions["AAPL"].ExactQuantity.Equal(decimal.NewFromInt(3)))
	}
	require.Empty(t, broker.placedRequests(), "reconciliation must not place broker orders")
}
