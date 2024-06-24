package internal

import (
	"context"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/qrm"
)

type FactorMetricsMissingDataError struct {
	Err error
}

func (e FactorMetricsMissingDataError) Error() string {
	return e.Err.Error()
}

type FactorMetricCalculations interface {
	Price(pr *service.PriceCache, symbol string, date time.Time) (float64, error)
	PricePercentChange(pr *service.PriceCache, symbol string, start, end time.Time) (float64, error)
	AnnualizedStdevOfDailyReturns(ctx context.Context, pr *service.PriceCache, symbol string, start, end time.Time) (float64, error)
	MarketCap(tx qrm.Queryable, symbol string, date time.Time) (float64, error)
	PeRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error)
	PbRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error)
}

type PriceCacheDataInput struct {
	Type string
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
	// these may contain duplicates
	Prices []service.LoadPriceCacheInput
	Stdevs []service.LoadStdevCacheInput
}

func (h *DryRunFactorMetricsHandler) Price(pr *service.PriceCache, symbol string, date time.Time) (float64, error) {
	h.Prices = append(h.Prices, service.LoadPriceCacheInput{
		Date:   date,
		Symbol: symbol,
	})
	return 0, nil
}

func (h *DryRunFactorMetricsHandler) PricePercentChange(pr *service.PriceCache, symbol string, start, end time.Time) (float64, error) {
	h.Prices = append(h.Prices, service.LoadPriceCacheInput{
		Date:   start,
		Symbol: symbol,
	})
	h.Prices = append(h.Prices, service.LoadPriceCacheInput{
		Date:   end,
		Symbol: symbol,
	})

	return 1, nil
}

func (h *DryRunFactorMetricsHandler) AnnualizedStdevOfDailyReturns(ctx context.Context, pr *service.PriceCache, symbol string, start, end time.Time) (float64, error) {
	h.Stdevs = append(h.Stdevs, service.LoadStdevCacheInput{
		Start:  start,
		End:    end,
		Symbol: symbol,
	})
	return 1, nil
}

func (h *DryRunFactorMetricsHandler) MarketCap(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	return 1, nil
}

func (h *DryRunFactorMetricsHandler) PeRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	return 1, nil
}

func (h *DryRunFactorMetricsHandler) PbRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	return 1, nil
}

func (h factorMetricsHandler) Price(pr *service.PriceCache, symbol string, date time.Time) (float64, error) {
	return pr.Get(symbol, date)
}

func (h factorMetricsHandler) PricePercentChange(pr *service.PriceCache, symbol string, start, end time.Time) (float64, error) {
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

func (h factorMetricsHandler) AnnualizedStdevOfDailyReturns(ctx context.Context, pr *service.PriceCache, symbol string, start, end time.Time) (float64, error) {
	return pr.GetStdev(ctx, symbol, start, end)
}

func (h factorMetricsHandler) MarketCap(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	out, err := h.AssetFundamentalsRepository.Get(tx, symbol, date)
	if err != nil {
		return 0, FactorMetricsMissingDataError{err}
	}

	price, err := h.AdjustedPriceRepository.Get(symbol, date)
	if err != nil {
		return 0, err
	}
	if out.SharesOutstandingBasic == nil {
		return 0, FactorMetricsMissingDataError{fmt.Errorf("%s does not have # shares outstanding on %v", symbol, date)}
	}

	return *out.SharesOutstandingBasic * price, nil
}

func (h factorMetricsHandler) PeRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	price, err := h.AdjustedPriceRepository.Get(symbol, date)
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

func (h factorMetricsHandler) PbRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error) {
	price, err := h.AdjustedPriceRepository.Get(symbol, date)
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
