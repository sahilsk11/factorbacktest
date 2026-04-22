package testseed

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/util"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/shopspring/decimal"
)

// FixtureUserBasic creates a single manual UserAccount.
const FixtureUserBasic = "user_basic"

// Keys exposed by user_basic.
const (
	KeyUserAccountID = "user_account_id"
)

var userBasicFixture = Fixture{
	Name: FixtureUserBasic,
	Apply: func(ctx context.Context, db *sql.DB, _ map[string]Result) (Result, error) {
		var user model.UserAccount
		if err := table.UserAccount.
			INSERT(table.UserAccount.MutableColumns).
			MODEL(model.UserAccount{
				FirstName: util.StringPointer("Test"),
				LastName:  util.StringPointer("User"),
				Email:     util.StringPointer("test@gmail.com"),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Provider:  model.UserAccountProviderType_Manual,
			}).
			RETURNING(table.UserAccount.AllColumns).
			Query(db, &user); err != nil {
			return nil, fmt.Errorf("insert user_account: %w", err)
		}
		return Result{KeyUserAccountID: user.UserAccountID}, nil
	},
}

// FixtureStrategyMomentum creates the sample momentum strategy used by the
// rebalance and backtest integration tests.
const FixtureStrategyMomentum = "strategy_momentum"

// Keys exposed by strategy_momentum.
const (
	KeyStrategyID = "strategy_id"
)

const sampleMomentumFactorExpression = `pricePercentChange(
  nDaysAgo(7),
  currentDate
)`

var strategyMomentumFixture = Fixture{
	Name:         FixtureStrategyMomentum,
	Dependencies: []string{FixtureBaseUniverse, FixtureUserBasic},
	Apply: func(ctx context.Context, db *sql.DB, deps map[string]Result) (Result, error) {
		userID, err := lookupUUID(deps, FixtureUserBasic, KeyUserAccountID)
		if err != nil {
			return nil, err
		}
		var strategy model.Strategy
		if err := table.Strategy.
			INSERT(table.Strategy.MutableColumns).
			MODEL(model.Strategy{
				StrategyName:      "test_strategy",
				FactorExpression:  sampleMomentumFactorExpression,
				RebalanceInterval: "MONTHLY",
				NumAssets:         3,
				AssetUniverse:     "SPY_TOP_80",
				UserAccountID:     &userID,
				CreatedAt:         time.Now(),
				ModifiedAt:        time.Now(),
				Published:         false,
				Saved:             false,
				Description:       nil,
			}).
			RETURNING(table.Strategy.AllColumns).
			Query(db, &strategy); err != nil {
			return nil, fmt.Errorf("insert strategy: %w", err)
		}
		return Result{KeyStrategyID: strategy.StrategyID}, nil
	},
}

// FixtureInvestmentBasic creates an Investment + HoldingsVersion + a single
// $100 :CASH holding against the sample momentum strategy. Pulls in the
// whole chain via transitive dependencies.
const FixtureInvestmentBasic = "investment_basic"

// Keys exposed by investment_basic.
const (
	KeyInvestmentID               = "investment_id"
	KeyInvestmentHoldingsVersion  = "investment_holdings_version_id"
	KeyInvestmentCashHoldingID    = "investment_cash_holding_id"
)

var investmentBasicFixture = Fixture{
	Name: FixtureInvestmentBasic,
	Dependencies: []string{
		FixtureStrategyMomentum,
		FixturePrices2020,
	},
	Apply: func(ctx context.Context, db *sql.DB, deps map[string]Result) (Result, error) {
		userID, err := lookupUUID(deps, FixtureUserBasic, KeyUserAccountID)
		if err != nil {
			return nil, err
		}
		strategyID, err := lookupUUID(deps, FixtureStrategyMomentum, KeyStrategyID)
		if err != nil {
			return nil, err
		}

		var inv model.Investment
		if err := table.Investment.
			INSERT(table.Investment.MutableColumns).
			MODEL(model.Investment{
				AmountDollars: 100,
				StartDate:     time.Now(),
				StrategyID:    strategyID,
				UserAccountID: userID,
				CreatedAt:     time.Now(),
				ModifiedAt:    time.Now(),
				EndDate:       nil,
				PausedAt:      nil,
			}).
			RETURNING(table.Investment.AllColumns).
			Query(db, &inv); err != nil {
			return nil, fmt.Errorf("insert investment: %w", err)
		}

		var hv model.InvestmentHoldingsVersion
		if err := table.InvestmentHoldingsVersion.
			INSERT(table.InvestmentHoldingsVersion.MutableColumns).
			MODEL(model.InvestmentHoldingsVersion{
				InvestmentID:    inv.InvestmentID,
				CreatedAt:       time.Now(),
				RebalancerRunID: nil,
			}).
			RETURNING(table.InvestmentHoldingsVersion.AllColumns).
			Query(db, &hv); err != nil {
			return nil, fmt.Errorf("insert investment_holdings_version: %w", err)
		}

		var cashTicker model.Ticker
		if err := table.Ticker.
			SELECT(table.Ticker.AllColumns).
			WHERE(table.Ticker.Symbol.EQ(postgres.String(":CASH"))).
			Query(db, &cashTicker); err != nil {
			return nil, fmt.Errorf("lookup :CASH ticker: %w", err)
		}

		var holding model.InvestmentHoldings
		if err := table.InvestmentHoldings.
			INSERT(table.InvestmentHoldings.MutableColumns).
			MODEL(model.InvestmentHoldings{
				TickerID:                    cashTicker.TickerID,
				Quantity:                    decimal.NewFromInt(100),
				CreatedAt:                   time.Now(),
				InvestmentHoldingsVersionID: hv.InvestmentHoldingsVersionID,
			}).
			RETURNING(table.InvestmentHoldings.AllColumns).
			Query(db, &holding); err != nil {
			return nil, fmt.Errorf("insert investment_holdings: %w", err)
		}

		return Result{
			KeyInvestmentID:              inv.InvestmentID,
			KeyInvestmentHoldingsVersion: hv.InvestmentHoldingsVersionID,
			KeyInvestmentCashHoldingID:   holding.InvestmentHoldingsID,
		}, nil
	},
}
