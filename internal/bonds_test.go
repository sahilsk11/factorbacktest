package internal

import (
	interestrate "factorbacktest/pkg/interest_rate"
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
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
			Rates: map[int]float64{
				1: 0.05,
			},
		}

		value := bond.currentValue(time.Now(), interestRatesMap)

		require.Equal(t, float64(100), value)
	})

	t.Run("interest rate drops", func(t *testing.T) {
		bond := Bond{
			Expiration:       time.Now().Add(730 * time.Hour), // 1mo
			AnnualCouponRate: 0.05,
			ParValue:         100,
		}
		interestRatesMap := interestrate.InterestRateMap{
			Rates: map[int]float64{
				1: 0.03,
			},
		}

		value := bond.currentValue(time.Now(), interestRatesMap)

		require.Equal(t, float64(104), value)
	})

	t.Run("interest rate rises", func(t *testing.T) {
		bond := Bond{
			Expiration:       time.Now().Add(730 * time.Hour), // 1mo
			AnnualCouponRate: 0.05,
			ParValue:         100,
		}
		interestRatesMap := interestrate.InterestRateMap{
			Rates: map[int]float64{
				1: 0.1,
			},
		}

		value := bond.currentValue(time.Now(), interestRatesMap)

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
					CouponPayments:       map[uuid.UUID][]Payment{},
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
			NewBond(200000, start, 1, 0.0148),
			NewBond(200000, start, 2, 0.0151),
			NewBond(200000, start, 3, 0.0155),
		}

		require.Equal(
			t,
			"",
			cmp.Diff(
				&BondPortfolio{
					Cash:                 0,
					TargetDurationMonths: []int{1, 2, 3},
					Bonds:                expectedBonds,
					CouponPayments:       map[uuid.UUID][]Payment{},
				},
				portfolio,
				cmpopts.IgnoreFields(Bond{}, "ID"),
			),
		)
	})
}

func TestBondPortfolio_RefreshCouponPayments(t *testing.T) {
	t.Run("refresh coupons", func(t *testing.T) {
		start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

		bond1 := NewBond(100, start, 1, 0.05)
		bond2 := NewBond(1000, start, 1, 0.1)
		bondPortfolio := &BondPortfolio{
			Bonds: []Bond{
				bond1,
				bond2,
			},
			CouponPayments: map[uuid.UUID][]Payment{},
		}

		firstRefresh := start.AddDate(0, 1, 1)
		bondPortfolio.refreshCouponPayments(firstRefresh)

		require.InDelta(t, 8.75, bondPortfolio.Cash, 0.0001)

		require.Equal(
			t,
			"",
			cmp.Diff(
				map[uuid.UUID][]Payment{
					bond1.ID: {
						{
							Date:   firstRefresh,
							Amount: 0.4166,
						},
					},
					bond2.ID: {
						{
							Date:   firstRefresh,
							Amount: 8.3333,
						},
					},
				},
				bondPortfolio.CouponPayments,
				floatCompare,
			),
		)
	})

}

var floatCompare = cmp.Comparer(func(i, j float64) bool {
	return math.Abs(i-j) < 1e-4
})

func TestBondPortfolio_RefreshBondHoldings(t *testing.T) {
	t.Run("refresh bond holdings", func(t *testing.T) {
		start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

		bond1 := NewBond(200, start, 1, 0.01)
		bond2 := NewBond(100, start, 2, 0.02)
		bondPortfolio := &BondPortfolio{
			Bonds: []Bond{
				bond1,
				bond2,
			},
			TargetDurationMonths: []int{1, 2},
		}

		firstRefresh := start.AddDate(0, 1, 1)
		bondPortfolio.refreshBondHoldings(firstRefresh, interestrate.InterestRateMap{
			Rates: map[int]float64{
				2: 0.05,
			},
		})

		require.Equal(
			t,
			"",
			cmp.Diff(
				[]Bond{
					bond2,
					NewBond(200, firstRefresh, 2, 0.05),
				},
				bondPortfolio.Bonds,
				floatCompare,
				cmpopts.IgnoreFields(Bond{}, "ID"),
			),
		)
	})
}
