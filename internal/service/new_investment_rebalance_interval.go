package service

import (
	"errors"
	"fmt"
	"strings"
)

// AllowedNewInvestmentRebalanceIntervals is the rebalance intervals accepted
// when creating a new investment.
//
// Temporary: live rebalance currently ignores strategy.rebalance_interval and
// rebalances every investment each cron run (effectively daily). Until other
// intervals are actually honored, only daily is allowed at creation. Do not
// use this list in the rebalance cron. Existing investments and saved
// strategies are unchanged. Expand or delete this when interval-aware live
// rebalance ships.
var AllowedNewInvestmentRebalanceIntervals = []string{"daily"}

// ErrUnsupportedNewInvestmentRebalanceInterval is returned when creating an
// investment whose strategy uses an interval that is not currently supported.
var ErrUnsupportedNewInvestmentRebalanceInterval = errors.New("only daily rebalance is currently supported")

// NewInvestmentRebalanceIntervalAllowed reports whether interval may be used
// for a newly created investment. Comparison is case-insensitive so stored
// values like "DAILY" and "daily" both match.
func NewInvestmentRebalanceIntervalAllowed(interval string) bool {
	for _, allowed := range AllowedNewInvestmentRebalanceIntervals {
		if strings.EqualFold(interval, allowed) {
			return true
		}
	}
	return false
}

// CheckNewInvestmentRebalanceInterval returns
// ErrUnsupportedNewInvestmentRebalanceInterval when interval is not allowed
// for a new investment.
func CheckNewInvestmentRebalanceInterval(interval string) error {
	if NewInvestmentRebalanceIntervalAllowed(interval) {
		return nil
	}
	return fmt.Errorf("%w for new investments (got %q)", ErrUnsupportedNewInvestmentRebalanceInterval, interval)
}
