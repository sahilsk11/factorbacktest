package service

import (
	"context"
	"fmt"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/logger"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
)

// rebalancePlanEntry is one investment's committed rebalance attempt.
//
// Data model (relevant tables):
//   - rebalancer_run: one cron invocation
//   - investment_rebalance: per-investment plan for that run (state PENDING → ERROR/COMPLETED)
//   - investment_trade: proposed legs for that plan (trade_order_id links to outcome)
//   - trade_order: broker outcome (PENDING/COMPLETED/ERROR)
//   - investment: error_at blocks future cron until ops clears it
type rebalancePlanEntry struct {
	InvestmentID          uuid.UUID
	InvestmentRebalanceID uuid.UUID
	ProposedTrades        []*domain.ProposedTrade
	InvestmentTrades      []model.InvestmentTrade
}

type rebalanceExecutionResult struct {
	ExecutedOrders []model.TradeOrder
	Err            error
}

func (h investmentServiceHandler) executeRebalancePlan(
	ctx context.Context,
	rebalancerRun *model.RebalancerRun,
	plan []rebalancePlanEntry,
) rebalanceExecutionResult {
	log := logger.FromContext(ctx)

	proposedTrades := []*domain.ProposedTrade{}
	investmentTrades := []model.InvestmentTrade{}
	for _, entry := range plan {
		proposedTrades = append(proposedTrades, entry.ProposedTrades...)
		investmentTrades = append(investmentTrades, entry.InvestmentTrades...)
	}
	if len(investmentTrades) == 0 {
		return rebalanceExecutionResult{}
	}

	executedOrders, err := h.TradingService.ExecuteBlock(ctx, proposedTrades, rebalancerRun.RebalancerRunID)
	if linkErr := h.linkExecutedTradesToInvestmentTrades(investmentTrades, executedOrders); linkErr != nil {
		log.Errorf("failed linking trade orders to investment trades: %s", linkErr.Error())
	}

	finalizeErr := h.finalizeRebalancePlan(plan, executedOrders, err)
	if finalizeErr != nil {
		log.Errorf("failed finalizing rebalance plan: %s", finalizeErr.Error())
	}

	if err != nil {
		if stateErr := h.markRebalancerRunTradeFailure(rebalancerRun, executedOrders, err); stateErr != nil {
			log.Errorf("failed updating rebalancer run after trade error: %s", stateErr.Error())
		}
	}

	return rebalanceExecutionResult{ExecutedOrders: executedOrders, Err: err}
}

func (h investmentServiceHandler) finalizeRebalancePlan(
	plan []rebalancePlanEntry,
	executedOrders []model.TradeOrder,
	executionErr error,
) error {
	orderByID := map[uuid.UUID]model.TradeOrder{}
	for _, order := range executedOrders {
		orderByID[order.TradeOrderID] = order
	}

	for _, entry := range plan {
		if len(entry.InvestmentTrades) == 0 {
			continue
		}

		hasError := executionErr != nil
		hasPending := false
		for _, trade := range entry.InvestmentTrades {
			if trade.TradeOrderID == nil {
				if executionErr != nil {
					hasError = true
				}
				continue
			}
			order, ok := orderByID[*trade.TradeOrderID]
			if !ok {
				hasError = true
				continue
			}
			switch order.Status {
			case model.TradeOrderStatus_Error, model.TradeOrderStatus_Canceled:
				hasError = true
			case model.TradeOrderStatus_Pending:
				hasPending = true
			}
		}

		if hasError {
			if err := h.markInvestmentRebalanceError(entry.InvestmentRebalanceID); err != nil {
				return err
			}
			reason := "rebalance trade execution failed"
			if executionErr != nil {
				reason = executionErr.Error()
			}
			if err := h.InvestmentRepository.SetErrorAt(nil, entry.InvestmentID, reason); err != nil {
				return err
			}
			continue
		}

		if hasPending {
			continue
		}
	}
	return nil
}

func (h investmentServiceHandler) markInvestmentRebalanceError(rebalanceID uuid.UUID) error {
	_, err := h.InvestmentRebalanceRepository.Update(nil, model.InvestmentRebalance{
		InvestmentRebalanceID: rebalanceID,
		State:                 model.RebalancerRunState_Error,
	}, postgres.ColumnList{table.InvestmentRebalance.State})
	return err
}

func (h investmentServiceHandler) markRebalancerRunTradeFailure(
	rebalancerRun *model.RebalancerRun,
	executedOrders []model.TradeOrder,
	executionErr error,
) error {
	note := fmt.Sprintf("trade execution failed: %s", executionErr.Error())
	if rebalancerRun.Notes != nil {
		note = *rebalancerRun.Notes + "; " + note
	}
	rebalancerRun.Notes = &note
	rebalancerRun.RebalancerRunState = model.RebalancerRunState_Error
	if hasPendingExecutedTrade(executedOrders) {
		rebalancerRun.RebalancerRunState = model.RebalancerRunState_Pending
	}
	_, err := h.RebalancerRunRepository.Update(nil, rebalancerRun, []postgres.Column{
		table.RebalancerRun.RebalancerRunState,
		table.RebalancerRun.Notes,
	})
	return err
}
