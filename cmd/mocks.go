package cmd

import (
	"factorbacktest/internal"
	"factorbacktest/internal/repository"
	"fmt"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const UseMockAlpaca = true

// idk if alpaca has sandbox but this is a hacky way to
// simulate markets being open and orders completed
// should not be used in prod, obv

type MockAlpacaRepository interface {
	PlaceOrder(req repository.AlpacaPlaceOrderRequest) (*alpaca.Order, error)
	CancelOpenOrders() error
	GetPositions() ([]alpaca.Position, error)
	IsMarketOpen() (bool, error)
	GetAccount() (*alpaca.Account, error)
	GetOrder(alpacaOrderID uuid.UUID) (*alpaca.Order, error)
	GetLatestPrices(symbols []string) (map[string]decimal.Decimal, error)
}

type mockAlpacaRepositoryHandler struct {
	realAlpacaRepository repository.AlpacaRepository
	tradeOrderRepository repository.TradeOrderRepository
	tickerRepository     repository.TickerRepository
}

func NewMockAlpacaRepository(alpacaRepository repository.AlpacaRepository, toRepository repository.TradeOrderRepository, tickerRepository repository.TickerRepository) MockAlpacaRepository {
	fmt.Println(`*******************
WARNING: Using mock Alpaca service. May not reflect real conditions
*******************`)

	// todo - ensure we're using paper trading

	return mockAlpacaRepositoryHandler{
		realAlpacaRepository: alpacaRepository,
		tradeOrderRepository: toRepository,
		tickerRepository:     tickerRepository,
	}
}
func (m mockAlpacaRepositoryHandler) PlaceOrder(req repository.AlpacaPlaceOrderRequest) (*alpaca.Order, error) {
	return m.realAlpacaRepository.PlaceOrder(req)
}

func (m mockAlpacaRepositoryHandler) CancelOpenOrders() error {
	return m.realAlpacaRepository.CancelOpenOrders()
}

func (m mockAlpacaRepositoryHandler) GetPositions() ([]alpaca.Position, error) {
	return m.realAlpacaRepository.GetPositions()
}

func (m mockAlpacaRepositoryHandler) IsMarketOpen() (bool, error) {
	return m.realAlpacaRepository.IsMarketOpen()
}

func (m mockAlpacaRepositoryHandler) GetAccount() (*alpaca.Account, error) {
	return m.realAlpacaRepository.GetAccount()
}

func (m mockAlpacaRepositoryHandler) GetOrder(alpacaOrderID uuid.UUID) (*alpaca.Order, error) {
	// return m.realAlpacaRepository.GetOrder(alpacaOrderID)

	trade, err := m.tradeOrderRepository.Get(repository.TradeOrderGetFilter{
		ProviderID: &alpacaOrderID,
	})
	if err != nil {
		return nil, err
	}

	ticker, err := m.tickerRepository.Get(trade.TickerID)
	if err != nil {
		return nil, err
	}

	prices, err := m.realAlpacaRepository.GetLatestPrices([]string{ticker.Symbol})
	if err != nil {
		return nil, err
	}
	price := prices[ticker.Symbol]

	return &alpaca.Order{
		FilledAt: internal.TimePointer(time.Now().UTC()),
		// ExpiredAt:      &time.Time{},
		// CanceledAt:     &time.Time{},
		// FailedAt:       &time.Time{},

		// Status: alpaca.Fill,
		// Notional:       &decimal.Decimal{},
		Qty:            &trade.RequestedQuantity,
		FilledQty:      trade.RequestedQuantity,
		FilledAvgPrice: &price,
	}, nil
}

func (m mockAlpacaRepositoryHandler) GetLatestPrices(symbols []string) (map[string]decimal.Decimal, error) {
	return m.realAlpacaRepository.GetLatestPrices(symbols)
}