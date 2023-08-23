package internal

import (
	"alpha/internal/repository"
	"database/sql"
	"math"
	"time"

	"github.com/montanaflynn/stats"
)

type FactorMetricCalculations interface {
	Price(tx *sql.Tx, symbol string, date time.Time) (float64, error)
	PricePercentChange(tx *sql.Tx, symbol string, start, end time.Time) (float64, error)
	AnnualizedStdevOfDailyReturns(tx *sql.Tx, symbol string, start, end time.Time) (float64, error)
	MarketCap(tx *sql.Tx, symbol string, date time.Time) (float64, error)
	PeRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error)
	PbRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error)
}

type FactorMetricsHandler struct {
	AdjustedPriceRepository     repository.AdjustedPriceRepository
	AssetFundamentalsRepository repository.AssetFundamentalsRepository
}

func (h FactorMetricsHandler) Price(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
	return h.AdjustedPriceRepository.Get(tx, symbol, date)
}

func (h FactorMetricsHandler) PricePercentChange(tx *sql.Tx, symbol string, start, end time.Time) (float64, error) {
	startPrice, err := h.AdjustedPriceRepository.Get(tx, symbol, start)
	if err != nil {
		return 0, err
	}

	endPrice, err := h.AdjustedPriceRepository.Get(tx, symbol, end)
	if err != nil {
		return 0, err
	}

	return percentChange(endPrice, startPrice), nil
}

func percentChange(end, start float64) float64 {
	return ((end - start) / end) * 100
}

func (h FactorMetricsHandler) AnnualizedStdevOfDailyReturns(tx *sql.Tx, symbol string, start, end time.Time) (float64, error) {
	priceModels, err := h.AdjustedPriceRepository.List(tx, symbol, start, end)
	if err != nil {
		return 0, err
	}
	intradayChanges := make([]float64, len(priceModels)-1)
	for i := 1; i < len(priceModels); i++ {
		intradayChanges[i-1] = percentChange(
			priceModels[i].Price,
			priceModels[i-1].Price,
		)
	}

	stdev, err := stats.StandardDeviationSample(intradayChanges)
	if err != nil {
		return 0, err
	}
	magicNumber := math.Sqrt(252)

	return stdev * magicNumber, nil
}

func (h FactorMetricsHandler) MarketCap(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
	return 0, nil
}

func (h FactorMetricsHandler) PeRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
	return 0, nil
}

func (h FactorMetricsHandler) PbRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
	return 0, nil
}
