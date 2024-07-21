package internal

import (
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"factorbacktest/internal/util"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

type BenchmarkHandler struct {
	PriceRepository repository.AdjustedPriceRepository
}

// GetIntraPeriodChange get historic prices for an asset
// and converts it to % change from start
func (h BenchmarkHandler) GetIntraPeriodChange(
	symbol string,
	start,
	end time.Time,
	granularity time.Duration,
) (map[time.Time]float64, error) {
	prices, err := h.PriceRepository.List(
		[]string{symbol},
		start,
		end,
	)
	if err != nil {
		return nil, err
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("no prices found for symbol %s between %v and %v", symbol, start, end)
	}
	return intraPeriodChangeIterator(prices, end, granularity), nil
}

// okay doofenshmirtz
func intraPeriodChangeIterator(
	prices []domain.AssetPrice,
	end time.Time,
	granularity time.Duration,
) map[time.Time]float64 {
	layout := "2006-01-02"

	sort.Slice(prices, func(i2, j int) bool {
		return prices[i2].Date.Before(prices[j].Date)
	})

	i := 1
	// TODO - handle if len(prices) == 0
	// or value out of range
	out := map[time.Time]float64{
		prices[0].Date: 0,
	}
	nextTarget := prices[0].Date.Add(granularity)
	for i < len(prices) && util.DateLte(prices[i].Date, end) {
		for nextTarget.Format(layout) < prices[i].Date.Format(layout) {
			nextTarget = nextTarget.Add(24 * time.Hour)
		}
		if prices[i].Date.Format(layout) == nextTarget.Format(layout) {
			out[nextTarget] = decimal.NewFromInt(100).Mul((prices[i].Price.Sub(prices[0].Price))).Div(prices[0].Price).InexactFloat64()
			nextTarget = nextTarget.Add(granularity)
		}
		i++
	}

	return out
}
