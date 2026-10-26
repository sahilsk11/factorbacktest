package service

import (
	"strings"
	"time"
)

// dueForRebalance reports whether an investment should rebalance now.
// A nil lastPlacedAt means it has never had a COMPLETED/PENDING rebalance.
// monthly is due once now's UTC calendar month is after lastPlacedAt's month.
func dueForRebalance(interval string, lastPlacedAt *time.Time, now time.Time) bool {
	if lastPlacedAt == nil {
		return true
	}
	last := lastPlacedAt.UTC()
	now = now.UTC()
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "weekly":
		return !now.Before(last.AddDate(0, 0, 7))
	case "monthly":
		ly, lm, _ := last.Date()
		ny, nm, _ := now.Date()
		return ny > ly || (ny == ly && nm > lm)
	case "yearly":
		return now.Year() > last.Year()
	default: // daily and unknown: next UTC calendar day (same-day cron is a no-op)
		ly, lm, ld := last.Date()
		ny, nm, nd := now.Date()
		lastDay := time.Date(ly, lm, ld, 0, 0, 0, 0, time.UTC)
		nowDay := time.Date(ny, nm, nd, 0, 0, 0, 0, time.UTC)
		return nowDay.After(lastDay)
	}
}
