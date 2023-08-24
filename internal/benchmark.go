package internal

import (
	"alpha/internal/domain"
	"alpha/internal/repository"
	"database/sql"
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
	fmt.Println(start, end)
	prices, err := h.PriceRepository.List(
		tx,
		symbol,
		start,
		end,
	)
	if err != nil {
		return nil, err
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("no prices found for symbol %s between %v and %v", symbol, start, end)
	}
	fmt.Println(len(prices))
	return intraPeriodChangeIterator(prices, start, end, granularity), nil
}

// okay doofenshmirtz
func intraPeriodChangeIterator(
	prices []domain.AssetPrice,
	start,
	end time.Time,
	granularity time.Duration,
) map[time.Time]float64 {
	layout := "2006-01-02"

	sort.Slice(prices, func(i2, j int) bool {
		return prices[i2].Date.Before(prices[j].Date)
	})
	Pprint(prices)
	fmt.Println(len(prices))

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
			// || i+1 < len(prices) && prices[i+1].Date.Unix() > nextTarget.Unix()
			// fmt.Println(nextTarget)
			out[nextTarget] = 100 * (prices[i].Price - prices[0].Price) / prices[0].Price
			nextTarget = nextTarget.Add(granularity)
			fmt.Println(nextTarget)
		}
		i++
	}
	fmt.Println(out)

	return out
}
