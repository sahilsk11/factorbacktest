package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckNewInvestmentRebalanceInterval(t *testing.T) {
	t.Parallel()

	t.Run("accepts daily", func(t *testing.T) {
		t.Parallel()
		for _, interval := range []string{"daily", "DAILY", "Daily"} {
			require.NoError(t, CheckNewInvestmentRebalanceInterval(interval), interval)
			require.True(t, NewInvestmentRebalanceIntervalAllowed(interval), interval)
		}
	})

	t.Run("rejects weekly monthly yearly", func(t *testing.T) {
		t.Parallel()
		for _, interval := range []string{"weekly", "WEEKLY", "monthly", "MONTHLY", "yearly", "YEARLY"} {
			err := CheckNewInvestmentRebalanceInterval(interval)
			require.ErrorIs(t, err, ErrUnsupportedNewInvestmentRebalanceInterval, interval)
			require.ErrorContains(t, err, "only daily rebalance is currently supported", interval)
			require.False(t, NewInvestmentRebalanceIntervalAllowed(interval), interval)
		}
	})
}
