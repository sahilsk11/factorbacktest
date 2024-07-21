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
	TradeOrderRepository      repository.TradeOrderRepository
	HoldingsVersionRepository repository.InvestmentHoldingsVersionRepository
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

	// note - assumes everything is due for rebalance when run, i.e. rebalances everything
	investmentsToRebalance, err := h.InvestmentService.ListForRebalance()
	if err != nil {
		return err
	}

	rebalancerRun, err := h.RebalancerRunRepository.Add(nil, model.RebalancerRun{
		Date:                    date,
		RebalancerRunType:       model.RebalancerRunType_ManualInvestmentRebalance,
		RebalancerRunState:      model.RebalancerRunState_Error,
		NumInvestmentsAttempted: int32(len(investmentsToRebalance)),
	})
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

	rebalancerRun.RebalancerRunState = model.RebalancerRunState_Pending
	if len(investmentsToRebalance) == 0 {
		rebalancerRun.RebalancerRunState = model.RebalancerRunState_Completed
		rebalancerRun.Notes = internal.StringPointer("no investments to rebalance")
	} else if len(insertedInvestmentTrades) == 0 {
		rebalancerRun.RebalancerRunState = model.RebalancerRunState_Completed
		rebalancerRun.Notes = internal.StringPointer("no investment trades generated")
	}

	_, err = h.RebalancerRunRepository.Update(tx, rebalancerRun, []postgres.Column{
		table.RebalancerRun.RebalancerRunState,
		table.RebalancerRun.Notes,
	})
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	if len(insertedInvestmentTrades) == 0 {
		return nil
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

	if len(executedTrades) == 0 {
		rebalancerRun.RebalancerRunState = model.RebalancerRunState_Completed
		rebalancerRun.Notes = internal.StringPointer("no trade orders generated - investment trades must have cancelled out")
		_, err = h.RebalancerRunRepository.Update(nil, rebalancerRun, []postgres.Column{
			table.RebalancerRun.RebalancerRunState,
			table.RebalancerRun.Notes,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

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

	return h.TradingService.ExecuteBlock(aggregatedTrades, rebalancerRunID)
}

func (h RebalancerHandler) UpdateAllPendingOrders() error {
	cashTicker, err := h.TickerRepository.GetCashTicker()
	if err != nil {
		return err
	}

	trades, err := h.TradeOrderRepository.List()
	if err != nil {
		return err
	}

	tx, err := h.Db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
		ReadOnly:  false,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rebalancerRuns := []uuid.UUID{}

	completedTrades := []model.InvestmentTradeStatus{}
	for _, trade := range trades {
		if trade.Status == model.TradeOrderStatus_Pending {
			updatedTrade, err := h.TradingService.UpdateOrder(tx, trade.TradeOrderID)
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

	completedTradesByInvestment := map[uuid.UUID][]model.InvestmentTradeStatus{}
	for _, t := range completedTrades {
		if _, ok := completedTradesByInvestment[*t.InvestmentID]; !ok {
			completedTradesByInvestment[*t.InvestmentID] = []model.InvestmentTradeStatus{}
		}
		completedTradesByInvestment[*t.InvestmentID] = append(completedTradesByInvestment[*t.InvestmentID], t)
	}

	for investmentID, newTrades := range completedTradesByInvestment {
		// should be the holdings prior to the new trades being completed
		currentHoldings, err := h.HoldingsRepository.GetLatestHoldings(tx, investmentID)
		if err != nil {
			return err
		}
		newPortfolio := currentHoldings.DeepCopy()
		for _, t := range newTrades {
			oldQuantity := decimal.Zero
			if p, ok := newPortfolio.Positions[*t.Symbol]; ok {
				oldQuantity = p.ExactQuantity
			} else {
				newPortfolio.Positions[*t.Symbol] = &domain.Position{
					Symbol:        *t.Symbol,
					Quantity:      0,
					ExactQuantity: decimal.Zero,
					TickerID:      *t.TickerID,
				}
			}
			orderQuantity := *t.Quantity
			orderPrice := *t.FilledPrice

			if *t.Side == model.TradeOrderSide_Sell {
				newPortfolio.Positions[*t.Symbol].ExactQuantity = oldQuantity.Sub(orderQuantity)
				newPortfolio.SetCash(newPortfolio.Cash.Add(orderQuantity.Mul(orderPrice)))
			} else {
				newPortfolio.Positions[*t.Symbol].ExactQuantity = oldQuantity.Add(orderQuantity)
				newPortfolio.SetCash(newPortfolio.Cash.Sub(orderQuantity.Mul(orderPrice)))
			}
		}

		// validate the portfolio
		// - ensure cash >= 0
		// - ensure position quantity >= 0
		// ensure allocations line up with expected

		version, err := h.HoldingsVersionRepository.Add(tx, model.InvestmentHoldingsVersion{
			InvestmentID: investmentID,
		})
		if err != nil {
			return err
		}

		for _, position := range newPortfolio.Positions {
			_, err = h.HoldingsRepository.Add(tx, model.InvestmentHoldings{
				InvestmentID:                investmentID,
				TickerID:                    position.TickerID,
				Quantity:                    position.ExactQuantity,
				InvestmentHoldingsVersionID: version.InvestmentHoldingsVersionID,
			})
			if err != nil {
				return err
			}
		}

		// record even small slippage in cash, so we know
		// their actual account balances
		if newPortfolio.Cash.Abs().GreaterThan(decimal.Zero) {
			_, err = h.HoldingsRepository.Add(tx, model.InvestmentHoldings{
				InvestmentID:                investmentID,
				TickerID:                    cashTicker.TickerID,
				Quantity:                    *newPortfolio.Cash,
				InvestmentHoldingsVersionID: version.InvestmentHoldingsVersionID,
			})
			if err != nil {
				return err
			}
		}
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
			if *t.Status != model.TradeOrderStatus_Completed {
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
