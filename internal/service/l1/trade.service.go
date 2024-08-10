package l1_service

import (
	"context"
	"database/sql"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"
	"factorbacktest/internal/repository"
	"fmt"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Responsible for executing trades, including
// aggregation and ensuring it meets Alpaca's spec
type TradeService interface {
	Buy(input BuyInput) (*model.TradeOrder, error)
	Sell(input SellInput) (*model.TradeOrder, error)
	ExecuteBlock(context.Context, []*domain.ProposedTrade, uuid.UUID) ([]model.TradeOrder, error)
	UpdateAllPendingOrders(ctx context.Context) error
}

type tradeServiceHandler struct {
	Db                          *sql.DB
	AlpacaRepository            repository.AlpacaRepository
	TradeOrderRepository        repository.TradeOrderRepository
	TickerRepository            repository.TickerRepository
	InvestmentTradeRepository   repository.InvestmentTradeRepository
	HoldingsRepository          repository.InvestmentHoldingsRepository
	HoldingsVersionRepository   repository.InvestmentHoldingsVersionRepository
	RebalancerRunRepository     repository.RebalancerRunRepository
	ExcessTradeVolumeRepository repository.ExcessTradeVolumeRepository
}

func NewTradeService(
	db *sql.DB,
	alpacaRepository repository.AlpacaRepository,
	tradeOrderRepository repository.TradeOrderRepository,
	tickerRepository repository.TickerRepository,
	itRepository repository.InvestmentTradeRepository,
	holdingsRepository repository.InvestmentHoldingsRepository,
	holdingsVersionRepository repository.InvestmentHoldingsVersionRepository,
	RebalancerRunRepository repository.RebalancerRunRepository,
	excessTradeVolumeRepository repository.ExcessTradeVolumeRepository,
) TradeService {
	return tradeServiceHandler{
		Db:                          db,
		AlpacaRepository:            alpacaRepository,
		TradeOrderRepository:        tradeOrderRepository,
		TickerRepository:            tickerRepository,
		InvestmentTradeRepository:   itRepository,
		HoldingsRepository:          holdingsRepository,
		HoldingsVersionRepository:   holdingsVersionRepository,
		RebalancerRunRepository:     RebalancerRunRepository,
		ExcessTradeVolumeRepository: excessTradeVolumeRepository,
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

	limit := expectedPrice.Mul(decimal.NewFromFloat(1.05))
	if alpacaSide == alpaca.Sell {
		limit = expectedPrice.Mul(decimal.NewFromFloat(0.95))
	}
	limit = limit.Round(2)
	order, err := h.AlpacaRepository.PlaceOrder(repository.AlpacaPlaceOrderRequest{
		TradeOrderID: insertedOrder.TradeOrderID,
		Quantity:     quantity,
		Symbol:       symbol,
		Side:         alpacaSide,
		LimitPrice:   &limit,
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

// coalesces trades by symbol and ensures nominal amount > $2
// for Alpaca's min order rule
func aggregateAndFormatTrades(ctx context.Context, trades []*domain.ProposedTrade) ([]*domain.ProposedTrade, map[uuid.UUID]decimal.Decimal) {
	log := logger.FromContext(ctx)
	// Map to hold aggregated trades by symbol
	aggregatedTrades := make(map[string]*domain.ProposedTrade)

	// Aggregate trades by symbol
	for _, trade := range trades {
		if existingTrade, exists := aggregatedTrades[trade.Symbol]; exists {
			// Update the existing trade quantity
			existingTrade.ExactQuantity = existingTrade.ExactQuantity.Add(trade.ExactQuantity)
			aggregatedTrades[trade.Symbol] = existingTrade
		} else {
			// Add a new trade to the map
			aggregatedTrades[trade.Symbol] = trade
		}
	}

	// Create a slice to hold the formatted trades
	var result []*domain.ProposedTrade
	for _, trade := range aggregatedTrades {
		if !trade.ExactQuantity.IsZero() {
			result = append(result, trade)
		}
	}

	// alpaca requires orders to be >= $1
	minOrderSize := decimal.NewFromFloat(1.5)

	// we could round all trades up to $1 but
	// if they have tons of little trades, that
	// could get expensive
	// round all buy orders to $1
	// TODO - i think we should use market value
	// and figure out whether to round up or down
	// also since price is stale, it could be just under $1
	// also we need to ledger these somewhere, as excess that
	// I own

	excessQuantities := map[uuid.UUID]decimal.Decimal{}
	totalExcessAmount := decimal.Zero
	for _, t := range trades {
		if t.ExactQuantity.GreaterThan(decimal.Zero) && t.ExactQuantity.Mul(t.ExpectedPrice).LessThan(minOrderSize) {
			newQuantity := minOrderSize.Div(t.ExpectedPrice)
			excessQuantity := newQuantity.Sub(t.ExactQuantity)
			excessQuantities[t.TickerID] = excessQuantity

			t.ExactQuantity = newQuantity
			totalExcessAmount = totalExcessAmount.Add(excessQuantity.Mul(t.ExpectedPrice))
		}
	}

	log.Infof("total excess amount: %f. breakdown %v", totalExcessAmount.InexactFloat64(), excessQuantities)

	// if totalExcess.GreaterThan(decimal.NewFromInt(10)) {
	// 	return nil, fmt.Errorf("excess amount exceeds $10: calculated %f", totalExcess.InexactFloat64())
	// }

	return result, excessQuantities
}

// assumes trades are already aggregated by symbol
func (h tradeServiceHandler) ExecuteBlock(ctx context.Context, rawTrades []*domain.ProposedTrade, rebalancerRunID uuid.UUID) ([]model.TradeOrder, error) {
	// TODO - should we still store the trade order if it failed,
	// but give it status failed? i think that will be easier to
	// look up later and understand what happened instead of
	// leaving the col null in investmentTrade

	trades, excessQuantities := aggregateAndFormatTrades(ctx, rawTrades)

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

	// do a simple two pass to run all sells first
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
			// only our buy orders should generate excess
			tx, err := h.Db.Begin()
			if err != nil {
				return generatedOrders, err
			}
			defer tx.Rollback()

			var excessModel *model.ExcessTradeVolume
			if excessQuantity, ok := excessQuantities[t.TickerID]; ok {
				excessModel, err = h.ExcessTradeVolumeRepository.Add(tx, model.ExcessTradeVolume{
					TickerID:        t.TickerID,
					Quantity:        excessQuantity,
					RebalancerRunID: rebalancerRunID,
				})
				if err != nil {
					return generatedOrders, err
				}
			}

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
			if excessModel != nil {
				excessModel.TradeOrderID = &order.TradeOrderID
				_, err = h.ExcessTradeVolumeRepository.Update(tx, *excessModel, postgres.ColumnList{
					table.ExcessTradeVolume.TradeOrderID,
				})
				if err != nil {
					return generatedOrders, err
				}
			}
			err = tx.Commit()
			if err != nil {
				return generatedOrders, err
			}

			generatedOrders = append(generatedOrders, *order)
		}
	}

	return generatedOrders, nil
}

// not safe to call in isolation - needs to update rebalancer run
// status and update holdings
func (h tradeServiceHandler) updateOrder(tx *sql.Tx, tradeOrderID uuid.UUID) (*model.TradeOrder, error) {
	tradeOrder, err := h.TradeOrderRepository.Get(repository.TradeOrderGetFilter{
		TradeOrderID: &tradeOrderID,
	})
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

	// todo - should we check order.Status

	state := tradeOrder.Status
	// check valid state transition
	if order.Status == "expired" || order.Status == "cancelled" {
		state = model.TradeOrderStatus_Canceled
	} else if state == model.TradeOrderStatus_Pending && order.FilledAt != nil {
		state = model.TradeOrderStatus_Completed
	} else if state == model.TradeOrderStatus_Pending && order.FailedAt != nil {
		state = model.TradeOrderStatus_Error
	}

	updatedTrade, err := h.TradeOrderRepository.Update(tx,
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

func (h tradeServiceHandler) UpdateAllPendingOrders(ctx context.Context) error {
	cashTicker, err := h.TickerRepository.GetCashTicker()
	if err != nil {
		return err
	}

	trades, err := h.TradeOrderRepository.List()
	if err != nil {
		return err
	}

	tx, err := h.Db.Begin() // this used to be level: read uncommitted. if it fails, revert
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rebalancerRuns := []uuid.UUID{}

	completedTrades := []*model.InvestmentTradeStatus{}
	for _, trade := range trades {
		if trade.Status == model.TradeOrderStatus_Pending {
			updatedTrade, err := h.updateOrder(tx, trade.TradeOrderID)
			if err != nil {
				return err
			}
			if updatedTrade.Status == model.TradeOrderStatus_Completed {
				relevantInvestmentTrades, err := h.InvestmentTradeRepository.List(tx, repository.InvestmentTradeListFilter{
					TradeOrderID: &updatedTrade.TradeOrderID,
				})
				if err != nil {
					return err
				}
				rebalancerRuns = append(rebalancerRuns, updatedTrade.RebalancerRunID)
				completedTrades = append(completedTrades, relevantInvestmentTrades...)
			}
		}
	}

	completedTradesByInvestment := map[uuid.UUID][]*model.InvestmentTradeStatus{}
	for _, t := range completedTrades {
		if _, ok := completedTradesByInvestment[*t.InvestmentID]; !ok {
			completedTradesByInvestment[*t.InvestmentID] = []*model.InvestmentTradeStatus{}
		}
		completedTradesByInvestment[*t.InvestmentID] = append(completedTradesByInvestment[*t.InvestmentID], t)
	}

	err = h.updatePortfoliosFromTrades(tx, completedTradesByInvestment, cashTicker.TickerID)
	if err != nil {
		return err
	}

	for _, rebalancerRunID := range rebalancerRuns {
		relevantInvestmentTrades, err := h.InvestmentTradeRepository.List(tx, repository.InvestmentTradeListFilter{
			RebalancerRunID: &rebalancerRunID,
		})
		if err != nil {
			return err
		}
		allCompleted := true
		for _, t := range relevantInvestmentTrades {
			if t.Status == nil || *t.Status != model.TradeOrderStatus_Completed {
				allCompleted = false
			}
		}
		if allCompleted {
			_, err = h.RebalancerRunRepository.Update(tx, &model.RebalancerRun{
				RebalancerRunID:    rebalancerRunID,
				RebalancerRunState: model.RebalancerRunState_Completed,
			}, []postgres.Column{
				table.RebalancerRun.RebalancerRunState,
			})
			if err != nil {
				return err
			}
		}
	}

	// todo - update holdings from these trades
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func AddTradesToPortfolio(trades []*model.InvestmentTradeStatus, portfolio *domain.Portfolio) *domain.Portfolio {
	// ensure we don't override given portfolio
	portfolio = portfolio.DeepCopy()

	for _, t := range trades {
		oldQuantity := decimal.Zero
		if p, ok := portfolio.Positions[*t.Symbol]; ok {
			oldQuantity = p.ExactQuantity
		} else {
			portfolio.Positions[*t.Symbol] = &domain.Position{
				Symbol:        *t.Symbol,
				Quantity:      0,
				ExactQuantity: decimal.Zero,
				TickerID:      *t.TickerID,
			}
		}
		orderQuantity := *t.Quantity
		orderPrice := *t.FilledPrice

		if *t.Side == model.TradeOrderSide_Sell {
			portfolio.Positions[*t.Symbol].ExactQuantity = oldQuantity.Sub(orderQuantity)
			portfolio.SetCash(portfolio.Cash.Add(orderQuantity.Mul(orderPrice)))
			if portfolio.Positions[*t.Symbol].ExactQuantity.Equal(decimal.Zero) {
				delete(portfolio.Positions, *t.Symbol)
			}
		} else {
			portfolio.Positions[*t.Symbol].ExactQuantity = oldQuantity.Add(orderQuantity)
			portfolio.SetCash(portfolio.Cash.Sub(orderQuantity.Mul(orderPrice)))
		}
	}

	return portfolio
}

func (h tradeServiceHandler) updatePortfoliosFromTrades(tx *sql.Tx, completedTradesByInvestment map[uuid.UUID][]*model.InvestmentTradeStatus, cashTickerID uuid.UUID) error {
	for investmentID, newTrades := range completedTradesByInvestment {
		if len(newTrades) == 0 {
			// i think
			continue
		}
		// should be the holdings prior to the new trades being completed
		currentHoldings, err := h.HoldingsRepository.GetLatestHoldings(tx, investmentID)
		if err != nil {
			return err
		}
		newPortfolio := AddTradesToPortfolio(newTrades, currentHoldings)

		rebalancerRunID := newTrades[0].RebalancerRunID

		for _, t := range newTrades {
			if t.RebalancerRunID == nil {
				return fmt.Errorf("failed to update portfolio: investment trade %s missing rebalancer run id", t.InvestmentTradeID)
			}
			if *t.RebalancerRunID != *rebalancerRunID {
				return fmt.Errorf("expected rebalancer run id %s, got %s", rebalancerRunID.String(), t.RebalancerRunID.String())
			}
		}

		// validate the portfolio
		// - ensure cash >= 0
		// - ensure position quantity >= 0
		// ensure allocations line up with expected

		version, err := h.HoldingsVersionRepository.Add(tx, model.InvestmentHoldingsVersion{
			InvestmentID:    investmentID,
			RebalancerRunID: rebalancerRunID,
		})
		if err != nil {
			return err
		}

		for _, position := range newPortfolio.Positions {
			if !position.ExactQuantity.Equal(decimal.Zero) {
				_, err = h.HoldingsRepository.Add(tx, model.InvestmentHoldings{
					TickerID:                    position.TickerID,
					Quantity:                    position.ExactQuantity,
					InvestmentHoldingsVersionID: version.InvestmentHoldingsVersionID,
				})
				if err != nil {
					return err
				}
			}
		}

		_, err = h.HoldingsRepository.Add(tx, model.InvestmentHoldings{
			TickerID:                    cashTickerID,
			Quantity:                    *newPortfolio.Cash,
			InvestmentHoldingsVersionID: version.InvestmentHoldingsVersionID,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
