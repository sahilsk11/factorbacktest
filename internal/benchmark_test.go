package internal

import (
	"factorbacktest/internal/domain"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func Test_intraPeriodChangeIterator(t *testing.T) {
	t.Run("two days, both present", func(t *testing.T) {
		t1 := NewDate(2020, 1, 1)
		t2 := NewDate(2020, 1, 2)
		out := intraPeriodChangeIterator(
			[]domain.AssetPrice{
				{
					Price: decimal.NewFromInt(100),
					Date:  NewDate(2020, 1, 1),
				},
				{
					Price: decimal.NewFromInt(110),
					Date:  NewDate(2020, 1, 2),
				},
			},
			// t1,
			t2,
			time.Hour*24,
		)

		require.Equal(
			t,
			"",
			cmp.Diff(
				map[time.Time]float64{
					t1: 0,
					t2: 10,
				},
				out,
			),
		)
	})
	t.Run("first day missing", func(t *testing.T) {
		// t1 := NewDate(2020, 1, 1)
		t2 := NewDate(2020, 1, 2)
		out := intraPeriodChangeIterator(
			[]domain.AssetPrice{
				{
					Price: decimal.NewFromInt(110),
					Date:  NewDate(2020, 1, 2),
				},
				{
					Price: decimal.NewFromInt(110),
					Date:  NewDate(2020, 1, 3),
				},
			},
			// t1,
			NewDate(2020, 1, 2),
			time.Hour*24,
		)

		require.Equal(
			t,
			"",
			cmp.Diff(
				map[time.Time]float64{
					t2: 0,
				},
				out,
			),
		)
	})

	t.Run("include last day", func(t *testing.T) {
		// t1 := NewDate(2020, 1, 1)
		t2 := NewDate(2020, 1, 2)
		t3 := NewDate(2020, 1, 3)
		out := intraPeriodChangeIterator(
			[]domain.AssetPrice{
				{
					Price: decimal.NewFromInt(110),
					Date:  NewDate(2020, 1, 2),
				},
				{
					Price: decimal.NewFromInt(110),
					Date:  NewDate(2020, 1, 3),
				},
			},
			// t1,
			NewDate(2020, 1, 3),
			time.Hour*24,
		)

		require.Equal(
			t,
			"",
			cmp.Diff(
				map[time.Time]float64{
					t2: 0,
					t3: 0,
				},
				out,
			),
		)
	})

}
