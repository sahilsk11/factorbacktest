package integration_tests

import (
	"factorbacktest/internal/repository"
	"factorbacktest/internal/testseed"
	"net/http"
	"testing"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestLiquidationWithBrokerLedgerMismatchEntersErrorState(t *testing.T) {
	t.Setenv("CRON_SECRET", "test-cron-secret")
	manager, err := NewTestDbManager()
	require.NoError(t, err)
	defer manager.Close()
	db := manager.DB()

	intc := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "INTC", Name: "Intel"})
	cash := testseed.LookupTickerBySymbol(db, ":CASH")
	user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: "intc-mismatch@example.com"})
	universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: "INTC_MISMATCH_TEST"})
	testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, intc.TickerID)
	strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name: "intc mismatch strategy", UserAccountID: user.UserAccountID,
		AssetUniverse: universe.AssetUniverseName, NumAssets: 1,
		RebalanceInterval: "DAILY", FactorExpression: "price()",
	})
	investment := testseed.CreateInvestment(db, testseed.InvestmentOpts{
		StrategyID: strategy.StrategyID, UserAccountID: user.UserAccountID, AmountDollars: 99,
	})
	version := testseed.CreateInvestmentHoldingsVersion(db, investment.InvestmentID)
	testseed.CreateInvestmentHolding(db, testseed.InvestmentHoldingOpts{
		VersionID: version.InvestmentHoldingsVersionID, TickerID: cash.TickerID, Quantity: decimal.NewFromFloat(1.22),
	})
	testseed.CreateInvestmentHolding(db, testseed.InvestmentHoldingOpts{
		VersionID: version.InvestmentHoldingsVersionID, TickerID: intc.TickerID, Quantity: decimal.RequireFromString("1.19749006"),
	})

	broker := newLiquidationTestBroker([]alpaca.Position{{
		Symbol: "INTC", Qty: decimal.RequireFromString("0.764168329"), QtyAvailable: decimal.RequireFromString("0.764168329"),
	}})
	server, err := NewTestServerWithDependencies(manager, broker, func(c *gin.Context) {
		if userAccountID := c.GetHeader("X-Test-User-Account-ID"); userAccountID != "" {
			c.Set("userAccountID", userAccountID)
		}
		c.Next()
	})
	require.NoError(t, err)
	defer server.Stop()

	hitAuthenticatedEndpoint(t, server.URL, "/investments/"+investment.InvestmentID.String()+"/request-liquidation", http.MethodPost, user.UserAccountID, http.StatusAccepted, &map[string]bool{})
	require.NoError(t, hitEndpoint(server.URL, "internal/cron/rebalance", http.MethodPost, map[string]string{}, &map[string]string{}))

	require.Empty(t, broker.placedRequests(), "must not place broker orders when ledger exceeds broker inventory")

	investmentRepository := repository.NewInvestmentRepository(db)
	errored, err := investmentRepository.Get(investment.InvestmentID)
	require.NoError(t, err)
	require.NotNil(t, errored.ErrorAt, "investment must enter error state for ops review")
	require.NotNil(t, errored.ErrorReason)

	holdingsRepository := repository.NewInvestmentHoldingsRepository(db)
	latest, err := holdingsRepository.GetLatestHoldings(nil, investment.InvestmentID)
	require.NoError(t, err)
	require.True(t, latest.Positions["INTC"].ExactQuantity.Equal(decimal.RequireFromString("1.19749006")),
		"ledger unchanged until reconciliation apply")

	require.NoError(t, hitEndpoint(server.URL, "internal/cron/rebalance", http.MethodPost, map[string]string{}, &map[string]string{}))
	require.Empty(t, broker.placedRequests(), "cron must skip investments already in error state")
}
