package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"factorbacktest/internal/db/models/postgres/public/model"
	"factorbacktest/internal/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var ErrStaleReconciliation = errors.New("reconciliation snapshot is stale")

type ReconciliationAdjustment struct {
	InvestmentID uuid.UUID `json:"investmentID"`
	Symbol       string    `json:"symbol"`
	FromQuantity string    `json:"fromQuantity"`
	ToQuantity   string    `json:"toQuantity"`
}

type ReconciliationDifference struct {
	Symbol         string `json:"symbol"`
	LedgerQuantity string `json:"ledgerQuantity"`
	BrokerQuantity string `json:"brokerQuantity"`
	Difference     string `json:"difference"`
	Kind           string `json:"kind"`
}

type ReconciliationPreview struct {
	ReconciliationRunID uuid.UUID                  `json:"reconciliationRunID"`
	Status              string                     `json:"status"`
	Differences         []ReconciliationDifference `json:"differences"`
	ProposedAdjustments []ReconciliationAdjustment `json:"proposedAdjustments"`
	brokerSnapshot      map[string]string
	ledgerSnapshot      map[string]map[uuid.UUID]string
}

func (h investmentServiceHandler) computeReconciliation(ctx context.Context) (*ReconciliationPreview, error) {
	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{IncludePaused: true})
	if err != nil {
		return nil, err
	}
	ledger := map[string]map[uuid.UUID]string{}
	ledgerTotals := map[string]decimal.Decimal{}
	for _, investment := range investments {
		portfolio, err := h.HoldingsRepository.GetLatestHoldings(nil, investment.InvestmentID)
		if err != nil {
			return nil, err
		}
		for symbol, position := range portfolio.Positions {
			if ledger[symbol] == nil {
				ledger[symbol] = map[uuid.UUID]string{}
			}
			ledger[symbol][investment.InvestmentID] = position.ExactQuantity.String()
			ledgerTotals[symbol] = ledgerTotals[symbol].Add(position.ExactQuantity)
		}
	}

	positions, err := h.AlpacaRepository.GetPositions()
	if err != nil {
		return nil, err
	}
	broker := map[string]string{}
	brokerTotals := map[string]decimal.Decimal{}
	for _, position := range positions {
		broker[position.Symbol] = position.QtyAvailable.String()
		brokerTotals[position.Symbol] = position.QtyAvailable
	}

	preview := &ReconciliationPreview{
		Status:              "MATCHED",
		Differences:         []ReconciliationDifference{},
		ProposedAdjustments: []ReconciliationAdjustment{},
		brokerSnapshot:      broker,
		ledgerSnapshot:      ledger,
	}
	symbols := map[string]bool{}
	for symbol := range ledgerTotals {
		symbols[symbol] = true
	}
	for symbol := range brokerTotals {
		symbols[symbol] = true
	}
	for symbol := range symbols {
		ledgerQty := ledgerTotals[symbol]
		brokerQty := brokerTotals[symbol]
		difference := brokerQty.Sub(ledgerQty)
		if difference.Abs().LessThan(decimal.NewFromFloat(1e-6)) {
			continue
		}
		preview.Status = "MISMATCH"
		kind := "EXCESS"
		if difference.IsNegative() {
			kind = "SHORTAGE"
		}
		preview.Differences = append(preview.Differences, ReconciliationDifference{
			Symbol: symbol, LedgerQuantity: ledgerQty.String(), BrokerQuantity: brokerQty.String(),
			Difference: difference.String(), Kind: kind,
		})
		if kind == "SHORTAGE" && !ledgerQty.IsZero() {
			for investmentID, quantityString := range ledger[symbol] {
				quantity, _ := decimal.NewFromString(quantityString)
				corrected := brokerQty.Mul(quantity).Div(ledgerQty)
				preview.ProposedAdjustments = append(preview.ProposedAdjustments, ReconciliationAdjustment{
					InvestmentID: investmentID, Symbol: symbol,
					FromQuantity: quantity.String(), ToQuantity: corrected.String(),
				})
			}
		}
	}
	return preview, nil
}

func (h investmentServiceHandler) PreviewReconciliation(ctx context.Context) (*ReconciliationPreview, error) {
	preview, err := h.computeReconciliation(ctx)
	if err != nil {
		return nil, err
	}
	broker, _ := json.Marshal(preview.brokerSnapshot)
	ledger, _ := json.Marshal(preview.ledgerSnapshot)
	adjustments, _ := json.Marshal(preview.ProposedAdjustments)
	run, err := repository.NewReconciliationRepository(h.Db).Add(model.ReconciliationRun{
		Status: "PREVIEW", BrokerSnapshot: string(broker), LedgerSnapshot: string(ledger),
		ProposedAdjustments: string(adjustments),
	})
	if err != nil {
		return nil, err
	}
	preview.ReconciliationRunID = run.ReconciliationRunID
	return preview, nil
}

func (h investmentServiceHandler) ApplyReconciliation(ctx context.Context, runID uuid.UUID) error {
	repo := repository.NewReconciliationRepository(h.Db)
	run, err := repo.Get(runID)
	if err != nil {
		return err
	}
	if run.Status != "PREVIEW" {
		return fmt.Errorf("reconciliation run is %s", run.Status)
	}
	current, err := h.computeReconciliation(ctx)
	if err != nil {
		return err
	}
	var storedBroker map[string]string
	var storedLedger map[string]map[uuid.UUID]string
	if err := json.Unmarshal([]byte(run.BrokerSnapshot), &storedBroker); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(run.LedgerSnapshot), &storedLedger); err != nil {
		return err
	}
	if !reflect.DeepEqual(current.brokerSnapshot, storedBroker) || !reflect.DeepEqual(current.ledgerSnapshot, storedLedger) {
		_ = repo.SetStatus(nil, runID, "STALE")
		return ErrStaleReconciliation
	}
	var adjustments []ReconciliationAdjustment
	if err := json.Unmarshal([]byte(run.ProposedAdjustments), &adjustments); err != nil {
		return err
	}
	byInvestment := map[uuid.UUID]map[string]decimal.Decimal{}
	for _, adjustment := range adjustments {
		quantity, err := decimal.NewFromString(adjustment.ToQuantity)
		if err != nil {
			return err
		}
		if byInvestment[adjustment.InvestmentID] == nil {
			byInvestment[adjustment.InvestmentID] = map[string]decimal.Decimal{}
		}
		byInvestment[adjustment.InvestmentID][adjustment.Symbol] = quantity
	}
	tx, err := h.Db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	cashTicker, err := h.TickerRepository.GetCashTicker()
	if err != nil {
		return err
	}
	for investmentID, corrected := range byInvestment {
		portfolio, err := h.HoldingsRepository.GetLatestHoldings(tx, investmentID)
		if err != nil {
			return err
		}
		version, err := h.HoldingsVersionRepository.Add(tx, model.InvestmentHoldingsVersion{
			InvestmentID: investmentID, ReconciliationRunID: &runID,
		})
		if err != nil {
			return err
		}
		for symbol, position := range portfolio.Positions {
			quantity := position.ExactQuantity
			if value, ok := corrected[symbol]; ok {
				quantity = value
			}
			if quantity.IsZero() {
				continue
			}
			_, err = h.HoldingsRepository.Add(tx, model.InvestmentHoldings{
				TickerID: position.TickerID, Quantity: quantity,
				InvestmentHoldingsVersionID: version.InvestmentHoldingsVersionID,
			})
			if err != nil {
				return err
			}
		}
		_, err = h.HoldingsRepository.Add(tx, model.InvestmentHoldings{
			TickerID: cashTicker.TickerID, Quantity: *portfolio.Cash,
			InvestmentHoldingsVersionID: version.InvestmentHoldingsVersionID,
		})
		if err != nil {
			return err
		}
	}
	if err := repo.SetStatus(tx, runID, "APPLIED"); err != nil {
		return err
	}
	return tx.Commit()
}
