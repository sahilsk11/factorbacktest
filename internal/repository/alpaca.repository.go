package repository

import (
	"fmt"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type AlpacaRepository interface {
	PlaceOrder(req AlpacaPlaceOrderRequest) (*alpaca.Order, error)
	CancelOpenOrders() error
	GetPositions() ([]alpaca.Position, error)
	IsMarketOpen() (bool, error)
	GetAccount() (*alpaca.Account, error)
	GetOrder(alpacaOrderID uuid.UUID) (*alpaca.Order, error)
}

func NewAlpacaRepository(apiKey, apiSecret string) AlpacaRepository {
	client := alpaca.NewClient(alpaca.ClientOpts{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		BaseURL:    "https://paper-api.alpaca.markets",
		RetryLimit: 3,
	})

	// todo - verify key

	return &alpacaRepositoryHandler{
		Client: client,
	}
}

type alpacaRepositoryHandler struct {
	Client *alpaca.Client
}

func (h alpacaRepositoryHandler) CancelOpenOrders() error {
	orders, err := h.Client.GetOrders(alpaca.GetOrdersRequest{
		Status: "open",
		Until:  time.Now(),
		Limit:  100,
	})
	if err != nil {
		return fmt.Errorf("Failed to list orders: %w", err)
	}
	for _, order := range orders {
		if err := h.Client.CancelOrder(order.ID); err != nil {
			return fmt.Errorf("Failed to cancel order %s: %w", order.ID, err)
		}
	}

	fmt.Printf("%d order(s) cancelled\n", len(orders))
	return nil
}

func (h alpacaRepositoryHandler) GetPositions() ([]alpaca.Position, error) {
	positions, err := h.Client.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("get positions: %w", err)
	}
	return positions, nil
}

func (h alpacaRepositoryHandler) IsMarketOpen() (bool, error) {
	clock, err := h.Client.GetClock()
	if err != nil {
		return false, err
	}
	return clock.IsOpen, nil
}

func (h alpacaRepositoryHandler) GetAccount() (*alpaca.Account, error) {
	acct, err := h.Client.GetAccount()
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	return acct, nil
}

type AlpacaPlaceOrderRequest struct {
	TradeOrderID uuid.UUID
	Quantity     decimal.Decimal
	Symbol       string
	Side         alpaca.Side
}

func (a AlpacaPlaceOrderRequest) isValid() error {
	if a.Quantity.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("amount is <= 0, order of | %s %s| not sent\n", a.Quantity.String(), a.Side)
	}
	return nil
}

func (h alpacaRepositoryHandler) GetOrder(alpacaOrderID uuid.UUID) (*alpaca.Order, error) {
	return h.Client.GetOrder(alpacaOrderID.String())
}

func (h alpacaRepositoryHandler) PlaceOrder(req AlpacaPlaceOrderRequest) (*alpaca.Order, error) {
	if err := req.isValid(); err != nil {
		return nil, fmt.Errorf("invalid input to alpaca submit order on trade order %s: %w", req.TradeOrderID.String(), err)
	}

	order, err := h.Client.PlaceOrder(alpaca.PlaceOrderRequest{
		Symbol:        req.Symbol,
		Qty:           &req.Quantity,
		Side:          req.Side,
		Type:          alpaca.Market,
		TimeInForce:   alpaca.Day,
		ClientOrderID: req.TradeOrderID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("Order for trade request id %s failed: %w", req.TradeOrderID.String(), err)
	}

	return order, nil
}
