package internal

import (
	"database/sql"
	"factorbacktest/internal/repository"
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

type PriceRetriever interface {
	Get(symbol string, date time.Time) (float64, error)
}

type FactorMetricCalculations interface {
	Price(pr PriceRetriever, symbol string, date time.Time) (float64, error)
	PricePercentChange(pr PriceRetriever, symbol string, start, end time.Time) (float64, error)
	AnnualizedStdevOfDailyReturns(tx *sql.Tx, symbol string, start, end time.Time) (float64, error)
	MarketCap(tx *sql.Tx, symbol string, date time.Time) (float64, error)
	PeRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error)
	PbRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error)
}

type DataInput struct {
	Type   string
	Date   time.Time
	Symbol string
}

type factorMetricsHandler struct {
	// both dependencies should be wrapped in some mds service
	AdjustedPriceRepository     repository.AdjustedPriceRepository
	AssetFundamentalsRepository repository.AssetFundamentalsRepository
}

func NewFactorMetricsHandler(adjPriceRepository repository.AdjustedPriceRepository, afRepository repository.AssetFundamentalsRepository) FactorMetricCalculations {
	return factorMetricsHandler{
		AdjustedPriceRepository:     adjPriceRepository,
		AssetFundamentalsRepository: afRepository,
	}
}

type DryRunFactorMetricsHandler struct {
	Data []DataInput
}

func (h *DryRunFactorMetricsHandler) Price(pr PriceRetriever, symbol string, date time.Time) (float64, error) {
	h.Data = append(h.Data, DataInput{
		Type:   "price",
		Date:   date,
		Symbol: symbol,
	})
	return 0, nil
}

func (h *DryRunFactorMetricsHandler) PricePercentChange(pr PriceRetriever, symbol string, start, end time.Time) (float64, error) {
	h.Data = append(h.Data, DataInput{
		Type:   "price",
		Date:   start,
		Symbol: symbol,
	})
	h.Data = append(h.Data, DataInput{
		Type:   "price",
		Date:   end,
		Symbol: symbol,
	})
	return 1, nil
}

func (h *DryRunFactorMetricsHandler) AnnualizedStdevOfDailyReturns(tx *sql.Tx, symbol string, start, end time.Time) (float64, error) {
	current := start
	for current.Before(end) {
		h.Data = append(h.Data, DataInput{
			Type:   "price",
			Date:   current,
			Symbol: symbol,
		})
		current = current.AddDate(0, 0, 1)
	}
	return 1, nil
}

func (h *DryRunFactorMetricsHandler) MarketCap(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
	return 1, nil
}

func (h *DryRunFactorMetricsHandler) PeRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
	return 1, nil
}

func (h *DryRunFactorMetricsHandler) PbRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
	return 1, nil
}

func (h factorMetricsHandler) Price(pr PriceRetriever, symbol string, date time.Time) (float64, error) {
	return pr.Get(symbol, date)
}

func (h factorMetricsHandler) PricePercentChange(pr PriceRetriever, symbol string, start, end time.Time) (float64, error) {
	startPrice, err := pr.Get(symbol, start)
	if err != nil {
		return 0, err
	}

	endPrice, err := pr.Get(symbol, end)
	if err != nil {
		return 0, err
	}

	return percentChange(endPrice, startPrice), nil
}

func percentChange(end, start float64) float64 {
	return ((end - start) / end) * 100
}

func (h factorMetricsHandler) AnnualizedStdevOfDailyReturns(tx *sql.Tx, symbol string, start, end time.Time) (float64, error) {
	priceModels, err := h.AdjustedPriceRepository.List(tx, []string{symbol}, start, end)
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

func (h factorMetricsHandler) MarketCap(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
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

func (h factorMetricsHandler) PeRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
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

func (h factorMetricsHandler) PbRatio(tx *sql.Tx, symbol string, date time.Time) (float64, error) {
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
