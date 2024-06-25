package internal

import (
	interestrate "factorbacktest/pkg/interest_rate"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestBond_currentValue(t *testing.T) {
	t.Run("interest rate steady", func(t *testing.T) {
		bond := Bond{
			Expiration:       time.Now().Add(730 * time.Hour), // 1mo
			AnnualCouponRate: 0.05,
			ParValue:         100,
		}
		interestRatesMap := interestrate.InterestRateMap{
			SortedKeys: []int{1},
			Rates: map[int]float64{
				1: 0.05,
			},
		}

		value := bond.currentValue(interestRatesMap)

		require.Equal(t, float64(100), value)
	})

	t.Run("interest rate drops", func(t *testing.T) {
		bond := Bond{
			Expiration:       time.Now().Add(730 * time.Hour), // 1mo
			AnnualCouponRate: 0.05,
			ParValue:         100,
		}
		interestRatesMap := interestrate.InterestRateMap{
			SortedKeys: []int{1},
			Rates: map[int]float64{
				1: 0.03,
			},
		}

		value := bond.currentValue(interestRatesMap)

		require.Equal(t, float64(104), value)
	})

	t.Run("interest rate rises", func(t *testing.T) {
		bond := Bond{
			Expiration:       time.Now().Add(730 * time.Hour), // 1mo
			AnnualCouponRate: 0.05,
			ParValue:         100,
		}
		interestRatesMap := interestrate.InterestRateMap{
			SortedKeys: []int{1},
			Rates: map[int]float64{
				1: 0.1,
			},
		}

		value := bond.currentValue(interestRatesMap)

		require.Equal(t, float64(90), value)
	})
}

func TestConstructBondPortfolio(t *testing.T) {
	t.Run("no bonds purchased", func(t *testing.T) {
		portfolio, err := ConstructBondPortfolio(
			time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			[]int{},
			100_000,
		)
		require.NoError(t, err)

		require.Equal(
			t,
			"",
			cmp.Diff(
				&BondPortfolio{
					Cash:                 100000,
					TargetDurationMonths: []int{},
					Bonds:                []Bond{},
				},
				portfolio,
			),
		)
	})

	t.Run("1mo bond spread", func(t *testing.T) {
		start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

		portfolio, err := ConstructBondPortfolio(
			start,
			[]int{1, 2, 3},
			600_000,
		)
		require.NoError(t, err)

		expectedBonds := []Bond{
			{
				Expiration:       start.AddDate(0, 1, 0),
				AnnualCouponRate: 0.0148,
				ParValue:         200000,
			},
			{
				Expiration:       start.AddDate(0, 2, 0),
				AnnualCouponRate: 0.0151,
				ParValue:         200000,
			},
			{
				Expiration:       start.AddDate(0, 3, 0),
				AnnualCouponRate: 0.0155,
				ParValue:         200000,
			},
		}

		require.Equal(
			t,
			"",
			cmp.Diff(
				&BondPortfolio{
					Cash:                 0,
					TargetDurationMonths: []int{1, 2, 3},
					Bonds:                expectedBonds,
				},
				portfolio,
			),
		)
	})
}
