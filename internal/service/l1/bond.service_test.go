package l1_service_test

// import (
// 	"factorbacktest/internal/domain"
// 	mock_repository "factorbacktest/internal/repository/mocks"
// 	"math"
// 	"testing"
// 	"time"

// 	"github.com/golang/mock/gomock"
// 	"github.com/google/go-cmp/cmp"
// 	"github.com/google/go-cmp/cmp/cmpopts"
// 	"github.com/google/uuid"
// 	"github.com/stretchr/testify/require"
// )

// func TestBond_currentValue(t *testing.T) {
// 	t.Run("interest rate steady", func(t *testing.T) {
// 		bond := Bond{
// 			Expiration:       time.Now().Add(730 * time.Hour), // 1mo
// 			AnnualCouponRate: 0.05,
// 			ParValue:         100,
// 		}
// 		interestRatesMap := domain.InterestRateMap{
// 			Rates: map[int]float64{
// 				1: 0.05,
// 			},
// 		}

// 		value, err := bond.currentValue(time.Now(), interestRatesMap)
// 		require.NoError(t, err)

// 		require.Equal(t, float64(100), value)
// 	})

// 	t.Run("interest rate drops", func(t *testing.T) {
// 		bond := Bond{
// 			Expiration:       time.Now().AddDate(1, 0, 0),
// 			AnnualCouponRate: 0.05,
// 			ParValue:         100,
// 		}
// 		interestRatesMap := domain.InterestRateMap{
// 			Rates: map[int]float64{
// 				1: 0.01,
// 			},
// 		}

// 		value, err := bond.currentValue(time.Now(), interestRatesMap)
// 		require.NoError(t, err)

// 		require.Equal(t, float64(104), value)
// 	})

// 	t.Run("interest rate rises", func(t *testing.T) {
// 		bond := Bond{
// 			Expiration:       time.Now().AddDate(1, 0, 0),
// 			AnnualCouponRate: 0.05,
// 			ParValue:         100,
// 		}
// 		interestRatesMap := domain.InterestRateMap{
// 			Rates: map[int]float64{
// 				12: 0.1,
// 			},
// 		}

// 		value, err := bond.currentValue(time.Now(), interestRatesMap)
// 		require.NoError(t, err)

// 		require.Equal(t, float64(95), value)
// 	})
// }

// func TestConstructBondPortfolio(t *testing.T) {
// 	t.Run("no bonds purchased", func(t *testing.T) {
// 		ctrl := gomock.NewController(t)
// 		interestRateRepository := mock_repository.NewMockInterestRateRepository(ctrl)
// 		bs := BondService{
// 			InterestRateRepository: interestRateRepository,
// 		}

// 		interestRateRepository.EXPECT().GetRatesOnDate(gomock.Any(), nil).Return(
// 			&domain.InterestRateMap{}, nil,
// 		)

// 		portfolio, err := bs.ConstructBondPortfolio(
// 			nil,
// 			time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
// 			[]int{},
// 			100_000,
// 		)
// 		require.NoError(t, err)

// 		require.Equal(
// 			t,
// 			"",
// 			cmp.Diff(
// 				&BondPortfolio{
// 					Cash:                 100000,
// 					TargetDurationMonths: []int{},
// 					Bonds:                []Bond{},
// 					CouponPayments:       map[uuid.UUID][]Payment{},
// 					BondStreams:          []map[uuid.UUID]struct{}{},
// 				},
// 				portfolio,
// 			),
// 		)
// 	})

// 	t.Run("1mo bond spread", func(t *testing.T) {
// 		ctrl := gomock.NewController(t)
// 		interestRateRepository := mock_repository.NewMockInterestRateRepository(ctrl)
// 		bs := BondService{
// 			InterestRateRepository: interestRateRepository,
// 		}

// 		start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// 		interestRateRepository.EXPECT().GetRatesOnDate(start, nil).Return(
// 			&domain.InterestRateMap{
// 				Rates: map[int]float64{
// 					1: 0.0148,
// 					2: 0.0151,
// 					3: 0.0155,
// 				},
// 			}, nil,
// 		)

// 		portfolio, err := bs.ConstructBondPortfolio(
// 			nil,
// 			start,
// 			[]int{1, 2, 3},
// 			600_000,
// 		)
// 		require.NoError(t, err)

// 		expectedBonds := []Bond{
// 			NewBond(200000, start, 1, 0.0148),
// 			NewBond(200000, start, 2, 0.0151),
// 			NewBond(200000, start, 3, 0.0155),
// 		}

// 		require.Equal(
// 			t,
// 			"",
// 			cmp.Diff(
// 				&BondPortfolio{
// 					Cash:                 0,
// 					TargetDurationMonths: []int{1, 2, 3},
// 					Bonds:                expectedBonds,
// 					CouponPayments:       map[uuid.UUID][]Payment{},
// 					BondStreams: []map[uuid.UUID]struct{}{
// 						{
// 							expectedBonds[0].ID: {},
// 						},
// 						{
// 							expectedBonds[1].ID: {},
// 						},
// 						{
// 							expectedBonds[2].ID: {},
// 						},
// 					},
// 				},
// 				portfolio,
// 				cmpopts.IgnoreFields(Bond{}, "ID"),
// 			),
// 		)
// 	})
// }

// func TestBondPortfolio_RefreshCouponPayments(t *testing.T) {
// 	t.Run("check in several months later", func(t *testing.T) {
// 		start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// 		bond1 := NewBond(100, start, 10, 0.05)
// 		bond2 := NewBond(1000, start, 10, 0.1)
// 		bondPortfolio := &BondPortfolio{
// 			Bonds: []Bond{
// 				bond1,
// 				bond2,
// 			},
// 			CouponPayments: map[uuid.UUID][]Payment{},
// 		}

// 		firstRefresh := start.AddDate(0, 3, 15)
// 		bondPortfolio.refreshCouponPayments(firstRefresh)

// 		// temporarily removing payments from cash
// 		require.InDelta(t, 0, bondPortfolio.Cash, 0.0001)

// 		require.Equal(
// 			t,
// 			"",
// 			cmp.Diff(
// 				map[uuid.UUID][]Payment{
// 					bond1.ID: {
// 						{
// 							Date:   start.AddDate(0, 1, 0),
// 							Amount: 0.4166,
// 						},
// 						{
// 							Date:   start.AddDate(0, 2, 0),
// 							Amount: 0.4166,
// 						},
// 						{
// 							Date:   start.AddDate(0, 3, 0),
// 							Amount: 0.4166,
// 						},
// 					},
// 					bond2.ID: {
// 						{
// 							Date:   start.AddDate(0, 1, 0),
// 							Amount: 8.3333,
// 						},
// 						{
// 							Date:   start.AddDate(0, 2, 0),
// 							Amount: 8.3333,
// 						},
// 						{
// 							Date:   start.AddDate(0, 3, 0),
// 							Amount: 8.3333,
// 						},
// 					},
// 				},
// 				bondPortfolio.CouponPayments,
// 				floatCompare,
// 			),
// 		)
// 	})

// }

// var floatCompare = cmp.Comparer(func(i, j float64) bool {
// 	return math.Abs(i-j) < 1e-4
// })

// func TestBondPortfolio_RefreshBondHoldings(t *testing.T) {
// 	t.Run("refresh bond holdings", func(t *testing.T) {
// 		start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// 		bond1 := NewBond(200, start, 1, 0.01)
// 		bond2 := NewBond(100, start, 2, 0.02)
// 		bondPortfolio := &BondPortfolio{
// 			Bonds: []Bond{
// 				bond1,
// 				bond2,
// 			},
// 			TargetDurationMonths: []int{1, 2},
// 		}

// 		firstRefresh := start.AddDate(0, 1, 1)
// 		bondPortfolio.refreshBondHoldings(firstRefresh, domain.InterestRateMap{
// 			Rates: map[int]float64{
// 				2: 0.05,
// 			},
// 		})

// 		require.Equal(
// 			t,
// 			"",
// 			cmp.Diff(
// 				[]Bond{
// 					bond2,
// 					NewBond(200, bond1.Expiration, 2, 0.05),
// 				},
// 				bondPortfolio.Bonds,
// 				floatCompare,
// 				cmpopts.IgnoreFields(Bond{}, "ID"),
// 			),
// 		)
// 	})
// }

// func Test_computeMetrics(t *testing.T) {
// 	t.Run("compute average coupon rate", func(t *testing.T) {
// 		bond := NewBond(100, time.Now(), 1, 0.05)
// 		bond1 := NewBond(100, time.Now(), 1, 0.1)
// 		bond2 := NewBond(200, time.Now(), 1, 0.1)
// 		bonds := []Bond{
// 			bond,
// 			bond1,
// 			bond2,
// 		}
// 		payments := map[uuid.UUID][]Payment{
// 			bond.ID: {
// 				{
// 					Amount: 5.0 / 12,
// 				},
// 			},
// 			bond1.ID: {
// 				{
// 					Amount: 10.0 / 12,
// 				},
// 			},
// 			bond2.ID: {
// 				{
// 					Amount: 20.0 / 12,
// 				},
// 			},
// 		}
// 		metrics, err := computeMetrics(bonds, payments, nil)
// 		require.NoError(t, err)

// 		require.Equal(t, 0.0875, metrics.AverageCoupon)
// 	})

// 	t.Run("compute stdev", func(t *testing.T) {
// 		portfolioValues := []BondPortfolioReturn{
// 			{
// 				Date:                 time.Time{},
// 				DateStr:              "",
// 				ReturnSinceInception: 0,
// 			},
// 			{
// 				Date:                 time.Time{},
// 				DateStr:              "",
// 				ReturnSinceInception: 0.05,
// 			},
// 			{
// 				Date:                 time.Time{},
// 				DateStr:              "",
// 				ReturnSinceInception: 0.3,
// 			},
// 		}
// 		metrics, err := computeMetrics(nil, nil, portfolioValues)
// 		require.NoError(t, err)

// 		require.InDelta(t, 2.1, metrics.Stdev, 1e-4)
// 	})

// 	t.Run("compute max drawdown", func(t *testing.T) {
// 		portfolioValues := []BondPortfolioReturn{
// 			{
// 				Date:                 time.Time{},
// 				DateStr:              "",
// 				ReturnSinceInception: 0,
// 			},
// 			{
// 				Date:                 time.Time{},
// 				DateStr:              "",
// 				ReturnSinceInception: 0.05,
// 			},
// 			{
// 				Date:                 time.Time{},
// 				DateStr:              "",
// 				ReturnSinceInception: 0.3,
// 			},
// 			{
// 				Date:                 time.Time{},
// 				DateStr:              "",
// 				ReturnSinceInception: 0.1,
// 			},
// 		}
// 		metrics, err := computeMetrics(nil, nil, portfolioValues)
// 		require.NoError(t, err)

// 		require.InDelta(t, -0.2, metrics.MaximumDrawdown, 1e-4)
// 	})
// }
