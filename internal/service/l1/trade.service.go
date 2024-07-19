package l1_service

import (
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"fmt"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type TradeService interface {
	Buy(input BuyInput) (*model.TradeOrder, error)
	Sell(input SellInput) (*model.TradeOrder, error)
	ExecuteBlock([]*domain.ProposedTrade, uuid.UUID) ([]model.TradeOrder, error)
	UpdateOrder(tradeOrderID uuid.UUID) (*model.TradeOrder, error)
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
	Quantity        decimal.Decimal
	RebalancerRunID uuid.UUID
	Reason          *string
	ExpectedPrice   decimal.Decimal
}

func (h tradeServiceHandler) placeOrder(
	tickerID uuid.UUID,
	symbol string,
	notes *string,
	quantity decimal.Decimal,
	dbSide model.TradeOrderSide,
	alpacaSide alpaca.Side,
	rebalancerRunID uuid.UUID,
	expectedPrice decimal.Decimal,
) (*model.TradeOrder, error) {
	insertedOrder, err := h.TradeOrderRepository.Add(nil, model.TradeOrder{
		TickerID:          tickerID,
		Side:              dbSide,
		RequestedQuantity: quantity,
		ExpectedPrice:     expectedPrice,
		Status:            model.TradeOrderStatus_Error,
		Notes:             notes,
		FilledQuantity:    decimal.Zero,
		RebalancerRunID:   rebalancerRunID,
	})
	if err != nil {
		return nil, err
	}

	// should we include investmentOrder updates here

	order, err := h.AlpacaRepository.PlaceOrder(repository.AlpacaPlaceOrderRequest{
		TradeOrderID: insertedOrder.TradeOrderID,
		Quantity:     quantity,
		Symbol:       symbol,
		Side:         alpacaSide,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute order for trade order %s: %w", insertedOrder.TradeOrderID, err)
	}

	orderID, err := uuid.Parse(order.ID)
	if err != nil {
		return nil, err
	}

	// todo - figure out alpaca to db status mapping
	// todo - figure out what alpaca returns for qty/price

	// don't keep on the same tx because we don't want
	// to roll back and lose the record if this fails
	updatedOrder, err := h.TradeOrderRepository.Update(nil,
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
			table.TradeOrder.FilledAt,
		})
	if err != nil {
		return nil, err
	}

	return updatedOrder, nil
}

type SellInput struct {
	TickerID        uuid.UUID
	Symbol          string
	Quantity        decimal.Decimal
	RebalancerRunID uuid.UUID
	Reason          *string
	ExpectedPrice   decimal.Decimal
}

func (h tradeServiceHandler) Sell(input SellInput) (*model.TradeOrder, error) {
	order, err := h.placeOrder(input.TickerID, input.Symbol, input.Reason, input.Quantity, model.TradeOrderSide_Sell, alpaca.Sell, input.RebalancerRunID, input.ExpectedPrice)
	if err != nil {
		return nil, fmt.Errorf("failed to sell: %w", err)
	}
	return order, nil
}

func (h tradeServiceHandler) Buy(input BuyInput) (*model.TradeOrder, error) {
	order, err := h.placeOrder(input.TickerID, input.Symbol, input.Reason, input.Quantity, model.TradeOrderSide_Buy, alpaca.Buy, input.RebalancerRunID, input.ExpectedPrice)
	if err != nil {
		return nil, fmt.Errorf("failed to buy: %w", err)
	}
	return order, nil
}

func (h tradeServiceHandler) UpdateOrder(tradeOrderID uuid.UUID) (*model.TradeOrder, error) {
	tradeOrder, err := h.TradeOrderRepository.Get(tradeOrderID)
	if err != nil {
		return nil, err
	}
	if tradeOrder.ProviderID == nil {
		return nil, fmt.Errorf("failed to update order: %s has no provider id", tradeOrderID.String())
	}

	order, err := h.AlpacaRepository.GetOrder(*tradeOrder.ProviderID)
	if err != nil {
		return nil, err
	}

	state := tradeOrder.Status
	// check valid state transition
	if state == model.TradeOrderStatus_Pending && order.FilledAt != nil {
		state = model.TradeOrderStatus_Completed
	} else if state == model.TradeOrderStatus_Pending && order.FailedAt != nil {
		state = model.TradeOrderStatus_Error
	}

	updatedTrade, err := h.TradeOrderRepository.Update(nil,
		tradeOrderID,
		model.TradeOrder{
			Status:         state,
			FilledQuantity: order.FilledQty,
			FilledPrice:    order.FilledAvgPrice,
			FilledAt:       order.FilledAt,
		}, postgres.ColumnList{
			table.TradeOrder.Status,
			table.TradeOrder.FilledQuantity,
			table.TradeOrder.FilledPrice,
			table.TradeOrder.FilledAt,
		})
	if err != nil {
		return nil, err
	}

	return updatedTrade, nil
}

// assumes trades are already aggregated by symbol
func (h tradeServiceHandler) ExecuteBlock(trades []*domain.ProposedTrade, rebalancerRunID uuid.UUID) ([]model.TradeOrder, error) {
	// TODO - should we still store the trade order if it failed,
	// but give it status failed? i think that will be easier to
	// look up later and understand what happened instead of
	// leaving the col null in investmentTrade

	// first ensure that we have enough quantity for the order
	currentHoldings, err := h.AlpacaRepository.GetPositions()
	if err != nil {
		return nil, err
	}
	for _, t := range trades {
		if t.ExactQuantity.LessThan(decimal.Zero) {
			for _, position := range currentHoldings {
				// if we hold less of the symbol than we want to sell, error
				if t.Symbol == position.Symbol && (position.Qty.LessThan(t.ExactQuantity) ||
					position.QtyAvailable.LessThan(t.ExactQuantity)) {
					return nil, fmt.Errorf("insufficient %s (%f) to sell %f", t.Symbol, position.QtyAvailable.InexactFloat64(), t.ExactQuantity.InexactFloat64())
				}
			}
		}
	}

	// maybe check buying power

	generatedOrders := []model.TradeOrder{}

	// do a simple two pass to run all trades first
	for _, t := range trades {
		if t.ExactQuantity.LessThan(decimal.Zero) {
			order, err := h.Sell(SellInput{
				TickerID:        t.TickerID,
				Symbol:          t.Symbol,
				Quantity:        t.ExactQuantity.Abs(),
				ExpectedPrice:   t.ExpectedPrice,
				RebalancerRunID: rebalancerRunID,
			})
			if err != nil {
				return generatedOrders, err
			}
			generatedOrders = append(generatedOrders, *order)
		}
	}

	for _, t := range trades {
		if t.ExactQuantity.GreaterThan(decimal.Zero) {
			order, err := h.Buy(BuyInput{
				TickerID:        t.TickerID,
				Symbol:          t.Symbol,
				Quantity:        t.ExactQuantity,
				ExpectedPrice:   t.ExpectedPrice,
				RebalancerRunID: rebalancerRunID,
			})
			if err != nil {
				return generatedOrders, err
			}
			generatedOrders = append(generatedOrders, *order)
		}
	}

	return generatedOrders, nil
}
