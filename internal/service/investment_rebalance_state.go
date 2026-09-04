package service

import (
	"database/sql"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/db/models/postgres/public/table"
	"factorbacktest/internal/repository"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/google/uuid"
)

func markInvestmentRebalanceCompleted(
	repo repository.InvestmentRebalanceRepository,
	tx *sql.Tx,
	rebalanceID uuid.UUID,
) error {
	_, err := repo.Update(tx, model.InvestmentRebalance{
		InvestmentRebalanceID: rebalanceID,
		State:                 model.RebalancerRunState_Completed,
	}, postgres.ColumnList{table.InvestmentRebalance.State})
	return err
}

func investmentTradeStatusesAllCompleted(trades []*model.InvestmentTradeStatus) bool {
	if len(trades) == 0 {
		return true
	}
	for _, t := range trades {
		if t.Status == nil || *t.Status != model.TradeOrderStatus_Completed {
			return false
		}
	}
	return true
}

func completeInvestmentRebalancesForRun(
	tx *sql.Tx,
	investmentRebalanceRepository repository.InvestmentRebalanceRepository,
	investmentTradeRepository repository.InvestmentTradeRepository,
	rebalancerRunID uuid.UUID,
) error {
	investmentRebalances, err := investmentRebalanceRepository.ListByRebalancerRunID(tx, rebalancerRunID)
	if err != nil {
		return err
	}

	for _, ir := range investmentRebalances {
		if ir.State != model.RebalancerRunState_Pending {
			continue
		}

		trades, err := investmentTradeRepository.List(tx, repository.InvestmentTradeListFilter{
			RebalancerRunID: &rebalancerRunID,
			InvestmentID:    &ir.InvestmentID,
		})
		if err != nil {
			return err
		}
		if !investmentTradeStatusesAllCompleted(trades) {
			continue
		}
		if err := markInvestmentRebalanceCompleted(investmentRebalanceRepository, tx, ir.InvestmentRebalanceID); err != nil {
			return err
		}
	}
	return nil
}
