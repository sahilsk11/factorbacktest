package interestrate

import (
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestGetYieldCurve(t *testing.T) {
	t.Run("random date", func(t *testing.T) {
		response, err := GetYieldCurve(time.Date(
			2020, 1, 1, 0, 0, 0, 0, time.UTC,
		))
		require.NoError(t, err)

		expected := map[int]float64{
			120: 0.0192,
			1:   0.0148,
			12:  0.0159,
			240: 0.0225,
			2:   0.0151,
			24:  0.0158,
			360: 0.0239,
			3:   0.0155,
			36:  0.0162,
			60:  0.0169,
			6:   0.016,
			84:  0.0183,
		}

		require.Equal(
			t,
			"",
			cmp.Diff(
				&InterestRateMap{
					Rates: expected,
				},
				response,
				cmp.Comparer(func(i, j float64) bool {
					return math.Abs(i-j) < 0.0001
				}),
			),
		)
	})
}
