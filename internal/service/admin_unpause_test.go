package service

import (
	"context"
	"database/sql"
	"testing"

	"factorbacktest/internal/repository"
	"factorbacktest/internal/testseed"
	"factorbacktest/internal/util"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAdminUnpauseMakesInvestmentRebalanceEligible(t *testing.T) {
	db, err := util.NewTestDb()
	require.NoError(t, err)
	repo := repository.NewInvestmentRepository(db)
	handler := investmentServiceHandler{InvestmentRepository: repo}

	user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: uuid.NewString() + "@example.com"})
	universeName := "ADMIN_UNPAUSE_" + uuid.NewString()
	testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: universeName})
	strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              "admin-unpause-strategy",
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
	_, err = db.Exec(`UPDATE investment SET paused_at = now() WHERE investment_id = $1`, investment.InvestmentID)
	require.NoError(t, err)

	tradeOrdersBefore := countTable(t, db, "trade_order")
	investmentTradesBefore := countTable(t, db, "investment_trade")

	listed, err := repo.List(repository.StrategyInvestmentListFilter{UserAccountIDs: []uuid.UUID{user.UserAccountID}})
	require.NoError(t, err)
	require.Empty(t, listed, "paused investments must be excluded from rebalance candidate listing")

	result, err := handler.AdminUnpause(context.Background(), investment.InvestmentID)
	require.NoError(t, err)
	require.Equal(t, investment.InvestmentID, result.InvestmentID)
	require.False(t, result.Paused)
	require.False(t, result.AlreadyActive)

	listed, err = repo.List(repository.StrategyInvestmentListFilter{UserAccountIDs: []uuid.UUID{user.UserAccountID}})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, investment.InvestmentID, listed[0].InvestmentID)
	require.Nil(t, listed[0].PausedAt)

	require.Equal(t, tradeOrdersBefore, countTable(t, db, "trade_order"))
	require.Equal(t, investmentTradesBefore, countTable(t, db, "investment_trade"))

	again, err := handler.AdminUnpause(context.Background(), investment.InvestmentID)
	require.NoError(t, err)
	require.True(t, again.AlreadyActive)
	require.False(t, again.Paused)
}

func countTable(t *testing.T, db *sql.DB, tableName string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM "+tableName).Scan(&n))
	return n
}
