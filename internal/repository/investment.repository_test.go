package repository

import (
	"database/sql"
	"errors"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/testseed"
	"factorbacktest/internal/util"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func seedTestInvestment(t *testing.T, namePrefix string) (*sql.DB, InvestmentRepository, model.UserAccount, model.Investment) {
	t.Helper()
	db, err := util.NewTestDb()
	require.NoError(t, err)
	repo := NewInvestmentRepository(db)
	user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: uuid.NewString() + "@example.com"})
	universeName := namePrefix + "_" + uuid.NewString()
	testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: universeName})
	strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              namePrefix + "-strategy",
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
	return db, repo, user, investment
}

func pauseInvestment(t *testing.T, db *sql.DB, investmentID uuid.UUID) {
	t.Helper()
	_, err := db.Exec(`UPDATE investment SET paused_at = now() WHERE investment_id = $1`, investmentID)
	require.NoError(t, err)
}

func TestInvestmentLiquidationLifecycle(t *testing.T) {
	db, repository, user, investment := seedTestInvestment(t, "LIQUIDATION")

	requested, err := repository.RequestLiquidation(investment.InvestmentID, user.UserAccountID)
	require.NoError(t, err)
	require.NotNil(t, requested.LiquidationRequestedAt)
	firstRequestedAt := *requested.LiquidationRequestedAt

	requestedAgain, err := repository.RequestLiquidation(investment.InvestmentID, user.UserAccountID)
	require.NoError(t, err)
	require.Equal(t, firstRequestedAt, *requestedAgain.LiquidationRequestedAt)

	_, err = repository.RequestLiquidation(investment.InvestmentID, uuid.New())
	require.True(t, errors.Is(err, ErrInvestmentNotFound))

	pauseInvestment(t, db, investment.InvestmentID)
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

func TestInvestmentUnpause(t *testing.T) {
	t.Run("clears paused_at and makes the investment list-eligible", func(t *testing.T) {
		db, repo, user, investment := seedTestInvestment(t, "UNPAUSE")
		pauseInvestment(t, db, investment.InvestmentID)

		pausedListed, err := repo.List(StrategyInvestmentListFilter{UserAccountIDs: []uuid.UUID{user.UserAccountID}})
		require.NoError(t, err)
		require.Empty(t, pausedListed)

		result, err := repo.Unpause(investment.InvestmentID)
		require.NoError(t, err)
		require.False(t, result.AlreadyActive)
		require.Nil(t, result.Investment.PausedAt)

		active, err := repo.List(StrategyInvestmentListFilter{UserAccountIDs: []uuid.UUID{user.UserAccountID}})
		require.NoError(t, err)
		require.Len(t, active, 1)
		require.Equal(t, investment.InvestmentID, active[0].InvestmentID)
		require.Nil(t, active[0].PausedAt)

		again, err := repo.Unpause(investment.InvestmentID)
		require.NoError(t, err)
		require.True(t, again.AlreadyActive)
		require.Nil(t, again.Investment.PausedAt)
	})

	t.Run("not found", func(t *testing.T) {
		_, repo, _, _ := seedTestInvestment(t, "UNPAUSE_MISSING")
		_, err := repo.Unpause(uuid.New())
		require.ErrorIs(t, err, ErrInvestmentNotFound)
	})

	t.Run("already active is a no-op", func(t *testing.T) {
		_, repo, _, investment := seedTestInvestment(t, "UNPAUSE_ACTIVE")
		result, err := repo.Unpause(investment.InvestmentID)
		require.NoError(t, err)
		require.True(t, result.AlreadyActive)
		require.Nil(t, result.Investment.PausedAt)
	})

	t.Run("refuses ended investments", func(t *testing.T) {
		db, repo, user, investment := seedTestInvestment(t, "UNPAUSE_ENDED")
		pauseInvestment(t, db, investment.InvestmentID)
		_, err := repo.RequestLiquidation(investment.InvestmentID, user.UserAccountID)
		require.NoError(t, err)
		completed, err := repo.CompleteLiquidation(nil, investment.InvestmentID)
		require.NoError(t, err)
		require.True(t, completed)

		_, err = repo.Unpause(investment.InvestmentID)
		require.ErrorIs(t, err, ErrInvestmentEnded)
	})

	t.Run("refuses investments in liquidation", func(t *testing.T) {
		db, repo, user, investment := seedTestInvestment(t, "UNPAUSE_LIQ")
		pauseInvestment(t, db, investment.InvestmentID)
		_, err := repo.RequestLiquidation(investment.InvestmentID, user.UserAccountID)
		require.NoError(t, err)

		_, err = repo.Unpause(investment.InvestmentID)
		require.ErrorIs(t, err, ErrInvestmentInLiquidation)
	})
}
