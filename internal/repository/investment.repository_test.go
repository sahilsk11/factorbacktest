package repository

import (
	"errors"
	"factorbacktest/internal/testseed"
	"factorbacktest/internal/util"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInvestmentLiquidationLifecycle(t *testing.T) {
	db, err := util.NewTestDb()
	require.NoError(t, err)
	repository := NewInvestmentRepository(db)
	user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: uuid.NewString() + "@example.com"})
	universeName := "LIQUIDATION_" + uuid.NewString()
	testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: universeName})
	strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              "liquidation-test",
		UserAccountID:     user.UserAccountID,
		AssetUniverse:     universeName,
		NumAssets:         3,
		RebalanceInterval: "DAILY",
		FactorExpression:  "price()",
	})
	investment := testseed.CreateInvestment(db, testseed.InvestmentOpts{
		StrategyID:    strategy.StrategyID,
		UserAccountID: user.UserAccountID,
		AmountDollars: 100,
	})

	requested, err := repository.RequestLiquidation(investment.InvestmentID, user.UserAccountID)
	require.NoError(t, err)
	require.NotNil(t, requested.LiquidationRequestedAt)
	firstRequestedAt := *requested.LiquidationRequestedAt

	requestedAgain, err := repository.RequestLiquidation(investment.InvestmentID, user.UserAccountID)
	require.NoError(t, err)
	require.Equal(t, firstRequestedAt, *requestedAgain.LiquidationRequestedAt)

	_, err = repository.RequestLiquidation(investment.InvestmentID, uuid.New())
	require.True(t, errors.Is(err, ErrInvestmentNotFound))

	_, err = db.Exec(`UPDATE investment SET paused_at = now() WHERE investment_id = $1`, investment.InvestmentID)
	require.NoError(t, err)
	active, err := repository.List(StrategyInvestmentListFilter{UserAccountIDs: []uuid.UUID{user.UserAccountID}})
	require.NoError(t, err)
	require.Len(t, active, 1, "a paused investment must remain eligible while liquidation is pending")

	completed, err := repository.CompleteLiquidation(nil, investment.InvestmentID)
	require.NoError(t, err)
	require.True(t, completed)
	ended, err := repository.Get(investment.InvestmentID)
	require.NoError(t, err)
	require.NotNil(t, ended.EndDate)
}
