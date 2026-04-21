package integration_tests

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"factorbacktest/api"
	"factorbacktest/cmd"
	"factorbacktest/internal/data"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/util"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type TestServer struct {
	URL      string
	listener net.Listener
	server   *http.Server
}

func NewTestServer(testDb *TestDbManager) (*TestServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen on ephemeral port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	secrets := util.Secrets{
		Port:             port,
		DataJockeyApiKey: "",
		ChatGPTApiKey:    "",
		Db:               testDb.DBConfig,
		Alpaca:           util.AlpacaSecrets{},
		Jwt:              "",
		SES:              util.SESSecrets{},
	}

	alpacaRepository := NewMockAlpacaRepositoryForTests()
	priceRepository := repository.NewAdjustedPriceRepository(testDb.db)
	priceService := data.NewPriceService(testDb.db, priceRepository, nil, nil)
	handler, err := cmd.InitializeDependencies(secrets, &api.ApiHandler{
		AlpacaRepository: alpacaRepository,
		PriceService: NewMockPriceServiceForTests(
			priceService,
		),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize dependencies: %w", err)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	lg := logger.New()
	ctx := context.WithValue(context.Background(), logger.ContextKey, lg)
	engine := handler.InitializeRouterEngine(ctx)

	server := &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", port),
		Handler: engine,
	}

	go server.Serve(listener)

	return &TestServer{
		URL:      fmt.Sprintf("http://localhost:%d", port),
		listener: listener,
		server:   server,
	}, nil
}

func (s *TestServer) Stop() error {
	if err := s.server.Shutdown(context.Background()); err != nil {
		s.listener.Close()
	}
	return nil
}

func seedInvestment(db *sql.DB) error {
	userAccount := model.UserAccount{}
	err := table.UserAccount.
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
		Query(db, &userAccount)
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
		Query(db, &strategy)
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
		Query(db, &investment)
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
		Query(db, &holdingVersion)
	if err != nil {
		return fmt.Errorf("failed to insert holding version: %w", err)
	}

	cashTicker := model.Ticker{}
	err = table.Ticker.SELECT(table.Ticker.AllColumns).WHERE(table.Ticker.Symbol.EQ(postgres.String(":CASH"))).Query(db, &cashTicker)
	if err != nil {
		return fmt.Errorf("failed to get cash ticker: %w", err)
	}

	holding := model.InvestmentHoldings{}
	err = table.InvestmentHoldings.
		INSERT(table.InvestmentHoldings.MutableColumns).
		MODEL(model.InvestmentHoldings{
			TickerID:                    cashTicker.TickerID,
			Quantity:                    decimal.NewFromInt(100),
			CreatedAt:                   time.Now(),
			InvestmentHoldingsVersionID: holdingVersion.InvestmentHoldingsVersionID,
		}).
		RETURNING(table.InvestmentHoldings.AllColumns).
		Query(db, &holding)
	if err != nil {
		return fmt.Errorf("failed to insert holding: %w", err)
	}

	return nil
}

func Test_rebalanceFlow(t *testing.T) {
	manager, err := NewTestDbManager()
	require.NoError(t, err)

	defer manager.Close()

	server, err := NewTestServer(manager)
	require.NoError(t, err)
	defer server.Stop()

	db := manager.DB()

	err = seedUniverse(db)
	require.NoError(t, err)

	err = seedPrices(db)
	require.NoError(t, err)

	err = seedInvestment(db)
	require.NoError(t, err)

	startTime := time.Now()
	request := map[string]string{}
	response := map[string]string{}
	err = hitEndpoint(server.URL, "rebalance", http.MethodPost, request, &response)
	require.NoError(t, err)
	elapsed := time.Since(startTime).Milliseconds()

	// consider calling update, reconcile

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
