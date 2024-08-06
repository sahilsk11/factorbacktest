package l3_service

import (
	"context"
	"factorbacktest/internal/repository"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type StrategyService interface {
	// Add()
	// AddRun()
	CalculateMetrics(ctx context.Context, strategyID uuid.UUID, backtestResults []BacktestResult) (*CalculateMetricsResult, error)
	// Save()
	// Publish()
	// Unsave()
	// Unpublish()
}

func NewStrategyService(
	strategyRepository repository.StrategyRepository,
	universeRepository repository.AssetUniverseRepository,
	priceRepository repository.AdjustedPriceRepository,
	backtestHandler BacktestHandler,
) StrategyService {
	return strategyServiceHandler{
		StrategyRepository: strategyRepository,
		UniverseRepository: universeRepository,
		PriceRepository:    priceRepository,
		BacktestHandler:    backtestHandler,
	}
}

type strategyServiceHandler struct {
	StrategyRepository repository.StrategyRepository
	UniverseRepository repository.AssetUniverseRepository
	PriceRepository    repository.AdjustedPriceRepository
	BacktestHandler    BacktestHandler
}

func (h strategyServiceHandler) CalculateMetrics(ctx context.Context, strategyID uuid.UUID, backtestResults []BacktestResult) (*CalculateMetricsResult, error) {
	strategy, err := h.StrategyRepository.Get(strategyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy: %w", err)
	}

	// let's use three year windows for stats
	start := backtestResults[0].Date
	end := backtestResults[len(backtestResults)-1].Date

	assets, err := h.UniverseRepository.GetAssets(strategy.AssetUniverse)
	if err != nil {
		return nil, fmt.Errorf("failed to get assets from universie name")
	}
	getPricesInput := []repository.GetManyInput{}
	for _, a := range assets {
		getPricesInput = append(getPricesInput, repository.GetManyInput{
			Symbol:  a.Symbol,
			MinDate: start,
			MaxDate: end,
		})
	}
	prices, err := h.PriceRepository.GetMany(getPricesInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get prices: %w", err)
	}

	mappedPrices := map[time.Time]map[string]decimal.Decimal{}
	for _, p := range prices {
		if _, ok := mappedPrices[p.Date]; !ok {
			mappedPrices[p.Date] = map[string]decimal.Decimal{}
		}
		mappedPrices[p.Date][p.Symbol] = p.Price
	}

	relevantTradingDays, err := h.PriceRepository.ListTradingDays(
		start,
		end,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list trading days: %w", err)
	}

	metrics, err := CalculateMetrics(backtestResults, relevantTradingDays, mappedPrices)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate metrics: %w", err)
	}

	return metrics, nil
}
