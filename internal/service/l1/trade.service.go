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
	Sell(input SellInput) error
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
	TickerID        uuid.UUID
	Symbol          string
	AmountInDollars decimal.Decimal
	Reason          *string
}

func (h tradeServiceHandler) placeOrder(
	tickerID uuid.UUID,
	symbol string,
	notes *string,
	amountInDollars decimal.Decimal,
	dbSide model.TradeOrderSide,
	alpacaSide alpaca.Side,
) error {
	tx, err := h.Db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	insertedOrder, err := h.TradeOrderRepository.Add(tx, model.TradeOrder{
		TickerID:                 tickerID,
		Side:                     dbSide,
		RequestedAmountInDollars: amountInDollars,
		Status:                   model.TradeOrderStatus_Pending,
		Notes:                    notes,
		FilledQuantity:           decimal.Zero,
	})
	if err != nil {
		return err
	}

	order, err := h.AlpacaRepository.PlaceOrder(repository.AlpacaPlaceOrderRequest{
		TradeOrderID:    insertedOrder.TradeOrderID,
		AmountInDollars: amountInDollars,
		Symbol:          symbol,
		Side:            alpacaSide,
	})
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	orderID, err := uuid.Parse(order.ID)
	if err != nil {
		return err
	}

	// todo - figure out alpaca to db status mapping
	// todo - figure out what alpaca returns for qty/price

	// don't keep on the same tx because we don't want
	// to roll back and lose the record if this fails
	_, err = h.TradeOrderRepository.Update(nil,
		insertedOrder.TradeOrderID,
		model.TradeOrder{
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

	return nil
}

type SellInput struct {
	TickerID        uuid.UUID
	Symbol          string
	AmountInDollars decimal.Decimal
	Reason          *string
}

func (h tradeServiceHandler) Sell(input SellInput) error {
	if err := h.placeOrder(input.TickerID, input.Symbol, input.Reason, input.AmountInDollars, model.TradeOrderSide_Sell, alpaca.Sell); err != nil {
		return fmt.Errorf("failed to sell: %w", err)
	}
	return nil
}

func (h tradeServiceHandler) Buy(input BuyInput) error {
	if input.AmountInDollars.LessThan(decimal.NewFromInt(1)) {
		return fmt.Errorf("failed to submit buy order: amount must be >= 1. got %f", input.AmountInDollars.InexactFloat64())
	}

	if err := h.placeOrder(input.TickerID, input.Symbol, input.Reason, input.AmountInDollars, model.TradeOrderSide_Buy, alpaca.Buy); err != nil {
		return fmt.Errorf("failed to buy: %w", err)
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

	_, err = h.TradeOrderRepository.Update(nil,
		tradeOrderID,
		model.TradeOrder{
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
