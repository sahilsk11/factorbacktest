package integration_tests

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/util"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

var cashTicker = uuid.New()

func seedInvestment(tx *sql.Tx) error {
	userAccount := model.UserAccount{}
	err := table.UserAccount.
		INSERT(table.UserAccount.MutableColumns).
		MODEL(model.UserAccount{
			FirstName: "Test",
			LastName:  "User",
			Email:     "test@gmail.com",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}).
		RETURNING(table.UserAccount.AllColumns).
		Query(tx, &userAccount)
	if err != nil {
		return fmt.Errorf("failed to insert user account: %w", err)
	}

	strategy := model.Strategy{}
	err = table.Strategy.
		INSERT(table.Strategy.MutableColumns).
		MODEL(model.Strategy{
			StrategyName: "test_strategy",
			FactorExpression: `pricePercentChange(
  nDaysAgo(7),
  currentDate
)`,
			RebalanceInterval: "MONTHLY",
			NumAssets:         3,
			AssetUniverse:     "SPY_TOP_80",
			UserAccountID:     &userAccount.UserAccountID,
			CreatedAt:         time.Now(),
			ModifiedAt:        time.Now(),
			Published:         false,
			Saved:             false,
			Description:       nil,
		}).
		RETURNING(table.Strategy.AllColumns).
		Query(tx, &strategy)
	if err != nil {
		return fmt.Errorf("failed to insert strategy: %w", err)
	}

	investment := model.Investment{}
	err = table.Investment.
		INSERT(table.Investment.MutableColumns).
		MODEL(model.Investment{
			AmountDollars: 100,
			StartDate:     time.Now(),
			StrategyID:    strategy.StrategyID,
			UserAccountID: userAccount.UserAccountID,
			CreatedAt:     time.Now(),
			ModifiedAt:    time.Now(),
			EndDate:       nil,
			PausedAt:      nil,
		}).
		RETURNING(table.Investment.AllColumns).
		Query(tx, &investment)
	if err != nil {
		return fmt.Errorf("failed to insert investment: %w", err)
	}

	holdingVersion := model.InvestmentHoldingsVersion{}
	err = table.InvestmentHoldingsVersion.
		INSERT(table.InvestmentHoldingsVersion.MutableColumns).
		MODEL(model.InvestmentHoldingsVersion{
			InvestmentID:    investment.InvestmentID,
			CreatedAt:       time.Now(),
			RebalancerRunID: nil,
		}).
		RETURNING(table.InvestmentHoldingsVersion.AllColumns).
		Query(tx, &holdingVersion)
	if err != nil {
		return fmt.Errorf("failed to insert holding version: %w", err)
	}

	holding := model.InvestmentHoldings{}
	err = table.InvestmentHoldings.
		INSERT(table.InvestmentHoldings.MutableColumns).
		MODEL(model.InvestmentHoldings{
			InvestmentID:                investment.InvestmentID,
			TickerID:                    cashTicker,
			Quantity:                    decimal.NewFromInt(100),
			CreatedAt:                   time.Now(),
			InvestmentHoldingsVersionID: holdingVersion.InvestmentHoldingsVersionID,
		}).
		RETURNING(table.InvestmentHoldings.AllColumns).
		Query(tx, &holding)
	if err != nil {
		return fmt.Errorf("failed to insert holding: %w", err)
	}

	return nil
}

func cleanupUsers(db *sql.DB) error {
	if _, err := table.UserStrategy.DELETE().WHERE(postgres.Bool(true)).Exec(db); err != nil {
		return err
	}
	if _, err := table.LatencyTracking.DELETE().WHERE(postgres.Bool(true)).Exec(db); err != nil {
		return err
	}
	if _, err := table.APIRequest.DELETE().WHERE(postgres.Bool(true)).Exec(db); err != nil {
		return err
	}
	if _, err := table.UserAccount.DELETE().WHERE(postgres.Bool(true)).Exec(db); err != nil {
		return err
	}
	return nil
}

func cleanupRebalance(db *sql.DB) error {
	_, err := table.ExcessTradeVolume.DELETE().WHERE(postgres.Bool(true)).Exec(db)
	if err != nil {
		return err
	}
	_, err = table.InvestmentTrade.DELETE().WHERE(postgres.Bool(true)).Exec(db)
	if err != nil {
		return err
	}
	_, err = table.RebalancePrice.DELETE().WHERE(postgres.Bool(true)).Exec(db)
	if err != nil {
		return err
	}

	_, err = table.TradeOrder.DELETE().WHERE(postgres.Bool(true)).Exec(db)
	if err != nil {
		return err
	}

	_, err = table.InvestmentHoldings.DELETE().WHERE(postgres.Bool(true)).Exec(db)
	if err != nil {
		return err
	}
	_, err = table.InvestmentRebalance.DELETE().WHERE(postgres.Bool(true)).Exec(db)
	if err != nil {
		return err
	}
	_, err = table.InvestmentHoldingsVersion.DELETE().WHERE(postgres.Bool(true)).Exec(db)
	if err != nil {
		return err
	}

	_, err = table.RebalancerRun.DELETE().WHERE(postgres.Bool(true)).Exec(db)
	if err != nil {
		return err
	}

	return nil
}

func Test_rebalanceFlow(t *testing.T) {
	cleanup := func(db *sql.DB) {
		err := cleanupRebalance(db)
		require.NoError(t, err)
		err = cleanupStrategies(db)
		require.NoError(t, err)
		err = cleanupUsers(db)
		require.NoError(t, err)
		err = cleanupUniverse(db)
		require.NoError(t, err)
	}
	db, err := util.NewTestDb()
	require.NoError(t, err)
	cleanup(db) // redundant but ensures tables are empty

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()
	// defer cleanup(db)

	err = seedUniverse(tx)
	require.NoError(t, err)
	err = seedPrices(tx)
	require.NoError(t, err)
	err = seedInvestment(tx)
	require.NoError(t, err)
	err = tx.Commit()
	require.NoError(t, err)

	startTime := time.Now()
	request := map[string]string{}
	response := map[string]string{}
	err = hitEndpoint("rebalance", http.MethodPost, request, &response)
	require.NoError(t, err)
	elapsed := time.Since(startTime).Milliseconds()

	excess, err := getExcess(db)
	require.NoError(t, err)
	require.Equal(t, 1, len(excess))
	require.InEpsilon(t, 0.0112807398018001, excess[0].Quantity.InexactFloat64(), 0.00001)

	// maybe investment rebalance

	trades, err := getInvestmentTrades(db)
	require.NoError(t, err)
	require.Equal(t, "", cmp.Diff(
		[]model.InvestmentTrade{
			{
				// TickerID:              [16]byte{},
				Side:     model.TradeOrderSide_Buy,
				Quantity: decimal.NewFromFloat(0.0002537589730466),
				// TradeOrderID:          &[16]byte{},
				// InvestmentRebalanceID: [16]byte{},
				ExpectedPrice: decimal.NewFromFloat(130.04466247558594),
			},
			{
				// TickerID:              [16]byte{},
				Side:     model.TradeOrderSide_Buy,
				Quantity: decimal.NewFromFloat(0.3245532418788067),
				// TradeOrderID:          &[16]byte{},
				// InvestmentRebalanceID: [16]byte{},
				ExpectedPrice: decimal.NewFromFloat(272.8704833984375),
			},
			{
				// TickerID:              [16]byte{},
				Side:     model.TradeOrderSide_Buy,
				Quantity: decimal.NewFromFloat(0.1302143956152017),
				// TradeOrderID:          &[16]byte{},
				// InvestmentRebalanceID: [16]byte{},
				ExpectedPrice: decimal.NewFromFloat(87.5940017700195),
			},
		},
		trades,
		cmpopts.IgnoreFields(model.InvestmentTrade{}, "TickerID"),
		cmpopts.IgnoreFields(model.InvestmentTrade{}, "InvestmentTradeID"),
		cmpopts.IgnoreFields(model.InvestmentTrade{}, "CreatedAt"),
		cmpopts.IgnoreFields(model.InvestmentTrade{}, "ModifiedAt"),
		cmpopts.IgnoreFields(model.InvestmentTrade{}, "TradeOrderID"),
		cmpopts.IgnoreFields(model.InvestmentTrade{}, "InvestmentRebalanceID"),
		cmp.Comparer(func(d1, d2 decimal.Decimal) bool {
			return d1.Sub(d2).Abs().LessThan(decimal.NewFromFloat(0.00001))
		}),
		cmpopts.SortSlices(func(i, j model.InvestmentTrade) bool {
			return i.Quantity.LessThan(j.Quantity)
		}),
	))

	rebalancePrices, err := getRebalancePrices(db)
	require.NoError(t, err)
	require.Equal(t, "", cmp.Diff(
		[]model.RebalancePrice{
			{
				// TickerID:         [16]byte{},
				Price: decimal.NewFromFloat(87.5940017700195),
				// RebalancerRunID:  [16]byte{},
			},
			{
				// TickerID:         [16]byte{},
				Price: decimal.NewFromFloat(130.04466247558594),
				// RebalancerRunID:  [16]byte{},
			},
			{
				// TickerID:         [16]byte{},
				Price: decimal.NewFromFloat(272.8704833984375),
				// RebalancerRunID:  [16]byte{},
			},
		},
		rebalancePrices,
		cmpopts.IgnoreFields(model.RebalancePrice{}, "RebalancePriceID"),
		cmpopts.IgnoreFields(model.RebalancePrice{}, "TickerID"),
		cmpopts.IgnoreFields(model.RebalancePrice{}, "RebalancerRunID"),
		cmpopts.IgnoreFields(model.RebalancePrice{}, "CreatedAt"),
		cmpopts.SortSlices(func(i, j model.RebalancePrice) bool {
			return i.Price.LessThan(j.Price)
		}),
	))

	date, err := time.Parse(time.DateOnly, "2020-12-31")
	require.NoError(t, err)
	rebalancerRuns, err := getRebalancerRuns(db)
	require.NoError(t, err)
	require.Equal(t, "", cmp.Diff(
		[]model.RebalancerRun{
			{
				Date:                    date,
				RebalancerRunType:       model.RebalancerRunType_ManualInvestmentRebalance,
				RebalancerRunState:      model.RebalancerRunState_Pending,
				NumInvestmentsAttempted: 1,
			},
		},
		rebalancerRuns,
		cmpopts.IgnoreFields(model.RebalancerRun{}, "RebalancerRunID"),
		cmpopts.IgnoreFields(model.RebalancerRun{}, "CreatedAt"),
		cmpopts.IgnoreFields(model.RebalancerRun{}, "ModifiedAt"),
	))

	tradeOrders, err := getTradeOrders(db)
	require.NoError(t, err)
	require.Equal(t, "", cmp.Diff(
		[]model.TradeOrder{
			{
				// ProviderID:        &[16]byte{},
				// TickerID:          [16]byte{},
				Side:           model.TradeOrderSide_Buy,
				Status:         model.TradeOrderStatus_Pending,
				FilledQuantity: decimal.Zero,
				FilledPrice:    nil,
				FilledAt:       nil,
				// RebalancerRunID:   [16]byte{},
				RequestedQuantity: decimal.NewFromFloat(0.1302143956152017),
				ExpectedPrice:     decimal.NewFromFloat(87.5940017700195),
			},
			{
				// ProviderID:        &[16]byte{},
				// TickerID:          [16]byte{},
				Side:           model.TradeOrderSide_Buy,
				Status:         model.TradeOrderStatus_Pending,
				FilledQuantity: decimal.Zero,
				FilledPrice:    nil,
				FilledAt:       nil,
				// RebalancerRunID:   [16]byte{},
				RequestedQuantity: decimal.NewFromFloat(0.0115344987748467),
				ExpectedPrice:     decimal.NewFromFloat(130.04466247558594),
			},
			{
				// ProviderID:        &[16]byte{},
				// TickerID:          [16]byte{},
				Side:           model.TradeOrderSide_Buy,
				Status:         model.TradeOrderStatus_Pending,
				FilledQuantity: decimal.Zero,
				FilledPrice:    nil,
				FilledAt:       nil,
				// RebalancerRunID:   [16]byte{},
				RequestedQuantity: decimal.NewFromFloat(0.3245532418788067),
				ExpectedPrice:     decimal.NewFromFloat(272.8704833984375),
			},
		},
		tradeOrders,
		cmpopts.SortSlices(func(t1, t2 model.TradeOrder) bool {
			return t1.RequestedQuantity.LessThan(t2.RequestedQuantity)
		}),
		cmpopts.IgnoreFields(model.TradeOrder{}, "TradeOrderID"),
		cmpopts.IgnoreFields(model.TradeOrder{}, "ProviderID"),
		cmpopts.IgnoreFields(model.TradeOrder{}, "TickerID"),
		cmpopts.IgnoreFields(model.TradeOrder{}, "CreatedAt"),
		cmpopts.IgnoreFields(model.TradeOrder{}, "ModifiedAt"),
		cmpopts.IgnoreFields(model.TradeOrder{}, "RebalancerRunID"),
	))

	require.Less(t, elapsed, int64(2500))
}

func getExcess(db *sql.DB) ([]model.ExcessTradeVolume, error) {
	out := []model.ExcessTradeVolume{}
	err := table.ExcessTradeVolume.SELECT(table.ExcessTradeVolume.AllColumns).Query(db, &out)
	return out, err
}

func getInvestmentTrades(db *sql.DB) ([]model.InvestmentTrade, error) {
	out := []model.InvestmentTrade{}
	err := table.InvestmentTrade.SELECT(table.InvestmentTrade.AllColumns).Query(db, &out)
	return out, err
}

func getRebalancePrices(db *sql.DB) ([]model.RebalancePrice, error) {
	out := []model.RebalancePrice{}
	err := table.RebalancePrice.SELECT(table.RebalancePrice.AllColumns).Query(db, &out)
	return out, err
}

func getRebalancerRuns(db *sql.DB) ([]model.RebalancerRun, error) {
	out := []model.RebalancerRun{}
	err := table.RebalancerRun.SELECT(table.RebalancerRun.AllColumns).Query(db, &out)
	return out, err
}

func getTradeOrders(db *sql.DB) ([]model.TradeOrder, error) {
	out := []model.TradeOrder{}
	err := table.TradeOrder.SELECT(table.TradeOrder.AllColumns).Query(db, &out)
	return out, err
}
