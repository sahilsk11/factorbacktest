package service

import (
	"context"
	"fmt"
	"time"

	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/shopspring/decimal"
)

const cashMismatchSymbol = "CASH"

var (
	quantityMismatchEpsilonZero       = decimal.NewFromFloat(1e-6)
	quantityMismatchExcessThreshold   = decimal.NewFromInt(2)
)

type QuantityMismatchKind string

const (
	QuantityMismatchShortage QuantityMismatchKind = "SHORTAGE"
	QuantityMismatchExcess   QuantityMismatchKind = "EXCESS"
)

type QuantityMismatch struct {
	Symbol    string               `json:"symbol"`
	LedgerQty float64              `json:"ledgerQty"`
	BrokerQty float64              `json:"brokerQty"`
	Delta     float64              `json:"delta"`
	Kind      QuantityMismatchKind `json:"kind"`
}

type QuantityMismatchCheckResult struct {
	Status     string             `json:"status"`
	CheckedAt  time.Time          `json:"checkedAt"`
	Mismatches []QuantityMismatch `json:"mismatches"`
	Notes      []string           `json:"notes,omitempty"`
}

func (h investmentServiceHandler) CheckQuantityMismatch(ctx context.Context) (*QuantityMismatchCheckResult, error) {
	totalHoldings, err := h.aggregateInvestmentHoldings()
	if err != nil {
		return nil, err
	}

	account, err := h.AlpacaRepository.GetAccount()
	if err != nil {
		return nil, err
	}

	positions, err := h.AlpacaRepository.GetPositions()
	if err != nil {
		return nil, err
	}

	mismatches := detectQuantityMismatches(totalHoldings, account, positions)
	status := "OK"
	if len(mismatches) > 0 {
		status = "MISMATCH"
	}

	return &QuantityMismatchCheckResult{
		Status:     status,
		CheckedAt:  time.Now().UTC(),
		Mismatches: mismatches,
	}, nil
}

func (h investmentServiceHandler) aggregateInvestmentHoldings() (*domain.Portfolio, error) {
	investments, err := h.InvestmentRepository.List(repository.StrategyInvestmentListFilter{})
	if err != nil {
		return nil, err
	}

	totalHoldings := domain.NewPortfolio()
	for _, investment := range investments {
		holdings, err := h.HoldingsRepository.GetLatestHoldings(nil, investment.InvestmentID)
		if err != nil {
			return nil, err
		}
		totalHoldings.SetCash(totalHoldings.Cash.Add(*holdings.Cash))
		for _, position := range holdings.Positions {
			if _, ok := totalHoldings.Positions[position.Symbol]; !ok {
				totalHoldings.Positions[position.Symbol] = &domain.Position{
					Symbol:        position.Symbol,
					Quantity:      0,
					ExactQuantity: decimal.Zero,
					TickerID:      position.TickerID,
				}
			}
			totalHoldings.Positions[position.Symbol].Quantity += position.Quantity
			totalHoldings.Positions[position.Symbol].ExactQuantity = totalHoldings.Positions[position.Symbol].ExactQuantity.Add(position.ExactQuantity)
		}
	}

	return totalHoldings, nil
}

func detectQuantityMismatches(totalHoldings *domain.Portfolio, account *alpaca.Account, positions []alpaca.Position) []QuantityMismatch {
	mismatches := []QuantityMismatch{}

	if account.Cash.LessThan(*totalHoldings.Cash) {
		ledgerCash := totalHoldings.Cash.InexactFloat64()
		brokerCash := account.Cash.InexactFloat64()
		mismatches = append(mismatches, QuantityMismatch{
			Symbol:    cashMismatchSymbol,
			LedgerQty: ledgerCash,
			BrokerQty: brokerCash,
			Delta:     brokerCash - ledgerCash,
			Kind:      QuantityMismatchShortage,
		})
	}

	for _, ledgerPosition := range totalHoldings.Positions {
		for _, brokerPosition := range positions {
			if brokerPosition.Symbol != ledgerPosition.Symbol {
				continue
			}
			if brokerPosition.Qty.LessThan(ledgerPosition.ExactQuantity.Sub(quantityMismatchEpsilonZero)) {
				ledgerQty := ledgerPosition.ExactQuantity.InexactFloat64()
				brokerQty := brokerPosition.Qty.InexactFloat64()
				mismatches = append(mismatches, QuantityMismatch{
					Symbol:    brokerPosition.Symbol,
					LedgerQty: ledgerQty,
					BrokerQty: brokerQty,
					Delta:     brokerQty - ledgerQty,
					Kind:      QuantityMismatchShortage,
				})
			} else if brokerPosition.Qty.GreaterThan(ledgerPosition.ExactQuantity.Add(quantityMismatchExcessThreshold)) {
				ledgerQty := ledgerPosition.ExactQuantity.InexactFloat64()
				brokerQty := brokerPosition.Qty.InexactFloat64()
				mismatches = append(mismatches, QuantityMismatch{
					Symbol:    brokerPosition.Symbol,
					LedgerQty: ledgerQty,
					BrokerQty: brokerQty,
					Delta:     brokerQty - ledgerQty,
					Kind:      QuantityMismatchExcess,
				})
			}
		}
	}

	return mismatches
}

func quantityMismatchToReconErr(mismatch QuantityMismatch) ReconErr {
	switch mismatch.Symbol {
	case cashMismatchSymbol:
		return ReconErr{
			Message: fmt.Sprintf(
				"alpaca account holding insufficient cash: aggregate portfolio %f vs alpaca %f",
				mismatch.LedgerQty,
				mismatch.BrokerQty,
			),
		}
	default:
		if mismatch.Kind == QuantityMismatchShortage {
			return ReconErr{
				Message: fmt.Sprintf(
					"alpaca account holding insufficient %s: aggregate portfolio %f vs alpaca %f (%f)",
					mismatch.Symbol,
					mismatch.LedgerQty,
					mismatch.BrokerQty,
					mismatch.Delta,
				),
			}
		}
		return ReconErr{
			Message: fmt.Sprintf(
				"alpaca account holding excess %s: aggregate portfolio %f vs alpaca %f",
				mismatch.Symbol,
				mismatch.LedgerQty,
				mismatch.BrokerQty,
			),
		}
	}
}
