package internal

import (
	"alpha/internal/repository"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/montanaflynn/stats"
)

type FactorMetricsMissingDataError struct {
	Err error
}

func (e FactorMetricsMissingDataError) Error() string {
	return e.Err.Error()
}

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
	out, err := h.AssetFundamentalsRepository.Get(tx, symbol, date)
	if err != nil {
		return 0, FactorMetricsMissingDataError{err}
	}

	price, err := h.AdjustedPriceRepository.Get(tx, symbol, date)
	if err != nil {
		return 0, err
	}
	if out.SharesOutstandingBasic == nil {
		return 0, FactorMetricsMissingDataError{fmt.Errorf("%s does not have # shares outstanding on %v", symbol, date)}
	}

	return *out.SharesOutstandingBasic * price, nil
}

func (h FactorMetricsHandler) PeRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
	price, err := h.AdjustedPriceRepository.Get(tx, symbol, date)
	if err != nil {
		return 0, err
	}

	out, err := h.AssetFundamentalsRepository.Get(tx, symbol, date)
	if err != nil {
		return 0, FactorMetricsMissingDataError{err}
	}
	if out.EpsBasic == nil {
		return 0, FactorMetricsMissingDataError{fmt.Errorf("%s does not have eps on %v", symbol, date)}
	}

	return price / *out.EpsBasic, nil

}

func (h FactorMetricsHandler) PbRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
	price, err := h.AdjustedPriceRepository.Get(tx, symbol, date)
	if err != nil {
		return 0, err
	}

	out, err := h.AssetFundamentalsRepository.Get(tx, symbol, date)
	if err != nil {
		return 0, FactorMetricsMissingDataError{err}
	}

	if out.TotalAssets == nil {
		return 0, FactorMetricsMissingDataError{fmt.Errorf("missing total assets")}
	}
	if out.TotalLiabilities == nil {
		return 0, FactorMetricsMissingDataError{fmt.Errorf("missing total liabilities")}
	}
	if out.SharesOutstandingBasic == nil {
		return 0, FactorMetricsMissingDataError{fmt.Errorf("missing shares outstanding")}
	}

	return price / ((*out.TotalAssets - *out.TotalLiabilities) / *out.SharesOutstandingBasic), nil
}
