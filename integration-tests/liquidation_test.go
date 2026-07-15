package integration_tests

import (
	"bytes"
	"encoding/json"
	"factorbacktest/api"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/testseed"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type liquidationTestBroker struct {
	mockAlpacaForTestsHandler
	mu            sync.Mutex
	positions     []alpaca.Position
	requests      []repository.AlpacaPlaceOrderRequest
	orders        map[uuid.UUID]alpaca.Order
	orderRequests map[uuid.UUID]repository.AlpacaPlaceOrderRequest
	completed     bool
}

func newLiquidationTestBroker(positions []alpaca.Position) *liquidationTestBroker {
	return &liquidationTestBroker{
		positions:     positions,
		orders:        map[uuid.UUID]alpaca.Order{},
		orderRequests: map[uuid.UUID]repository.AlpacaPlaceOrderRequest{},
	}
}

func (b *liquidationTestBroker) PlaceOrder(req repository.AlpacaPlaceOrderRequest) (*alpaca.Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	providerID := uuid.New()
	order := alpaca.Order{ID: providerID.String(), Status: "new"}
	b.requests = append(b.requests, req)
	b.orders[providerID] = order
	b.orderRequests[providerID] = req
	return &order, nil
}

func (b *liquidationTestBroker) GetPositions() ([]alpaca.Position, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]alpaca.Position(nil), b.positions...), nil
}

func (b *liquidationTestBroker) GetOrder(providerID uuid.UUID) (*alpaca.Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	order, ok := b.orders[providerID]
	if !ok {
		return nil, fmt.Errorf("order %s not found", providerID)
	}
	if b.completed {
		now := time.Now().UTC()
		req := b.orderRequests[providerID]
		order.Status = "filled"
		order.FilledAt = &now
		order.FilledQty = req.Quantity
		price := liquidationTestPrices()[req.Symbol]
		order.FilledAvgPrice = &price
	}
	return &order, nil
}

func (b *liquidationTestBroker) completeAllOrders() {
	b.mu.Lock()
	b.completed = true
	b.mu.Unlock()
}

func (b *liquidationTestBroker) placedRequests() []repository.AlpacaPlaceOrderRequest {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]repository.AlpacaPlaceOrderRequest(nil), b.requests...)
}

func liquidationTestPrices() map[string]decimal.Decimal {
	return map[string]decimal.Decimal{
		"AAPL": decimal.NewFromFloat(130.04466247558594),
		"GOOG": decimal.NewFromFloat(87.5940017700195),
	}
}

func TestInvestmentLiquidationHTTPFlow(t *testing.T) {
	t.Setenv("CRON_SECRET", "test-cron-secret")
	manager, err := NewTestDbManager()
	require.NoError(t, err)
	defer manager.Close()
	db := manager.DB()

	aapl := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "AAPL", Name: "Apple"})
	goog := testseed.CreateTicker(db, testseed.TickerOpts{Symbol: "GOOG", Name: "Alphabet"})
	universe := testseed.CreateAssetUniverse(db, testseed.AssetUniverseOpts{Name: "LIQUIDATION_TEST"})
	testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, aapl.TickerID)
	testseed.CreateAssetUniverseTicker(db, universe.AssetUniverseID, goog.TickerID)
	user := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: "liquidation@example.com"})
	otherUser := testseed.CreateUserAccount(db, testseed.UserAccountOpts{Email: "other@example.com"})
	strategy := testseed.CreateStrategy(db, testseed.StrategyOpts{
		Name:              "liquidation test strategy",
		UserAccountID:     user.UserAccountID,
		AssetUniverse:     universe.AssetUniverseName,
		NumAssets:         3,
		RebalanceInterval: "DAILY",
		FactorExpression:  "price()",
	})
	investment := testseed.CreateInvestment(db, testseed.InvestmentOpts{
		StrategyID:    strategy.StrategyID,
		UserAccountID: user.UserAccountID,
		AmountDollars: 100,
	})
	version := testseed.CreateInvestmentHoldingsVersion(db, investment.InvestmentID)
	cash := testseed.LookupTickerBySymbol(db, ":CASH")
	testseed.CreateInvestmentHolding(db, testseed.InvestmentHoldingOpts{VersionID: version.InvestmentHoldingsVersionID, TickerID: cash.TickerID, Quantity: decimal.NewFromInt(10)})
	testseed.CreateInvestmentHolding(db, testseed.InvestmentHoldingOpts{VersionID: version.InvestmentHoldingsVersionID, TickerID: aapl.TickerID, Quantity: decimal.NewFromFloat(0.5)})
	testseed.CreateInvestmentHolding(db, testseed.InvestmentHoldingOpts{VersionID: version.InvestmentHoldingsVersionID, TickerID: goog.TickerID, Quantity: decimal.NewFromFloat(0.25)})

	broker := newLiquidationTestBroker([]alpaca.Position{
		{Symbol: "AAPL", Qty: decimal.NewFromFloat(0.5), QtyAvailable: decimal.NewFromFloat(0.5)},
		{Symbol: "GOOG", Qty: decimal.NewFromFloat(0.25), QtyAvailable: decimal.NewFromFloat(0.25)},
	})
	server, err := NewTestServerWithDependencies(manager, broker, func(c *gin.Context) {
		if userAccountID := c.GetHeader("X-Test-User-Account-ID"); userAccountID != "" {
			c.Set("userAccountID", userAccountID)
		}
		c.Next()
	})
	require.NoError(t, err)
	defer server.Stop()

	response := map[string]bool{}
	hitAuthenticatedEndpoint(t, server.URL, "/investments/"+investment.InvestmentID.String()+"/request-liquidation", http.MethodPost, otherUser.UserAccountID, http.StatusNotFound, nil)
	unmodified, err := repository.NewInvestmentRepository(db).Get(investment.InvestmentID)
	require.NoError(t, err)
	require.Nil(t, unmodified.LiquidationRequestedAt)

	hitAuthenticatedEndpoint(t, server.URL, "/investments/"+investment.InvestmentID.String()+"/request-liquidation", http.MethodPost, user.UserAccountID, http.StatusAccepted, &response)
	require.True(t, response["success"])

	investmentRepository := repository.NewInvestmentRepository(db)
	requested, err := investmentRepository.Get(investment.InvestmentID)
	require.NoError(t, err)
	require.NotNil(t, requested.LiquidationRequestedAt)
	requestedAt := *requested.LiquidationRequestedAt
	allInvestments := []api.GetInvestmentsResponse{}
	hitAuthenticatedEndpoint(t, server.URL, "/investments", http.MethodGet, user.UserAccountID, http.StatusOK, &allInvestments)
	require.Len(t, allInvestments, 1)
	require.Equal(t, "LIQUIDATION_REQUESTED", allInvestments[0].Status)
	require.Nil(t, allInvestments[0].EndDate)

	hitAuthenticatedEndpoint(t, server.URL, "/investments/"+investment.InvestmentID.String()+"/request-liquidation", http.MethodPost, user.UserAccountID, http.StatusAccepted, &response)
	requestedAgain, err := investmentRepository.Get(investment.InvestmentID)
	require.NoError(t, err)
	require.Equal(t, requestedAt, *requestedAgain.LiquidationRequestedAt)

	require.NoError(t, hitEndpoint(server.URL, "internal/cron/rebalance", http.MethodPost, map[string]string{}, &map[string]string{}))
	requests := broker.placedRequests()
	require.Len(t, requests, 2)
	sort.Slice(requests, func(i, j int) bool { return requests[i].Symbol < requests[j].Symbol })
	require.Equal(t, "AAPL", requests[0].Symbol)
	require.Equal(t, alpaca.Sell, requests[0].Side)
	require.Equal(t, decimal.NewFromFloat(0.5), requests[0].Quantity)
	require.Equal(t, "GOOG", requests[1].Symbol)
	require.Equal(t, alpaca.Sell, requests[1].Side)
	require.Equal(t, decimal.NewFromFloat(0.25), requests[1].Quantity)
	allInvestments = nil
	hitAuthenticatedEndpoint(t, server.URL, "/investments", http.MethodGet, user.UserAccountID, http.StatusOK, &allInvestments)
	require.Len(t, allInvestments, 1)
	require.Equal(t, "LIQUIDATING", allInvestments[0].Status)

	pending, err := investmentRepository.Get(investment.InvestmentID)
	require.NoError(t, err)
	require.Nil(t, pending.EndDate)

	broker.completeAllOrders()
	require.NoError(t, hitEndpoint(server.URL, "internal/cron/updateOrders", http.MethodPost, map[string]string{}, &map[string]string{}))

	ended, err := investmentRepository.Get(investment.InvestmentID)
	require.NoError(t, err)
	require.NotNil(t, ended.EndDate)
	latest, err := repository.NewInvestmentHoldingsRepository(db).GetLatestHoldings(nil, investment.InvestmentID)
	require.NoError(t, err)
	require.Empty(t, latest.Positions)
	expectedCash := decimal.NewFromInt(10).
		Add(decimal.NewFromFloat(0.5).Mul(liquidationTestPrices()["AAPL"])).
		Add(decimal.NewFromFloat(0.25).Mul(liquidationTestPrices()["GOOG"]))
	require.True(t, latest.Cash.Equal(expectedCash), "cash: got %s want %s", latest.Cash, expectedCash)

	active := []api.GetInvestmentsResponse{}
	hitAuthenticatedEndpoint(t, server.URL, "/activeInvestments", http.MethodGet, user.UserAccountID, http.StatusOK, &active)
	require.Empty(t, active)
	allInvestments = nil
	hitAuthenticatedEndpoint(t, server.URL, "/investments", http.MethodGet, user.UserAccountID, http.StatusOK, &allInvestments)
	require.Len(t, allInvestments, 1)
	require.Equal(t, "LIQUIDATED", allInvestments[0].Status)
	require.NotNil(t, allInvestments[0].EndDate)

	require.NoError(t, hitEndpoint(server.URL, "internal/cron/rebalance", http.MethodPost, map[string]string{}, &map[string]string{}))
	require.Len(t, broker.placedRequests(), 2, "an ended investment must not create more orders")
}

func hitAuthenticatedEndpoint(t *testing.T, baseURL, path, method string, userAccountID uuid.UUID, expectedStatus int, target any) {
	t.Helper()
	request, err := http.NewRequest(method, baseURL+path, bytes.NewReader([]byte("{}")))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Test-User-Account-ID", userAccountID.String())
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, expectedStatus, response.StatusCode, "body=%s", strings.TrimSpace(string(body)))
	if target != nil && len(body) > 0 {
		require.NoError(t, json.Unmarshal(body, target))
	}
}

var _ repository.AlpacaRepository = (*liquidationTestBroker)(nil)
