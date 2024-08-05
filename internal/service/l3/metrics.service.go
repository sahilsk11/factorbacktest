package l3_service

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/montanaflynn/stats"
	"github.com/shopspring/decimal"
)

type CalculateMetricsResult struct {
	AnnualizedStdev  float64
	AnnualizedReturn float64
	SharpeRatio      float64
}

// calculateMetrics calculates metrics for the given snapshots. it assumes
// the snapshots sufficiently cover the expected range, which should be
// like 2 or three years
func CalculateMetrics(backtestResults []BacktestResult, relevantTradingDays []time.Time, totalPriceMap map[time.Time]map[string]decimal.Decimal) (*CalculateMetricsResult, error) {
	returns, err := calculateReturns(backtestResults, relevantTradingDays, totalPriceMap)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate returns: %w", err)
	}

	stdev, err := stats.StandardDeviationSample(returns)
	if err != nil {
		return nil, err
	}

	annualizedStdev := stdev * math.Sqrt(252)

	startValue := backtestResults[0].TotalValue
	endValue := backtestResults[len(backtestResults)-1].TotalValue
	numHours := backtestResults[len(backtestResults)-1].Date.Sub(backtestResults[0].Date).Hours()
	numYears := numHours / (365 * 24)
	annualizedReturn := math.Pow((endValue/startValue), 1/numYears) - 1

	sharpeRatio := annualizedReturn / stdev

	return &CalculateMetricsResult{
		AnnualizedStdev:  annualizedStdev,
		AnnualizedReturn: annualizedReturn,
		SharpeRatio:      sharpeRatio,
	}, nil
}

func calculateReturns(backtestResults []BacktestResult, relevantTradingDays []time.Time, totalPriceMap map[time.Time]map[string]decimal.Decimal) ([]float64, error) {
	if len(backtestResults) < 2 {
		return nil, fmt.Errorf("cannot calculate metrics on < 2 backtest results")
	}
	sort.Slice(backtestResults, func(i, j int) bool {
		return backtestResults[i].Date.Before(backtestResults[j].Date)
	})

	if !backtestResults[0].Date.Equal(relevantTradingDays[0]) {
		return nil, fmt.Errorf("assumption violated: first backtest should align with first trading day")
	}

	returns := []float64{}

	currentPortfolio := backtestResults[0].Portfolio
	// we can't always use TotalValue because we don't have it calculated
	// for every day
	lastValue := decimal.NewFromFloat(backtestResults[0].TotalValue)

	nextBacktestIndex := 1
	for _, t := range relevantTradingDays {
		if nextBacktestIndex < len(backtestResults) {
			nextBacktest := backtestResults[nextBacktestIndex]
			if t.Equal(nextBacktest.Date) || t.After(nextBacktest.Date) {
				currentPortfolio = nextBacktest.Portfolio
				nextBacktestIndex++
			}
		}

		value, err := currentPortfolio.TotalValue(totalPriceMap[t])
		if err != nil {
			return nil, fmt.Errorf("failed to calculate portfolio value on %v: %w", t, err)
		}

		ret := (value.Sub(lastValue)).Div(lastValue).InexactFloat64()
		lastValue = value

		returns = append(returns, ret)
	}

	return returns, nil
}
