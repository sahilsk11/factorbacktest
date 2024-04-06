package internal

import (
	"database/sql"
	"factorbacktest/internal/domain"
	"factorbacktest/internal/repository"
	"fmt"
	"sort"
	"time"
)

type BenchmarkHandler struct {
	PriceRepository repository.AdjustedPriceRepository
}

func (h BenchmarkHandler) GetIntraPeriodChange(
	tx *sql.Tx,
	symbol string,
	start,
	end time.Time,
	granularity time.Duration,
) (map[time.Time]float64, error) {
	prices, err := h.PriceRepository.List(
		tx,
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
	for i < len(prices) && DateLte(prices[i].Date, end) {
		for nextTarget.Format(layout) < prices[i].Date.Format(layout) {
			nextTarget = nextTarget.Add(24 * time.Hour)
		}
		if prices[i].Date.Format(layout) == nextTarget.Format(layout) {
			out[nextTarget] = 100 * (prices[i].Price - prices[0].Price) / prices[0].Price
			nextTarget = nextTarget.Add(granularity)
		}
		i++
	}

	return out
}
