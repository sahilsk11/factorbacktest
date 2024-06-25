package internal

import (
	"context"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/service"
	"fmt"
	"math"
	"time"

	"github.com/go-jet/jet/v2/qrm"
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
	AnnualizedStdevOfDailyReturns(ctx context.Context, symbol string, start, end time.Time) (float64, error)
	MarketCap(tx qrm.Queryable, symbol string, date time.Time) (float64, error)
	PeRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error)
	PbRatio(tx qrm.Queryable, symbol string, date time.Time) (float64, error)
}

type DataInput struct {
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
	Data map[string]service.LoadPriceCacheInput
}

func (h *DryRunFactorMetricsHandler) Price(pr PriceRetriever, symbol string, date time.Time) (float64, error) {
	key := fmt.Sprintf("price/%s/%s", date.Format(time.DateOnly), symbol)
	h.Data[key] = service.LoadPriceCacheInput{
		Date:   date,
		Symbol: symbol,
	}
	return 0, nil
}

func (h *DryRunFactorMetricsHandler) PricePercentChange(pr PriceRetriever, symbol string, start, end time.Time) (float64, error) {
	key := fmt.Sprintf("price/%s/%s", start.Format(time.DateOnly), symbol)
	h.Data[key] = service.LoadPriceCacheInput{
		Date:   start,
		Symbol: symbol,
	}
	key = fmt.Sprintf("price/%s/%s", end.Format(time.DateOnly), symbol)
	h.Data[key] = service.LoadPriceCacheInput{
		Date:   end,
		Symbol: symbol,
	}
	return 1, nil
}

func (h *DryRunFactorMetricsHandler) AnnualizedStdevOfDailyReturns(ctx context.Context, symbol string, start, end time.Time) (float64, error) {
	current := start
	for current.Before(end) {
		key := fmt.Sprintf("price/%s/%s", current.Format(time.DateOnly), symbol)
		h.Data[key] = service.LoadPriceCacheInput{
			Date:   current,
			Symbol: symbol,
		}
		current = current.AddDate(0, 0, 1)
	}
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

// super hacky but because we can't control the direct caller of the metrics
// functions, we can't set the parent span correctly. so just create it here
// def one of those things I will look back and hate - sry
func (h factorMetricsHandler) createProfile(ctx context.Context, funcName string) (*domain.Profile, func()) {
	profile, _ := domain.GetProfile(ctx)
	newSpan, endSpan := profile.StartNewSpan(funcName)
	newProfile, endProfile := newSpan.NewSubProfile()
	end := func() {
		endSpan()
		endProfile()
	}
	return newProfile, end
}

func (h factorMetricsHandler) AnnualizedStdevOfDailyReturns(ctx context.Context, symbol string, start, end time.Time) (float64, error) {
	profile, endProfile := h.createProfile(ctx, "AnnualizedStdevOfDailyReturns")
	defer endProfile()

	_, endSpan := profile.StartNewSpan("listing prices")
	priceModels, err := h.AdjustedPriceRepository.List([]string{symbol}, start, end)
	if err != nil {
		return 0, err
	}
	endSpan()
	_, endSpan = profile.StartNewSpan("converting to percent change")
	intradayChanges := make([]float64, len(priceModels)-1)
	for i := 1; i < len(priceModels); i++ {
		intradayChanges[i-1] = percentChange(
			priceModels[i].Price,
			priceModels[i-1].Price,
		)
	}
	endSpan()

	_, endSpan = profile.StartNewSpan("calculating stdev")
	defer endSpan()
	stdev, err := stats.StandardDeviationSample(intradayChanges)
	if err != nil {
		return 0, err
	}
	magicNumber := math.Sqrt(252)

	return stdev * magicNumber, nil
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
