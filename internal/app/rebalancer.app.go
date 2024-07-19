package app

import (
	"context"
	"database/sql"
	"factorbacktest/internal"
	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	l1_service "factorbacktest/internal/service/l1"
	l3_service "factorbacktest/internal/service/l3"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type RebalancerHandler struct {
	Db                        *sql.DB
	InvestmentService         l3_service.InvestmentService
	TradingService            l1_service.TradeService
	RebalancerRunRepository   repository.RebalancerRunRepository
	TickerRepository          repository.TickerRepository
	InvestmentTradeRepository repository.InvestmentTradeRepository
	HoldingsRepository        repository.InvestmentHoldingsRepository
	AlpacaRepository          repository.AlpacaRepository
}

// Rebalance retrieves the latest proposed trades for the aggregate
// trading account, then calls the trading service to execute them
// Trade execution is non-blocking, so we will need to kick off an
// event that checks status after submission
//
// TODO - add some sort of reconciliation that figures out what
// everything got executed at.
// TODO - add idempotency around runs and somehow invalidate any
// old runs
func (h RebalancerHandler) Rebalance(ctx context.Context) error {
	date := time.Now().UTC()

	// get all assets
	// we could probably clean this up
	// by getting assets on the fly idk
	assets, err := h.TickerRepository.List()
	if err != nil {
		return err
	}
	symbols := []string{}
	tickerIDMap := map[string]uuid.UUID{}
	for _, s := range assets {
		if s.Symbol != ":CASH" {
			symbols = append(symbols, s.Symbol)
			tickerIDMap[s.Symbol] = s.TickerID
		}
	}
	pm, err := h.AlpacaRepository.GetLatestPrices(symbols)
	if err != nil {
		return fmt.Errorf("failed to get latest prices: %w", err)
	}

	rebalancerRun, err := h.RebalancerRunRepository.Add(nil, model.RebalancerRun{
		Date:               date,
		RebalancerRunType:  model.RebalancerRunType_ManualInvestmentRebalance,
		RebalancerRunState: model.RebalancerRunState_Error,
	})
	if err != nil {
		return err
	}

	// note - assumes everything is due for rebalance when run, i.e. rebalances everything
	investmentsToRebalance, err := h.InvestmentService.ListForRebalance()
	if err != nil {
		return err
	}

	proposedTrades := []*domain.ProposedTrade{}
	investmentTrades := []*model.InvestmentTrade{}
	// keyed by investment id
	mappedPortfolios := map[uuid.UUID]*domain.Portfolio{}

	for _, investment := range investmentsToRebalance {
		portfolio, trades, err := h.InvestmentService.GenerateRebalanceResults(
			ctx,
			investment,
			rebalancerRun.Date,
			pm,
			tickerIDMap,
		)
		if err != nil {
			return fmt.Errorf("failed to rebalance: failed to generate results for investment %s: %w", investment.InvestmentID.String(), err)
		}

		mappedPortfolios[investment.InvestmentID] = portfolio

		proposedTrades = append(proposedTrades, trades...)
		investmentTrades = append(investmentTrades,
			proposedTradesToInvestmentTradeModels(
				proposedTrades,
				investment.InvestmentID,
				rebalancerRun.RebalancerRunID,
			)...)
	}

	tx, err := h.Db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	insertedInvestmentTrades, err := h.InvestmentTradeRepository.AddMany(tx, investmentTrades)
	if err != nil {
		return err
	}

	// any errors from here cannot easily be rolled back
	rebalancerRun.RebalancerRunState = model.RebalancerRunState_Pending
	_, err = h.RebalancerRunRepository.Update(tx, rebalancerRun, []postgres.Column{
		table.RebalancerRun.RebalancerRunState,
	})
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	// until we have some fancier math for reconciling completed trades,
	// treat any failure here as fatal
	// TODO - improve reconciliation + partial trade completion
	executedTrades, tradeExecutionErr := h.aggregateAndExecuteTradeOrders(proposedTrades, rebalancerRun.RebalancerRunID)

	updateInvesmtentTradeErrors := []error{}
	for _, tradeOrder := range executedTrades {
		for _, investmentTrade := range insertedInvestmentTrades {
			if tradeOrder.TickerID == investmentTrade.TickerID {
				investmentTrade.TradeOrderID = &tradeOrder.TradeOrderID
				_, err = h.InvestmentTradeRepository.Update(
					nil,
					investmentTrade,
					[]postgres.Column{
						table.InvestmentTrade.TradeOrderID,
					},
				)
				if err != nil {
					updateInvesmtentTradeErrors = append(updateInvesmtentTradeErrors, err)
				}
			}
		}
	}

	if len(updateInvesmtentTradeErrors) > 0 && tradeExecutionErr != nil {
		return fmt.Errorf("failed to execute trades AND update %d investment trade status. trade err: %w | first update err: %w", len(updateInvesmtentTradeErrors), tradeExecutionErr, updateInvesmtentTradeErrors[0])
	}
	if tradeExecutionErr != nil {
		return fmt.Errorf("failure on executing orders for rebalance run %s: %w\n", rebalancerRun.RebalancerRunID.String(), tradeExecutionErr)
	}
	if len(updateInvesmtentTradeErrors) > 0 {
		return fmt.Errorf("failed to update %d investment trade status. first update err: %w", len(updateInvesmtentTradeErrors), updateInvesmtentTradeErrors[0])
	}

	// before we update holdings, we need trades to settle
	// so let's leave the run as pending and check back
	// later
	return nil

	// update positions to match whatever we just traded to get
	// them to
	// todo - when we improve reconciliation, handle partial failures
	// from this

	// okay seriously use trades to update holdings, not
	// target portfolios. if the trades fail, we need to know
	// exactly what is being held still
	// _, err = h.updateHoldings(mappedPortfolios, executedInvestmentTrade, rebalancerRun)
	// if err != nil {
	// 	return err
	// }

	// return nil
}

// func (h RebalancerHandler) updateHoldings(
// 	mappedPortfolios map[uuid.UUID]*domain.Portfolio, executedInvestmentTrades map[uuid.UUID][]*model.InvestmentTrade,
// 	rebalancerRun *model.RebalancerRun,
// ) (map[uuid.UUID][]error, error) {
// 	cashTicker, err := h.TickerRepository.GetCashTicker()
// 	if err != nil {
// 		return nil, err
// 	}

// 	for investmentID, executedInvestmentTrades := range executedInvestmentTrades {
// 		currentHoldings, err := h.HoldingsRepository.GetLatestHoldings(investmentID)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to get holdings from investment id %s: %w", investmentID.String(), err)
// 		}
// 		newPortfolio := currentHoldings.DeepCopy()
// 		for _, trade := range executedInvestmentTrades {
// 			if trade.Side == model.TradeOrderSide_Sell {
// 				newPortfolio.Cash = newPortfolio.Cash.Add(
// 					// how do we do this
// 					trade.Quantity.Mul(trade.Price),
// 				)
// 			}
// 		}
// 	}

// 	errorsByInvestment := map[uuid.UUID][]error{}
// 	for investmentID, portfolio := range mappedPortfolios {
// 		for _, position := range portfolio.Positions {
// 			_, err = h.HoldingsRepository.Add(nil, model.InvestmentHoldings{
// 				InvestmentID:    investmentID,
// 				TickerID:        position.TickerID,
// 				Quantity:        position.ExactQuantity,
// 				RebalancerRunID: rebalancerRun.RebalancerRunID,
// 			})
// 			if err != nil {
// 				errorsByInvestment[investmentID] = append(errorsByInvestment[investmentID], err)
// 			}
// 		}

// 		if portfolio.Cash.GreaterThan(decimal.Zero) {
// 			_, err = h.HoldingsRepository.Add(nil, model.InvestmentHoldings{
// 				InvestmentID:    investmentID,
// 				TickerID:        cashTicker.TickerID,
// 				Quantity:        portfolio.Cash,
// 				RebalancerRunID: rebalancerRun.RebalancerRunID,
// 			})
// 			if err != nil {
// 				errorsByInvestment[investmentID] = append(errorsByInvestment[investmentID], err)
// 			}
// 		}
// 	}

// 	return errorsByInvestment, nil
// }

func proposedTradesToInvestmentTradeModels(trades []*domain.ProposedTrade, investmentID, rebalancerRunID uuid.UUID) []*model.InvestmentTrade {
	out := []*model.InvestmentTrade{}
	for _, t := range trades {
		side := model.TradeOrderSide_Buy
		if t.ExactQuantity.LessThan(decimal.Zero) {
			side = model.TradeOrderSide_Sell
		}
		out = append(out, &model.InvestmentTrade{
			TickerID:        t.TickerID,
			Side:            side,
			InvestmentID:    investmentID,
			RebalancerRunID: rebalancerRunID,
			Quantity:        t.ExactQuantity,
			TradeOrderID:    nil, // need to update and set this
		})
	}
	return out
}

func (h RebalancerHandler) aggregateAndExecuteTradeOrders(proposedTrades []*domain.ProposedTrade, rebalancerRunID uuid.UUID) ([]model.TradeOrder, error) {
	aggregatedTrades := l3_service.AggregateAndFormatTrades(proposedTrades)
	internal.Pprint(aggregatedTrades)

	return h.TradingService.ExecuteBlock(aggregatedTrades, rebalancerRunID)
}
