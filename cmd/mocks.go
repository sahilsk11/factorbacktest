package cmd

import (
	"factorbacktest/internal"
	"factorbacktest/internal/repository"
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
}

func NewMockAlpacaRepository(alpacaRepository repository.AlpacaRepository, toRepository repository.TradeOrderRepository) MockAlpacaRepository {
	return mockAlpacaRepositoryHandler{
		realAlpacaRepository: alpacaRepository,
		tradeOrderRepository: toRepository,
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

	return &alpaca.Order{
		FilledAt: internal.TimePointer(time.Now().UTC()),
		// ExpiredAt:      &time.Time{},
		// CanceledAt:     &time.Time{},
		// FailedAt:       &time.Time{},

		Status: "",
		// Notional:       &decimal.Decimal{},
		Qty:            &trade.RequestedQuantity,
		FilledQty:      trade.RequestedQuantity,
		FilledAvgPrice: internal.DecimalPointer(100),
	}, nil
}

func (m mockAlpacaRepositoryHandler) GetLatestPrices(symbols []string) (map[string]decimal.Decimal, error) {
	return m.realAlpacaRepository.GetLatestPrices(symbols)
}
