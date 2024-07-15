package l1_service

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/repository"
	"fmt"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type TradeService interface {
	Buy(input BuyInput) error
	UpdateOrder(tradeOrderID uuid.UUID) error
}

type tradeServiceHandler struct {
	Db                   *sql.DB
	AlpacaRepository     repository.AlpacaRepository
	TradeOrderRepository repository.TradeOrderRepository
}

func NewTradeService(db *sql.DB, alpacaRepository repository.AlpacaRepository, tradeOrderRepository repository.TradeOrderRepository) TradeService {
	return tradeServiceHandler{
		Db:                   db,
		AlpacaRepository:     alpacaRepository,
		TradeOrderRepository: tradeOrderRepository,
	}
}

type BuyInput struct {
	Ticker          model.Ticker
	AmountInDollars decimal.Decimal
	Reason          *string
}

func (h tradeServiceHandler) Buy(input BuyInput) error {
	if input.AmountInDollars.LessThan(decimal.NewFromInt(1)) {
		return fmt.Errorf("failed to submit buy order: amount must be >= 1. got %f", input.AmountInDollars.InexactFloat64())
	}

	tx, err := h.Db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	insertedOrder, err := h.TradeOrderRepository.Add(tx, model.TradeOrder{
		TickerID:                 input.Ticker.TickerID,
		Side:                     model.TradeOrderSide_Buy,
		RequestedAmountInDollars: input.AmountInDollars,
		Status:                   model.TradeOrderStatus_Pending,
		Notes:                    input.Reason,
		FilledQuantity:           decimal.Zero,
	})
	if err != nil {
		return err
	}

	order, err := h.AlpacaRepository.PlaceOrder(repository.AlpacaPlaceOrderRequest{
		TradeOrderID:    insertedOrder.TradeOrderID,
		AmountInDollars: input.AmountInDollars,
		Symbol:          input.Ticker.Symbol,
		Side:            alpaca.Buy,
	})
	if err != nil {
		return err
	}

	orderID, err := uuid.Parse(order.ID)
	if err != nil {
		return err
	}

	// todo - figure out alpaca to db status mapping
	// todo - figure out what alpaca returns for qty/price

	_, err = h.TradeOrderRepository.Update(tx, model.TradeOrder{
		Status:         model.TradeOrderStatus_Pending,
		ProviderID:     &orderID,
		FilledQuantity: order.FilledQty,      // will probably be 0
		FilledPrice:    order.FilledAvgPrice, // will probably be nil
		FilledAt:       order.FilledAt,       // will probably be nil
	}, postgres.ColumnList{
		table.TradeOrder.Status,
		table.TradeOrder.ProviderID,
		table.TradeOrder.FilledQuantity,
		table.TradeOrder.FilledPrice,
	})
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (h tradeServiceHandler) UpdateOrder(tradeOrderID uuid.UUID) error {
	tradeOrder, err := h.TradeOrderRepository.Get(tradeOrderID)
	if err != nil {
		return err
	}
	if tradeOrder.ProviderID != nil {
		return fmt.Errorf("failed to update order: %s has no provider id", tradeOrderID.String())
	}

	order, err := h.AlpacaRepository.GetOrder(*tradeOrder.ProviderID)
	if err != nil {
		return err
	}

	_, err = h.TradeOrderRepository.Update(nil, model.TradeOrder{
		Status:         model.TradeOrderStatus_Pending,
		FilledQuantity: order.FilledQty,
		FilledPrice:    order.FilledAvgPrice,
		FilledAt:       order.FilledAt,
	}, postgres.ColumnList{
		table.TradeOrder.Status,
		table.TradeOrder.FilledQuantity,
		table.TradeOrder.FilledPrice,
	})
	if err != nil {
		return err
	}

	return nil
}
