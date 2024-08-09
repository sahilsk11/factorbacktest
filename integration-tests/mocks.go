package integration_tests

import (
	"context"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/util"
	"fmt"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func NewMockAlpacaRepositoryForTests() repository.AlpacaRepository {
	return mockAlpacaForTestsHandler{}
}

type mockAlpacaForTestsHandler struct {
}

func (m mockAlpacaForTestsHandler) GetLatestPrices(symbols []string) (map[string]decimal.Decimal, error) {
	return map[string]decimal.Decimal{
		"AAPL": decimal.NewFromFloat(130.04466247558594),
		"META": decimal.NewFromFloat(272.8704833984375),
		"GOOG": decimal.NewFromFloat(87.5940017700195),
	}, nil
}

func (m mockAlpacaForTestsHandler) PlaceOrder(req repository.AlpacaPlaceOrderRequest) (*alpaca.Order, error) {
	if "" == cmp.Diff(
		repository.AlpacaPlaceOrderRequest{
			Quantity:   decimal.NewFromFloat(0.3245532418788067),
			Symbol:     "META",
			Side:       "buy",
			LimitPrice: util.DecimalPointer(decimal.NewFromFloat(286.51)),
		},
		req,
		cmpopts.IgnoreFields(repository.AlpacaPlaceOrderRequest{}, "TradeOrderID"),
		cmp.Comparer(func(d1, d2 decimal.Decimal) bool {
			return d1.Sub(d2).Abs().LessThan(decimal.NewFromFloat(0.00001))
		}),
	) {
		return &alpaca.Order{
			ID: uuid.NewString(),
		}, nil
	} else if "" == cmp.Diff(
		repository.AlpacaPlaceOrderRequest{
			Quantity:   decimal.NewFromFloat(0.0115344987748467),
			Symbol:     "AAPL",
			Side:       "buy",
			LimitPrice: util.DecimalPointer(decimal.NewFromFloat(136.55)),
		},
		req,
		cmpopts.IgnoreFields(repository.AlpacaPlaceOrderRequest{}, "TradeOrderID"),
		cmp.Comparer(func(d1, d2 decimal.Decimal) bool {
			return d1.Sub(d2).Abs().LessThan(decimal.NewFromFloat(0.00001))
		}),
	) {
		return &alpaca.Order{
			ID: uuid.NewString(),
		}, nil
	} else if "" == cmp.Diff(
		repository.AlpacaPlaceOrderRequest{
			Quantity:   decimal.NewFromFloat(0.1302143956152017),
			Symbol:     "GOOG",
			Side:       "buy",
			LimitPrice: util.DecimalPointer(decimal.NewFromFloat(91.97)),
		},
		req,
		cmpopts.IgnoreFields(repository.AlpacaPlaceOrderRequest{}, "TradeOrderID"),
		cmp.Comparer(func(d1, d2 decimal.Decimal) bool {
			return d1.Sub(d2).Abs().LessThan(decimal.NewFromFloat(0.00001))
		}),
	) {
		return &alpaca.Order{
			ID: uuid.NewString(),
		}, nil
	}

	return nil, fmt.Errorf("PlaceOrder not implemented")
}

func (m mockAlpacaForTestsHandler) GetLatestPricesWithTs(symbols []string) (map[string]domain.AssetPrice, error) {
	return nil, fmt.Errorf("GetLatestPricesWithTs not implemented")
}

func (m mockAlpacaForTestsHandler) CancelOpenOrders(ctx context.Context) error {
	return fmt.Errorf("CancelOpenOrders not implemented")
}

func (m mockAlpacaForTestsHandler) GetPositions() ([]alpaca.Position, error) {
	return []alpaca.Position{}, nil
}

func (m mockAlpacaForTestsHandler) IsMarketOpen() (bool, error) {
	return false, fmt.Errorf("IsMarketOpen not implemented")
}

func (m mockAlpacaForTestsHandler) GetAccount() (*alpaca.Account, error) {
	return nil, fmt.Errorf("GetAccount not implemented")
}

func (m mockAlpacaForTestsHandler) GetOrder(alpacaOrderID uuid.UUID) (*alpaca.Order, error) {
	return nil, fmt.Errorf("GetOrder not implemented")
}
